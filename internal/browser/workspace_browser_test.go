//go:build browser

package browser_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	datav1alpha1 "github.com/devantler-tech/data-product-controller/api/v1alpha1"
	"github.com/devantler-tech/data-product-controller/internal/demoproduct"
	"github.com/devantler-tech/data-product-controller/internal/registry"
	"github.com/go-rod/rod/lib/input"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestRegistryWorkspace filters real descriptors and keeps unsafe/unready products inspectable without loading them.
func TestRegistryWorkspace(t *testing.T) {
	productServer := httptest.NewTLSServer(demoproduct.NewHandler())
	t.Cleanup(productServer.Close)
	product := workspaceProduct(productServer.URL)
	other := product.DeepCopy()
	other.Name, other.Spec.Name, other.Spec.Owner.Name = "weather", "Weather readings", "Weather team"
	other.Spec.Owner.Name = strings.Repeat("Weather", 24)
	other.Status.Conditions = nil
	other.Spec.Outputs[0].ContractURL = "javascript:alert(1)"
	other.Spec.Description = "<img src=x onerror=alert(1)>"
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(product, other).Build()
	server := httptest.NewTLSServer(
		registry.NewHandler(reader),
	)
	t.Cleanup(server.Close)
	page := contractBrowser(t).MustPage().MustNavigate(server.URL).MustWaitLoad()
	page.MustElement(".product-card").MustWaitVisible()
	if !page.MustEval(`() => !!document.querySelector('#product-search')`).Bool() {
		t.Fatal("workspace has no product search")
	}
	page.MustElement("#product-search").MustInput("HARBOUR TEAM")
	page.MustElement("#product-count").MustWait(`() => this.textContent === '1 of 2 products'`)
	page.MustElement(".product-card").MustFocus()
	page.Keyboard.MustType(input.Enter)
	if page.MustEval(`() => document.activeElement.id`).Str() != "interaction-title" {
		t.Fatal("product selection did not focus its details")
	}
	if got := page.MustElement("#product-owner").MustText(); got != "Harbour team" {
		t.Fatalf("owner = %q", got)
	}
	if got := page.MustElement("#product-interfaces a[data-kind=contract]").
		MustAttribute("href"); got == nil ||
		*got != productServer.URL+"/openapi.json" {
		t.Fatalf("contract link = %v", got)
	}
	if len(page.MustElements("iframe")) != 0 {
		t.Fatal("API workspace executed a product UI")
	}
	openContract := page.MustWaitOpen()
	page.MustElement("#product-interfaces a[data-kind=contract]").MustClick()
	contractPage := openContract().MustWaitLoad()
	if !strings.Contains(contractPage.MustElement("body").MustText(), `"openapi"`) {
		t.Fatal("workspace contract link did not open its real API contract")
	}
	contractPage.MustClose()
	page.MustElement("#product-search").MustSelectAllText().MustInput("not present")
	page.MustElement("#registry-status").
		MustWait(`() => this.textContent.includes('No products match')`)
	page.MustElement("#clear-filters").MustClick()
	page.MustElement("#readiness-filter").MustSelect("Not ready")
	page.MustElement("#product-count").MustWait(`() => this.textContent === '1 of 2 products'`)
	page.MustElement(".product-card").MustFocus()
	page.Keyboard.MustType(input.Enter)
	if len(page.MustElements("iframe")) != 0 {
		t.Fatal("unready product created a frame")
	}
	if page.MustEval(`() => !!document.querySelector('a[href^="javascript:"]') || !!document.querySelector('#interaction-description img')`).
		Bool() {
		t.Fatal("descriptor metadata became executable content")
	}
	// A narrow viewport must contain the workspace without a horizontal page scroll.
	page.MustSetViewport(375, 812, 1, false)
	if page.MustEval(`() => document.documentElement.scrollWidth > innerWidth`).Bool() {
		t.Fatal("mobile workspace overflows horizontally")
	}
}

// TestRegistryRetry clears failed state and re-reads the real API when the user retries.
func TestRegistryRetry(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(workspaceProduct("https://example.com")).
		Build()
	handler := registry.NewHandler(reader)
	var fail atomic.Bool
	fail.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/products" && fail.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	page := contractBrowser(t).MustPage().MustNavigate(server.URL).MustWaitLoad()
	if !page.MustEval(`() => !!document.querySelector('#refresh-products')`).Bool() {
		t.Fatal("workspace has no retry action")
	}
	page.MustElement("#registry-status").
		MustWait(`() => this.textContent.includes('Could not load')`)
	page.MustElement("#product-search").MustInput("harbour")
	if !strings.Contains(page.MustElement("#registry-status").MustText(), "Could not load") {
		t.Fatal("filtering an unavailable inventory hid the retry message")
	}
	fail.Store(false)
	page.MustElement("#refresh-products").MustClick()
	page.MustElement(".product-card").MustWaitVisible()
	if page.MustElement("#product-count").MustText() != "1 product" {
		t.Fatal("retry did not restore the inventory")
	}
}

// workspaceProduct supplies a complete published descriptor through the real registry handler.
func workspaceProduct(origin string) *datav1alpha1.DataProduct {
	return &datav1alpha1.DataProduct{
		ObjectMeta: metav1.ObjectMeta{Name: "harbour", Namespace: "products"},
		Spec: datav1alpha1.DataProductSpec{
			ID:          origin + "/products/harbour",
			Name:        "Harbour observations",
			Description: "Temperature and salinity",
			Version:     "v1.0.0",
			Owner:       datav1alpha1.ProductOwner{Name: "Harbour team"},
			Outputs: []datav1alpha1.OutputPort{
				{
					Name:        "observations",
					Protocol:    datav1alpha1.ProtocolOpenAPI,
					URL:         origin + "/api/observations",
					ContractURL: origin + "/openapi.json",
					MediaType:   "application/json",
				},
			},
			UI: &datav1alpha1.ProductUI{Title: "Harbour observations", URL: origin + "/ui"},
		},
		Status: datav1alpha1.DataProductStatus{Conditions: []metav1.Condition{
			{
				Type:    datav1alpha1.ConditionReady,
				Status:  metav1.ConditionTrue,
				Reason:  "DependenciesReady",
				Message: "Declared dependencies are ready.",
			},
		}},
	}
}
