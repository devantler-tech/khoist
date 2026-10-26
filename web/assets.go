// Package web distributes inert publisher-review assets without product execution.
package web

import (
	"embed"
	"net/http"
)

// Assets contains only publisher review and bounded metadata inspection libraries.
//
//go:embed metadata.js descriptor.js publisher.html publisher.css publisher.js preflight-report.js
var Assets embed.FS

// PublisherHandler serves offline report inspection under its independent release gate.
func PublisherHandler(enabled func() bool) http.Handler {
	files := http.FileServerFS(Assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enabled == nil || !enabled() ||
			(r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/",
			"/index.html",
			"/publisher-review",
			"/publisher.html",
			"/publisher.css",
			"/publisher.js",
			"/preflight-report.js",
			"/metadata.js",
			"/descriptor.js":
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().
			Set("Content-Security-Policy", "default-src 'self'; connect-src 'none'; frame-src 'none'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			http.Redirect(w, r, "/publisher-review", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/publisher-review" {
			r = r.Clone(r.Context())
			r.URL.Path = "/publisher.html"
		}
		files.ServeHTTP(w, r)
	})
}
