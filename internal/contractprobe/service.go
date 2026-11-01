// Package contractprobe observes an operator-selected public contract outside the controller.
package contractprobe

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

// Service serves private contract reachability probes and metrics.
type Service struct {
	client   *http.Client
	enabled  func(context.Context) bool
	target   string
	slots    chan struct{}
	metrics  http.Handler
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	failures *prometheus.CounterVec
	ready    prometheus.Gauge
	observed prometheus.Gauge
	metricMu sync.RWMutex
}

// probeObservation keeps diagnostics private until the whole completed result is published.
type probeObservation struct {
	result, stage, failureReason, httpStatus string
}

// NewService constructs a default-off probe with fixed network and resource boundaries.
func NewService(target string, enabled func(context.Context) bool) *Service {
	registry := prometheus.NewRegistry()
	service := &Service{
		registry: registry,
		target:   target,
		enabled:  enabled,
		slots:    make(chan struct{}, 1),
		client: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				Proxy:                  nil,
				DialContext:            (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
				TLSHandshakeTimeout:    3 * time.Second,
				ResponseHeaderTimeout:  5 * time.Second,
				IdleConnTimeout:        30 * time.Second,
				MaxConnsPerHost:        1,
				MaxIdleConns:           1,
				MaxIdleConnsPerHost:    1,
				MaxResponseHeaderBytes: 16 << 10,
				DisableCompression:     true,
			},
		},
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "contract_probe_requests_total",
				Help: "Contract probe results with a fixed reason vocabulary.",
			},
			[]string{"result"},
		),
		failures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "contract_probe_failures_total",
				Help: "Completed unavailable contract probes by bounded failure stage and cause.",
			},
			[]string{"stage", "reason", "http_status"},
		),
		ready: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "contract_probe_ready",
				Help: "Whether the latest completed contract check succeeded; zero before observation.",
			},
		),
		observed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "contract_probe_last_observation_timestamp_seconds",
				Help: "Unix time of the latest completed contract check.",
			},
		),
	}
	registry.MustRegister(service.requests, service.failures, service.ready, service.observed)
	service.metrics = promhttp.HandlerFor(service, promhttp.HandlerOpts{})
	return service
}

// Close releases idle connections.
func (s *Service) Close() { s.client.CloseIdleConnections() }

// ServeHTTP exposes read-only management endpoints without returning upstream bytes or URLs.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "ReadOnly", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 ||
		len(r.TransferEncoding) != 0 {
		http.Error(w, "QueryParametersAndBodiesNotSupported", http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case "/healthz":
		w.WriteHeader(http.StatusOK)
	case "/metrics":
		s.metrics.ServeHTTP(w, r)
	case "/readyz":
		s.readiness(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Gather snapshots one completed observation before HTTP response writing begins.
func (s *Service) Gather() ([]*dto.MetricFamily, error) {
	s.metricMu.RLock()
	defer s.metricMu.RUnlock()
	return s.registry.Gather()
}

// readiness limits concurrent probes and preserves the last completed observation when busy.
func (s *Service) readiness(w http.ResponseWriter, r *http.Request) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		s.requests.WithLabelValues("ContractProbeBusy").Inc()
		http.Error(w, "ContractProbeBusy", http.StatusServiceUnavailable)
		return
	}
	observation := probeObservation{result: "FeatureDisabled"}
	if s.enabled != nil && s.enabled(r.Context()) {
		observation = s.fetch(r.Context())
	}
	if r.Context().Err() != nil {
		s.requests.WithLabelValues("ContractProbeCancelled").Inc()
		http.Error(w, "ContractProbeCancelled", http.StatusServiceUnavailable)
		return
	}
	s.metricMu.Lock()
	if r.Context().Err() != nil {
		s.metricMu.Unlock()
		s.requests.WithLabelValues("ContractProbeCancelled").Inc()
		http.Error(w, "ContractProbeCancelled", http.StatusServiceUnavailable)
		return
	}
	s.requests.WithLabelValues(observation.result).Inc()
	if observation.stage != "" {
		s.failures.WithLabelValues(observation.stage, observation.failureReason, observation.httpStatus).
			Inc()
	}
	s.observed.SetToCurrentTime()
	s.ready.Set(0)
	if observation.result == "ContractReachable" {
		s.ready.Set(1)
	}
	s.metricMu.Unlock()
	if observation.result == "ContractReachable" {
		_, _ = io.WriteString(w, "ContractReachable")
		return
	}
	http.Error(w, observation.result, http.StatusServiceUnavailable)
}

// fetch discards a complete bounded response; the target is immutable operator configuration.
func (s *Service) fetch(ctx context.Context) probeObservation {
	target, err := url.Parse(s.target)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" ||
		target.User != nil || strings.Contains(s.target, "$(") ||
		strings.ContainsAny(s.target, "#\\") ||
		target.RawQuery != "" || target.ForceQuery ||
		target.Fragment != "" ||
		target.Opaque != "" {
		return probeObservation{result: "ContractConfigurationInvalid"}
	}
	// #nosec G704 -- The validated URL is fixed operator configuration, never caller input.
	// The independently deployed probe has explicitly restricted network egress.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.target, nil)
	if err != nil {
		return probeObservation{result: "ContractConfigurationInvalid"}
	}
	request.Header.Set("Accept-Encoding", "identity")
	// #nosec G704 -- A fixed operator-configured HTTPS URL is used, never caller input. TLS,
	// redirects, proxies and size/time/concurrency are bounded; NetworkPolicy scopes egress.
	response, err := s.client.Do(request)
	if err != nil {
		return unavailableObservation("request", 0, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return unavailableObservation("status", response.StatusCode, nil)
	}
	encodings := response.Header.Values("Content-Encoding")
	if response.ContentLength > 1<<20 ||
		(len(encodings) != 0 && (len(encodings) != 1 || encodings[0] != "identity")) {
		return probeObservation{result: "ContractInvalidResponse"}
	}
	size, err := io.Copy(io.Discard, io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return unavailableObservation("body", response.StatusCode, err)
	}
	if size == 0 || size > 1<<20 {
		return probeObservation{result: "ContractInvalidResponse"}
	}
	return probeObservation{result: "ContractReachable"}
}

// unavailableObservation accepts no upstream text as a label; status cardinality is bounded.
func unavailableObservation(stage string, status int, err error) probeObservation {
	observation := probeObservation{
		result:        "ContractUnavailable",
		stage:         stage,
		failureReason: "http_status",
		httpStatus:    "none",
	}
	if status != 0 {
		observation.httpStatus = "other"
		if status >= 100 && status <= 599 {
			observation.httpStatus = strconv.Itoa(status)
		}
	}
	if err != nil {
		observation.failureReason = failureReason(err)
	}
	return observation
}

// failureReason classifies typed causes through wrapping without retaining error strings or URLs.
func failureReason(err error) string {
	var dnsError *net.DNSError
	var networkError net.Error
	var certificateError *tls.CertificateVerificationError
	var recordError tls.RecordHeaderError
	switch {
	case errors.As(err, &dnsError):
		return "dns"
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()):
		return "timeout"
	case errors.As(err, &certificateError) || errors.As(err, &recordError):
		return "tls"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection_reset"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected_eof"
	case errors.Is(err, io.EOF):
		return "eof"
	default:
		return "other"
	}
}
