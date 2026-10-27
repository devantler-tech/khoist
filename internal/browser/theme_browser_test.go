//go:build browser

package browser_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	datav1alpha1 "github.com/devantler-tech/data-product-controller/api/v1alpha1"
	"github.com/devantler-tech/data-product-controller/internal/demoproduct"
	"github.com/devantler-tech/data-product-controller/internal/registry"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestWorkspaceThemes changes local appearance without losing selected metadata or contacting a publisher.
func TestWorkspaceThemes(t *testing.T) {
	page, loads := themeWorkspace(t, false)
	if !page.MustEval(`() => !!document.querySelector('select[aria-label="Theme"]')`).Bool() {
		t.Fatal("workspace has no accessible theme selector")
	}
	page.MustElement(".product-card").MustClick()
	page.MustElement("#product-owner").MustWait(`() => this.textContent === 'Harbour team'`)
	page.MustElement("#appearance").MustSelect("Dark")
	waitTheme(t, page, "rgb(0, 0, 0)")
	page.MustElement("#appearance").MustFocus()
	page.Keyboard.MustType(input.KeyL)
	waitTheme(t, page, "rgb(255, 255, 255)")
	if loads.Load() != 0 || page.MustElement("#product-owner").MustText() != "Harbour team" ||
		len(page.MustElements("iframe")) != 0 {
		t.Fatal("appearance change contacted the product or lost selected metadata")
	}
	page.MustElement("#appearance").MustSelect("Dark")
	page.MustReload().MustWaitLoad()
	page.MustElement(".product-card").MustWaitVisible()
	if got := page.MustElement("#appearance").MustProperty("value").Str(); got != "dark" {
		t.Fatalf("explicit appearance was not restored: %q", got)
	}
	page.MustElement("html").
		MustWait(`() => getComputedStyle(this).backgroundColor === 'rgb(0, 0, 0)'`)
	page.MustSetViewport(375, 812, 1, false)
	if page.MustEval(`() => document.documentElement.scrollWidth > innerWidth`).Bool() {
		t.Fatal("theme controls overflow the mobile workspace")
	}
}

// TestSystemThemeWithDeniedStorage follows preference changes even when browser storage is unavailable.
func TestSystemThemeWithDeniedStorage(t *testing.T) {
	page, _ := themeWorkspace(t, true)
	page.MustElement(".product-card").MustClick()
	page.MustElement("#product-owner").MustWait(`() => this.textContent === 'Harbour team'`)
	for _, theme := range []struct{ preference, background string }{{"dark", "rgb(0, 0, 0)"}, {"light", "rgb(255, 255, 255)"}} {
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-color-scheme", Value: theme.preference}}}).Call(
			page,
		); err != nil {
			t.Fatal(err)
		}
		waitTheme(t, page, theme.background)
	}
	if err := (proto.EmulationSetEmulatedMedia{}).Call(page); err != nil {
		t.Fatal(err)
	}
	background := "rgb(255, 255, 255)"
	if page.MustEval(`() => matchMedia('(prefers-color-scheme: dark)').matches`).Bool() {
		background = "rgb(0, 0, 0)"
	}
	page.MustElement("#appearance").MustSelect("Dark")
	waitTheme(t, page, "rgb(0, 0, 0)")
	page.MustElement("#appearance").MustSelect("System")
	waitTheme(t, page, background)
}

// waitTheme observes the workspace's actual palette rather than a theme attribute.
func waitTheme(t *testing.T, page *rod.Page, background string) {
	t.Helper()
	page.MustElement("html").
		MustWait(`(color) => getComputedStyle(this).backgroundColor === color`, background)
}

// themeWorkspace serves independently owned product metadata without registry execution.
func themeWorkspace(
	t *testing.T,
	deniedStorage bool,
) (*rod.Page, *atomic.Int32) {
	t.Helper()
	var loads atomic.Int32
	var handler http.Handler
	product := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ui" {
			loads.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(product.Close)
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	var registryHandler http.Handler
	server := httptest.NewTLSServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { registryHandler.ServeHTTP(w, r) },
		),
	)
	t.Cleanup(server.Close)
	published := workspaceProduct(product.URL)
	published.Spec.UI.Contract = &datav1alpha1.UIContract{
		APIVersion:   "data-product-ui/v2",
		HostOrigins:  []datav1alpha1.UIHostOrigin{datav1alpha1.UIHostOrigin(server.URL)},
		Capabilities: []datav1alpha1.UICapability{"status", "resize", "appearance"},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(published).Build()
	registryHandler = registry.NewHandler(reader)
	var err error
	handler, err = demoproduct.NewHandlerWithOptions(demoproduct.HandlerOptions{
		PublicBaseURL: product.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	page := contractBrowser(t).MustPage().Timeout(15 * time.Second)
	if deniedStorage {
		page.MustEvalOnNewDocument(
			`Object.defineProperty(window, 'localStorage', {get() {throw new DOMException('Denied', 'SecurityError')}})`,
		)
	}
	page.MustNavigate(server.URL).MustWaitLoad()
	page.MustElement(".product-card").MustWaitVisible()
	return page, &loads
}
