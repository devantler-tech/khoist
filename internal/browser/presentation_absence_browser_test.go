//go:build browser

package browser_test

import (
	"net/http/httptest"
	"testing"

	"github.com/devantler-tech/data-product-controller/internal/demoproduct"
)

// TestHarbourPresentationUnavailable exercises retired routes in the real browser.
func TestHarbourPresentationUnavailable(t *testing.T) {
	server := httptest.NewTLSServer(demoproduct.NewHandler())
	t.Cleanup(server.Close)
	page := contractBrowser(t).MustPage()
	for _, path := range []string{"/ui", "/ui-contract-config", "/ksail-releases/ui-contract-config"} {
		page.MustNavigate(server.URL + path).MustWaitLoad()
		if !page.MustEval(`() => document.body.textContent.includes('404 page not found') && typeof DataProductUI === 'undefined' && document.querySelectorAll('iframe,script').length === 0`).
			Bool() {
			t.Fatalf("retired route %s restored product presentation", path)
		}
	}
}
