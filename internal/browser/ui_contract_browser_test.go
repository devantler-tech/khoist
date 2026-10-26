//go:build browser

package browser_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	datav1alpha1 "github.com/devantler-tech/data-product-controller/api/v1alpha1"
	"github.com/devantler-tech/data-product-controller/internal/registry"
	"github.com/go-rod/rod"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestUIManifestValidation rejects unsafe contracts before any iframe navigation.
func TestUIManifestValidation(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(registry.NewHandler(
		fake.NewClientBuilder().
			WithScheme(scheme).
			Build(),
	))
	t.Cleanup(server.Close)
	browser := contractBrowser(t)
	page := browser.MustPage().Timeout(20 * time.Second).MustNavigate(server.URL).MustWaitLoad()
	result := page.MustEval(`async () => {
		await new Promise((resolve, reject) => {
			const script = document.createElement('script');
			script.src = '/assets/metadata.js'; script.onload = resolve;
			script.onerror = () => reject(new Error('portable UI library unavailable'));
			document.head.append(script);
		});
		const valid = {url:'https://product.example/ui',title:'Explore',contract:{apiVersion:'data-product-ui/v1',hostOrigins:[location.origin],capabilities:['status','resize']}};
		const accepted = DataProductMetadata.validate(valid, location.origin);
		if (accepted.url !== valid.url) return 'valid entrypoint changed';
		const themed = structuredClone(valid);
		themed.contract.apiVersion = 'data-product-ui/v2';
		themed.contract.capabilities.push('appearance');
		try { DataProductMetadata.validate(themed, location.origin); } catch { return 'bounded v2 appearance unsupported'; }
		for (const version of ['data-product-ui/v1','data-product-ui/v2']) {
			const malformed = structuredClone(valid);
			malformed.url = location.origin + '/invalid-product';
			malformed.contract.apiVersion = [version];
			try { DataProductMetadata.validate(malformed, location.origin); return 'accepted array protocol version'; } catch { /* Expected refusal. */ }

		}
		const invalid = [
			v => v.url = 'javascript:alert(1)',
			v => v.url = 'https://user:password@product.example/ui',
			v => v.url = 'https://product.example/ui#token',
			v => v.title = '',
			v => v.title = 'x'.repeat(201),
			v => v.contract.apiVersion = 'data-product-ui/v3',
			v => v.contract.hostOrigins = ['*'],
			v => v.contract.hostOrigins.push('https://*.example'),
			v => v.contract.hostOrigins.push('https://-invalid.example'),
			v => v.contract.hostOrigins.push('https://catalog.example:0'),
			v => v.contract.hostOrigins = [location.origin + '/'],
			v => v.contract.hostOrigins = ['https://other.example'],
			v => v.contract.capabilities = ['credentials'],
			v => v.contract.capabilities = ['appearance'],
			v => v.contract.capabilities = ['status','status'],
			v => v.contract.unexpected = true,
			v => v.extra = 'x'.repeat(17000),
		];
		for (let i=0;i<invalid.length;i++) {
			const value = structuredClone(valid); invalid[i](value);
			try { DataProductMetadata.validate(value, location.origin); return 'accepted invalid case '+i; } catch { /* Expected refusal. */ }
		}
		return 'validated';
	}`).Str()
	if result != "validated" {
		t.Fatal(result)
	}
}

// contractBrowser runs protocol checks in Chromium with certificates confined to TLS fixtures.
func contractBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	controlURL := launchBrowser(t, "ignore-certificate-errors")
	browser := rod.New().
		Context(t.Context()).
		Timeout(60 * time.Second).
		ControlURL(controlURL).
		WithPanic(func(value interface{}) { t.Fatalf("browser: %v", value) }).
		MustConnect()
	t.Cleanup(func() { _ = browser.Context(context.Background()).Timeout(3 * time.Second).Close() })
	return browser
}
