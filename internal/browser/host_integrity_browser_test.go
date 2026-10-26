//go:build browser

package browser_test

import (
	"net/http/httptest"
	"testing"

	"github.com/devantler-tech/data-product-controller/web"
)

// TestPortableManifestURLProfile compares real portable library admission with the delivered descriptor profile.
func TestPortableManifestURLProfile(t *testing.T) {
	kit := httptest.NewTLSServer(web.PublisherHandler(func() bool { return true }))
	t.Cleanup(kit.Close)
	page := contractBrowser(t).MustPage().MustNavigate(kit.URL).MustWaitLoad()
	for _, value := range []string{"https:product.example/ui", "https:///product.example/ui", "HTTPS://product.example/ui", "https://product.example:/ui", "https://product.example:0/ui", "https://bad_host.example/ui", "https://product.example/%zz", "https://127.1/ui", "https://product.example/ui#", "https://product.example/é"} {
		if page.MustEval(`(url)=>{try{DataProductMetadata.validateMetadata({url,title:'Surface',contract:{apiVersion:'data-product-ui/v1',hostOrigins:[location.origin],capabilities:[]}});return true;}catch{return false;}}`, value).
			Bool() {
			t.Errorf("nonportable manifest URL admitted: %s", value)
		}
	}
	for _, value := range []string{"https://product.example/ui", "https://localhost/query", "https://127.0.0.1:8443/ui", "https://[::1]:8443/ui", "https://product.example:443/path%20ok?query=%25"} {
		if !page.MustEval(`(url)=>{try{DataProductMetadata.validateMetadata({url,title:'Surface',contract:{apiVersion:'data-product-ui/v1',hostOrigins:[location.origin],capabilities:[]}});return true;}catch{return false;}}`, value).
			Bool() {
			t.Errorf("supported canonical manifest URL rejected: %s", value)
		}
	}
}
