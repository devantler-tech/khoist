//go:build browser

package browser_test

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

// TestDescriptorInspectionWithoutPresentationRuntime preserves strict admission in an inert publisher host.
func TestDescriptorInspectionWithoutPresentationRuntime(t *testing.T) {
	host := publisherHost(t)
	page := contractBrowser(
		t,
	).MustPage().
		Timeout(20 * time.Second).
		MustNavigate(host.URL + "/publisher-review").
		MustWaitLoad()
	defer page.MustClose()
	const approved = "https://host.example.test"
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		raw    string
		mount  bool
		valid  bool
	}{
		{name: "ready approved host", mount: true, valid: true},
		{name: "UI-less", valid: true, mutate: func(d map[string]any) { delete(d, "ui") }},
		{name: "unhealthy inspection", valid: true, mutate: func(d map[string]any) {
			d["ready"] = false
			descriptorObject(t, d, "readiness")["reason"] = "not-ready"
		}},
		{name: "wrong host", mount: true, mutate: func(d map[string]any) {
			descriptorObject(t, d, "ui", "contract")["hostOrigins"] = []string{"https://other.example.test"}
		}},
		{name: "bad capability", mutate: func(d map[string]any) {
			descriptorObject(t, d, "ui", "contract")["capabilities"] = []string{"credentials"}
		}},
		{name: "credential URL", mutate: func(d map[string]any) {
			endpoint := url.URL{Scheme: "https", Host: "example.test", Path: "/query", User: url.UserPassword("fixture", "invalid")}
			descriptorEntry(t, d["outputs"])["url"] = endpoint.String()
		}},
		{name: "malformed URL", mutate: func(d map[string]any) { descriptorObject(t, d, "owner")["url"] = "https:///example.test/owner" }},
		{name: "stale health", mutate: func(d map[string]any) { descriptorObject(t, d, "health", "source")["observedGeneration"] = 6 }},
		{name: "duplicate JSON", raw: `{"generation":7,"generation":8}`},
		{name: "unsafe number", raw: `{"generation":9007199254740992}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			descriptor := offlineDescriptor(approved, "https://publisher.example.test")
			if tc.mutate != nil {
				tc.mutate(descriptor)
			}
			wire := descriptorJSON(t, descriptor)
			if tc.raw != "" {
				wire = tc.raw
			}
			got := page.MustEval(`(wire,host,mount)=>{try{DataProductDescriptor.validate(DataProductDescriptor.parseJSON(wire));if(mount)DataProductDescriptor.parse(wire,host);return true;}catch{return false;}}`, wire, approved, tc.mount).
				Bool()
			if got != tc.valid {
				t.Fatalf("metadata admission=%t want=%t", got, tc.valid)
			}
		})
	}
	if !page.MustEval(`()=>Object.isFrozen(DataProductMetadata)&&typeof DataProductUI==='undefined'&&document.querySelectorAll('iframe').length===0`).
		Bool() {
		t.Fatal("inert inspection loaded a presentation runtime")
	}
}

// offlineDescriptor is a literal exported v1 document, independent of registry builders.
func offlineDescriptor(host, publisher string) map[string]any {
	health := map[string]any{}
	for _, dimension := range []string{"source", "connector", "contracts", "composition"} {
		health[dimension] = map[string]any{
			"state":              "ready",
			"message":            "Current observation",
			"generation":         7,
			"observedGeneration": 7,
		}
	}
	return map[string]any{
		"apiVersion":  "data-product-descriptor/v1",
		"kind":        "DataProduct",
		"namespace":   "products",
		"name":        "harbour",
		"id":          "urn:example:harbour",
		"displayName": "Harbour observations",
		"description": "Public observations",
		"version":     "v1.0.0",
		"owner":       map[string]any{"name": "Marine team", "url": publisher + "/owner"},
		"outputs": []any{
			map[string]any{
				"name":        "observations",
				"protocol":    "OpenAPI",
				"url":         publisher + "/query",
				"contractUrl": publisher + "/schema",
			},
		},
		"ui": map[string]any{
			"url":   publisher + "/ui",
			"title": "Harbour observations",
			"contract": map[string]any{
				"apiVersion":   "data-product-ui/v1",
				"hostOrigins":  []string{host},
				"capabilities": []string{"status", "resize"},
			},
		},
		"ready": true,
		"readiness": map[string]any{
			"reason":  "ready",
			"message": "Current metadata observation",
		},
		"generation":         7,
		"observedGeneration": 7,
		"health":             health,
	}
}

func descriptorJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// descriptorObject checks the public fixture path before applying an independent mutation.
func descriptorObject(t *testing.T, value any, keys ...string) map[string]any {
	t.Helper()
	for _, key := range keys {
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatal("descriptor fixture path is not an object")
		}
		value = object[key]
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatal("descriptor fixture value is not an object")
	}
	return object
}

func descriptorEntry(t *testing.T, value any) map[string]any {
	t.Helper()
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		t.Fatal("descriptor fixture array is empty or malformed")
	}
	return descriptorObject(t, values[0])
}
