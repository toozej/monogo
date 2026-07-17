// Package swaggertest provides shared assertions for the generated Swagger
// documents served by the apps. The per-app document tests all decode the same
// spec shape and check the API title, that the expected paths are present, and
// that the BasicAuth security scheme is defined; centralizing that here keeps the
// checks from drifting across apps.
package swaggertest

import (
	"encoding/json"
	"testing"
)

// AssertDocument decodes a served Swagger/OpenAPI document and verifies that it
// declares the expected API title, contains at least one path (and each path in
// wantPaths), and defines the BasicAuth security scheme with type "basic".
func AssertDocument(t testing.TB, body []byte, wantTitle string, wantPaths ...string) {
	t.Helper()

	var spec struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths               map[string]json.RawMessage `json:"paths"`
		SecurityDefinitions map[string]struct {
			Type string `json:"type"`
		} `json:"securityDefinitions"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("decode Swagger document: %v", err)
	}
	if spec.Info.Title != wantTitle {
		t.Errorf("Swagger title = %q, want %q", spec.Info.Title, wantTitle)
	}
	if len(spec.Paths) == 0 {
		t.Error("Swagger document contains no paths")
	}
	for _, path := range wantPaths {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("Swagger document is missing %s", path)
		}
	}
	if definition, ok := spec.SecurityDefinitions["BasicAuth"]; !ok {
		t.Error("Swagger document is missing the BasicAuth security definition")
	} else if definition.Type != "basic" {
		t.Errorf("BasicAuth type = %q, want %q", definition.Type, "basic")
	}
}
