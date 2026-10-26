package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestMetadataAssetHTTP preserves bounded inert library delivery for GET, HEAD and range clients.
func TestMetadataAssetHTTP(t *testing.T) {
	t.Parallel()
	handler := PublisherHandler(func() bool { return true })
	full := httptest.NewRecorder()
	handler.ServeHTTP(
		full,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata.js", nil),
	)
	if full.Code != http.StatusOK ||
		full.Header().Get("Content-Length") != strconv.Itoa(full.Body.Len()) ||
		!strings.Contains(full.Body.String(), "window.DataProductMetadata =") ||
		strings.Contains(full.Body.String(), "window.DataProductUI =") {
		t.Fatal("metadata response lost its inert public library")
	}
	head := httptest.NewRecorder()
	handler.ServeHTTP(
		head,
		httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/metadata.js", nil),
	)
	if head.Code != http.StatusOK || head.Body.Len() != 0 ||
		head.Header().Get("Content-Length") != full.Header().Get("Content-Length") {
		t.Fatal("HEAD lost metadata length or exposed a body")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metadata.js", nil)
	request.Header.Set("Range", "bytes=0-63")
	part := httptest.NewRecorder()
	handler.ServeHTTP(part, request)
	if part.Code != http.StatusPartialContent || part.Body.String() != full.Body.String()[:64] {
		t.Fatal("metadata range delivery changed its source bytes")
	}
}

// TestRetiredSessionAssetsAreNotEmbedded prevents a generic file server restoring removed code.
func TestRetiredSessionAssetsAreNotEmbedded(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"index.html", "kit.css", "kit.js", "ui-contract.js"} {
		if _, err := fs.ReadFile(Assets, name); err == nil {
			t.Errorf("retired asset %s remains embedded", name)
		}
	}
}
