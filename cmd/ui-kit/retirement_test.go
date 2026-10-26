package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestUIKitRetiredSessions cannot restore product execution through retained deployment settings.
func TestUIKitRetiredSessions(t *testing.T) {
	for _, transport := range []string{"tls", "gateway"} {
		for _, legacy := range []string{"", "false", "true", "invalid"} {
			for _, publisher := range []string{"false", "true"} {
				t.Run(transport+"/legacy="+legacy+"/publisher="+publisher, func(t *testing.T) {
					address := availableAddress(t)
					args := []string{"--listen-address", address}
					client := &http.Client{Timeout: time.Second}
					scheme := "http"
					if transport == "tls" {
						cert, key, trusted := hostCertificate(t)
						args = append(args, "--tls-cert", cert, "--tls-key", key)
						client.Transport = trusted
						scheme = "https"
					} else {
						args = append(args, "--http-behind-gateway")
					}
					startHost(t, args, legacy,
						"UI_APPEARANCE_ENABLED="+legacy,
						"REGISTRY_DISCOVERY_ENABLED="+legacy,
						"PUBLISHER_PREFLIGHT_ENABLED="+publisher,
					)
					base := scheme + "://" + address
					waitForHost(t, client, base+"/healthz")
					for _, path := range []string{"/kit.js", "/kit.css", "/ui-contract.js"} {
						status, _, _ := hostRequest(t, client, http.MethodGet, base+path, "")
						if status != http.StatusNotFound {
							t.Fatalf(
								"retired session asset %s remained available: %d",
								path,
								status,
							)
						}
					}
					for _, path := range []string{"/", "/index.html", "/publisher-review"} {
						status, body, headers := hostRequest(
							t,
							client,
							http.MethodGet,
							base+path,
							"",
						)
						want := http.StatusNotFound
						if publisher == "true" {
							want = http.StatusOK
						}
						if status != want {
							t.Fatalf("publisher-only %s: status=%d want=%d", path, status, want)
						}
						if publisher == "true" &&
							(!strings.Contains(body, "preflight-report.js") ||
								strings.Contains(body, "ui-contract.js") ||
								!strings.Contains(headers.Get("Content-Security-Policy"), "frame-src 'none'")) {
							t.Fatal("publisher host enabled product presentation")
						}
					}
				})
			}
		}
	}
}
