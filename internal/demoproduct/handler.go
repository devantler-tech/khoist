// Package demoproduct provides a self-contained example data product.
package demoproduct

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devantler-tech/data-product-controller/internal/config"
)

//go:embed ui/releases.html ui/product.css ui/product.js
var uiFiles embed.FS

type observation struct {
	Station     string    `json:"station"`
	Temperature float64   `json:"temperatureCelsius"`
	Salinity    float64   `json:"salinityPsu"`
	ObservedAt  time.Time `json:"observedAt"`
}

var observations = []observation{
	{
		Station:     "nordhavn",
		Temperature: 16.8,
		Salinity:    14.2,
		ObservedAt:  time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC),
	},
	{
		Station:     "refshaleoen",
		Temperature: 17.1,
		Salinity:    13.9,
		ObservedAt:  time.Date(2026, time.August, 29, 12, 5, 0, 0, time.UTC),
	},
}

// HandlerOptions binds published endpoints to publisher-owned configuration.
type HandlerOptions struct{ PublicBaseURL string }

// NewHandler returns the sample API and independent release browser.
func NewHandler() http.Handler { return newHandler(HandlerOptions{}) }

// NewHandlerWithPublicURL validates an explicitly published public endpoint.
func NewHandlerWithPublicURL(baseURL string) (http.Handler, error) {
	return NewHandlerWithOptions(HandlerOptions{PublicBaseURL: baseURL})
}

// NewHandlerWithOptions validates publication settings before serving.
func NewHandlerWithOptions(options HandlerOptions) (http.Handler, error) {
	baseURL := options.PublicBaseURL
	if baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil || len(baseURL) > 1024 {
			return nil, fmt.Errorf(
				"PUBLIC_BASE_URL requires an exact HTTPS URL with an optional path prefix",
			)
		}
		origin := "https://" + parsed.Host
		if _, err = config.UIHostOrigins(
			origin,
		); err != nil || parsed.Scheme != "https" || parsed.User != nil ||
			baseURL != origin+parsed.Path ||
			!publicPath.MatchString(parsed.Path) {
			return nil, fmt.Errorf(
				"PUBLIC_BASE_URL requires an exact HTTPS URL with an optional path prefix",
			)
		}
	}
	return newHandler(options), nil
}

var publicPath = regexp.MustCompile(`^(/[A-Za-z0-9_-]+)*$`)

// newHandler serves the public API and a separate consumer application.
func newHandler(_ HandlerOptions) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/observations", observationsHandler)
	mux.HandleFunc("/openapi.json", openAPIHandler)
	registerReleaseSurface(mux)
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)

			return
		}
		response.WriteHeader(http.StatusOK)
	})

	return mux
}

func observationsHandler(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	// This public API permits credential-free reads without sharing cookies or credentials.
	response.Header().Set("Access-Control-Allow-Origin", "*")

	station := strings.ToLower(strings.TrimSpace(request.URL.Query().Get("station")))
	items := make([]observation, 0, len(observations))
	for _, item := range observations {
		if station == "" || item.Station == station {
			items = append(items, item)
		}
	}

	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "public, max-age=60")
	if err := json.NewEncoder(response).Encode(struct {
		Items []observation `json:"items"`
	}{Items: items}); err != nil {
		slog.ErrorContext(request.Context(), "encode observations", "error", err)
	}
}

func openAPIHandler(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, _ = response.Write([]byte(openAPIDocument))
}

const openAPIDocument = `{"openapi":"3.1.0","info":{"title":"Harbour observations","version":"1.0.0"},"paths":{"/api/observations":{"get":{"summary":"Query harbour observations","parameters":[{"name":"station","in":"query","schema":{"type":"string"}}],"responses":{"200":{"description":"Matching observations","content":{"application/json":{"schema":{"type":"object","required":["items"],"properties":{"items":{"type":"array","items":{"type":"object","required":["station","temperatureCelsius","salinityPsu","observedAt"],"properties":{"station":{"type":"string"},"temperatureCelsius":{"type":"number"},"salinityPsu":{"type":"number"},"observedAt":{"type":"string","format":"date-time"}}}}}}}}}}}}}}`
