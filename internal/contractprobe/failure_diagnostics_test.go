package contractprobe

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
)

// TestCompletedFailureDiagnostics distinguishes rejected responses from incomplete bodies without changing readiness.
func TestCompletedFailureDiagnostics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		status     int
		partial    bool
	}{
		{"rejected", `contract_probe_failures_total{http_status="429",reason="http_status",stage="status"} 1`, 429, false},
		{"bad gateway", `contract_probe_failures_total{http_status="502",reason="http_status",stage="status"} 1`, 502, false},
		{"unavailable", `contract_probe_failures_total{http_status="503",reason="http_status",stage="status"} 1`, 503, false},
		{"redirect", `contract_probe_failures_total{http_status="302",reason="http_status",stage="status"} 1`, 302, false},
		{"nonstandard status", `contract_probe_failures_total{http_status="other",reason="http_status",stage="status"} 1`, 999, false},
		{"body interrupted", `contract_probe_failures_total{http_status="200",reason="unexpected_eof",stage="body"} 1`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var hits atomic.Int32
			upstream := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					hits.Add(1)
					w.Header().Set("Location", "https://private.invalid/response-sentinel")
					if tc.partial {
						w.Header().Set("Content-Length", "100")
					}
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, "response-sentinel")
				}),
			)
			defer upstream.Close()
			service := trustedService(t, upstream)
			response := request(t, service, "GET", "/readyz")
			if response.Code != 503 || response.Body.String() != "ContractUnavailable\n" {
				t.Fatalf("failure changed readiness: %d %s", response.Code, response.Body)
			}
			for range 2 {
				metrics := request(t, service, "GET", "/metrics").Body.String()
				if !strings.Contains(metrics, tc.want+"\n") {
					t.Fatalf(
						"completed failure lacks its diagnostic counter: want %s\n%s",
						tc.want,
						metrics,
					)
				}
				if strings.Contains(metrics, "sentinel") ||
					strings.Contains(metrics, upstream.URL) ||
					!strings.Contains(
						metrics,
						`contract_probe_requests_total{result="ContractUnavailable"} 1`,
					) ||
					!strings.Contains(metrics, "contract_probe_ready 0\n") {
					t.Fatalf(
						"metrics leaked response content or lost the failed observation: %s",
						metrics,
					)
				}
			}
			if hits.Load() != 1 {
				t.Fatalf(
					"diagnostics or scrapes retried/fetched the target: %d requests",
					hits.Load(),
				)
			}
		})
	}
}

// TestTransportFailureDiagnostics exercises the actual HTTP client with controlled dial failures, not error text parsing.
func TestTransportFailureDiagnostics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, reason string
		err          error
	}{
		{"dns", "dns", &net.DNSError{Err: "error-sentinel", Name: "private.invalid"}},
		{"timeout", "timeout", context.DeadlineExceeded},
		{"reset", "connection_reset", syscall.ECONNRESET},
		{"refused", "connection_refused", syscall.ECONNREFUSED},
		{"eof", "eof", io.EOF},
		{"unknown", "other", errors.New("error-sentinel https://private.invalid/token")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := NewService(
				"https://contract.example.test/schema",
				func(context.Context) bool { return true },
			)
			defer service.Close()
			transport, ok := service.client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("probe must retain its bounded HTTP transport")
			}
			transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: tc.err}
			}
			response := request(t, service, "GET", "/readyz")
			metrics := request(t, service, "GET", "/metrics").Body.String()
			want := `contract_probe_failures_total{http_status="none",reason="` + tc.reason + `",stage="request"} 1`
			if response.Code != 503 || response.Body.String() != "ContractUnavailable\n" ||
				!strings.Contains(metrics, want+"\n") {
				t.Fatalf(
					"transport failure lacks its fixed diagnostic: response=%d %s\nwant %s\n%s",
					response.Code,
					response.Body,
					want,
					metrics,
				)
			}
			if strings.Contains(metrics, "sentinel") ||
				strings.Contains(metrics, "private.invalid") ||
				strings.Contains(metrics, "contract.example.test") {
				t.Fatalf("transport diagnostics exposed error or target text: %s", metrics)
			}
		})
	}
}

// TestTLSFailureDiagnostics requires real certificate verification failure to remain distinct from HTTP failures.
func TestTLSFailureDiagnostics(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "response-sentinel")
		}),
	)
	defer upstream.Close()
	service := NewService(upstream.URL, func(context.Context) bool { return true })
	defer service.Close()
	response := request(t, service, "GET", "/readyz")
	metrics := request(t, service, "GET", "/metrics").Body.String()
	if response.Code != 503 || response.Body.String() != "ContractUnavailable\n" ||
		!strings.Contains(
			metrics,
			`contract_probe_failures_total{http_status="none",reason="tls",stage="request"} 1`+"\n",
		) {
		t.Fatalf(
			"TLS failure lost its verified diagnostic: response=%d %s\n%s",
			response.Code,
			response.Body,
			metrics,
		)
	}
}
