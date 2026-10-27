package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	datav1alpha1 "github.com/devantler-tech/data-product-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestRegistryOmitsLegacyUI exercises every metadata projection without changing stored declarations.
func TestRegistryOmitsLegacyUI(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	product := registryProduct()
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(product).Build()
	handler := traceHandler(reader)
	for _, path := range []string{
		"/api/v1/products",
		"/api/v2/products?namespace=products",
		"/api/v2/products/products/customer-catalog",
		"/api/v2/products/products/customer-catalog/lineage",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			response := discoveryRequest(t, handler, path)
			if response.Code != http.StatusOK {
				t.Fatalf("metadata response = %d: %s", response.Code, response.Body.String())
			}
			var document any
			if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
				t.Fatal(err)
			}
			assertNoLegacyUI(t, document)
		})
	}
}

// assertNoLegacyUI checks nested lineage descriptors as well as list and detail responses.
func assertNoLegacyUI(t *testing.T, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		if _, ok := value["ui"]; ok {
			t.Fatal("registry emitted retired product UI metadata")
		}
		for _, child := range value {
			assertNoLegacyUI(t, child)
		}
	case []any:
		for _, child := range value {
			assertNoLegacyUI(t, child)
		}
	}
}

// TestPublisherPreviewPreservesLegacyUI prevents registry retirement from silently changing reports.
func TestPublisherPreviewPreservesLegacyUI(t *testing.T) {
	t.Parallel()
	product := registryProduct()
	expected, err := json.Marshal(product.Spec.UI)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := PublicationPreview(product)
	if err != nil {
		t.Fatal(err)
	}
	var descriptor map[string]json.RawMessage
	if err := json.Unmarshal(preview, &descriptor); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(descriptor["ui"], expected) {
		t.Fatalf("publisher UI changed: got %s, want %s", descriptor["ui"], expected)
	}
}

// TestPublisherRejectsInvalidLegacyUI keeps retired metadata bounded in compatible reports.
func TestPublisherRejectsInvalidLegacyUI(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*datav1alpha1.DataProduct)
	}{
		{"backslash-url", func(p *datav1alpha1.DataProduct) {
			p.Spec.UI.URL = "https://example.test/path\\tail"
		}},
		{"blank-ui-title", func(p *datav1alpha1.DataProduct) { p.Spec.UI.Title = " " }},
		{"long-ui-title", func(p *datav1alpha1.DataProduct) { p.Spec.UI.Title = strings.Repeat("x", 201) }},
		{"long-utf16-ui-title", func(p *datav1alpha1.DataProduct) {
			p.Spec.UI.Title = strings.Repeat("🌊", 101)
		}},
		{"long-ui-url", func(p *datav1alpha1.DataProduct) {
			p.Spec.UI.URL = "https://example.test/" + strings.Repeat("x", 2048)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			product := registryProduct()
			test.mutate(product)
			if _, err := PublicationPreview(product); err == nil {
				t.Fatal("publisher accepted invalid legacy UI metadata")
			}
			data, err := encodePortableDescriptor(product)
			if err != nil {
				t.Fatalf("omitted UI metadata must not invalidate API-only discovery: %v", err)
			}
			var descriptor any
			if err := json.Unmarshal(data, &descriptor); err != nil {
				t.Fatal(err)
			}
			assertNoLegacyUI(t, descriptor)
		})
	}
}

// TestRegistryWorkspaceConfiguration preserves current capability gates without presentation settings.
func TestRegistryWorkspaceConfiguration(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run("retired-"+method, func(t *testing.T) {
			t.Parallel()
			response := httptest.NewRecorder()
			NewHandler(nil).ServeHTTP(response,
				httptest.NewRequestWithContext(t.Context(), method, "/api/v1/ui-config", nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("retired configuration = %d, want 404", response.Code)
			}
		})
	}
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandler(nil)
			if enabled {
				handler = traceHandler(nil)
			}
			response := discoveryRequest(t, handler, "/api/v1/workspace-config")
			if response.Code != http.StatusOK ||
				response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("workspace configuration unavailable or cacheable: %d", response.Code)
			}
			var configuration map[string]bool
			if err := json.Unmarshal(response.Body.Bytes(), &configuration); err != nil {
				t.Fatal(err)
			}
			if len(configuration) != 2 || configuration["discoveryEnabled"] != enabled ||
				configuration["lineageEnabled"] != enabled {
				t.Fatalf("unexpected workspace capability settings: %#v", configuration)
			}
		})
	}
}
