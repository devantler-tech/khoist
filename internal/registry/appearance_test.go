package registry

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	datav1alpha1 "github.com/devantler-tech/data-product-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestWorkspaceCapabilityConfiguration preserves the independent discovery and lineage gates.
func TestWorkspaceCapabilityConfiguration(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := datav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, discovery := range []bool{false, true} {
		for _, lineage := range []bool{false, true} {
			handler := NewHandlerWithOptions(
				fake.NewClientBuilder().WithScheme(scheme).Build(),
				HandlerOptions{
					DiscoveryEnabled: func(context.Context) bool { return discovery },
					LineageEnabled:   func(context.Context) bool { return lineage },
					InputCompatibility: func(datav1alpha1.InputPort, *datav1alpha1.DataProduct) string {
						return "compatible"
					},
				},
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/workspace-config", nil),
			)
			if response.Code != 200 {
				t.Fatalf("presentation configuration unavailable: %d", response.Code)
			}
			var config map[string]bool
			if err := json.Unmarshal(response.Body.Bytes(), &config); err != nil {
				t.Fatal(err)
			}
			if len(config) != 2 || config["discoveryEnabled"] != discovery ||
				config["lineageEnabled"] != (discovery && lineage) {
				t.Fatalf(
					"incorrect workspace policy for discovery=%t lineage=%t: %v",
					discovery,
					lineage,
					config,
				)
			}
		}
	}
}
