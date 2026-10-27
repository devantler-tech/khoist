//go:build browser

package browser_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devantler-tech/data-product-controller/internal/registry"
)

// TestWorkspaceConfigurationRollback withdraws stale metadata and explains how to recover an open page.
func TestWorkspaceConfigurationRollback(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{"missing", http.StatusNotFound, "not found"},
		{"unavailable", http.StatusServiceUnavailable, "unavailable"},
		{"legacy-html", http.StatusOK, "<!doctype html><title>Legacy workspace</title>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var rolledBack atomic.Bool
			var legacyReads atomic.Int64
			assets := registry.NewHandler(nil)
			product := discoveryProduct("harbour")
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/api/v1/workspace-config":
						if rolledBack.Load() {
							w.WriteHeader(test.status)
							_, _ = w.Write([]byte(test.body))
							return
						}
						_ = json.NewEncoder(w).
							Encode(map[string]bool{"discoveryEnabled": true, "lineageEnabled": true})
					case "/api/v2/products":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"apiVersion": "data-product-discovery/v1",
							"products":   []any{product},
							"continue":   "",
							"rejected":   0,
						})
					case "/api/v2/products/products/harbour":
						_ = json.NewEncoder(w).Encode(product)
					case "/api/v1/products":
						legacyReads.Add(1)
						_ = json.NewEncoder(w).Encode(map[string]any{"products": []any{product}})
					default:
						assets.ServeHTTP(w, r)
					}
				}),
			)
			t.Cleanup(server.Close)
			page := contractBrowser(t).MustPage().Timeout(10 * time.Second).
				MustNavigate(server.URL).MustWaitLoad()
			page.MustElement(".product-card").MustWaitVisible().MustClick()
			page.MustElement("#product-metadata").MustWaitVisible()
			rolledBack.Store(true)
			page.MustElement("#refresh-products").MustClick()
			page.MustElement("#refresh-products").MustWait(`() => !this.disabled`)
			if text := page.MustElement("#registry-status").
				MustText(); !strings.Contains(
				text,
				"Reload this page",
			) {
				t.Fatalf(
					"missing rollback recovery advice: %q; legacy inventory reads=%d",
					text,
					legacyReads.Load(),
				)
			}
			if !page.MustEval(`() => document.querySelector('#product-metadata').hidden &&
				document.querySelector('#descriptor-actions').hidden && document.querySelector('#catalog-scope').hidden &&
				document.querySelectorAll('.product-card').length === 0`).Bool() {
				t.Fatal(
					"unavailable configuration retained selected metadata or discovery controls",
				)
			}
			if legacyReads.Load() != 0 {
				t.Fatal("unavailable configuration silently selected the legacy inventory")
			}
			rolledBack.Store(false)
			page.MustElement("#refresh-products").MustClick()
			page.MustElement(".product-card").MustWaitVisible()
			if page.MustEval(`() => document.querySelector('#catalog-scope').hidden`).Bool() {
				t.Fatal("a successful retry did not restore current discovery settings")
			}
		})
	}
}
