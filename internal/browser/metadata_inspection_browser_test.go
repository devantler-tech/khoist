//go:build browser

package browser_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// metadataInspectionFixture keeps strict public metadata tests independent of product execution.
func metadataInspectionFixture(t *testing.T) (*rod.Page, string, string, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	product := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "metadata inspection must remain offline", http.StatusForbidden)
	}))
	t.Cleanup(product.Close)
	host := publisherHost(t)
	page := contractBrowser(
		t,
	).MustPage().
		Timeout(15 * time.Second).
		MustNavigate(host.URL + "/publisher-review").
		MustWaitLoad()
	return page, host.URL, product.URL, &calls
}

// inspectDescriptor exercises the canonical decoder without mounting publisher content.
func inspectDescriptor(t *testing.T, page *rod.Page, wire string) bool {
	t.Helper()
	accepted := page.MustEval(`wire=>{try{DataProductDescriptor.validate(DataProductDescriptor.parseJSON(wire));return true}catch{return false}}`, wire).
		Bool()
	if !page.MustEval(`()=>typeof DataProductUI==='undefined'&&document.querySelectorAll('iframe').length===0`).
		Bool() {
		t.Fatal("metadata inspection loaded product execution")
	}
	return accepted
}
