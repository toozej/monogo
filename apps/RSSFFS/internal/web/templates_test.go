package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadTemplates(t *testing.T) {
	// Reset templates to ensure clean test
	templates = nil

	tmpl, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates returned error: %v", err)
	}

	if tmpl == nil {
		t.Fatal("LoadTemplates returned nil template")
	}

	// Test that subsequent calls return the same template (caching)
	tmpl2, err := LoadTemplates()
	if err != nil {
		t.Fatalf("Second LoadTemplates call returned error: %v", err)
	}

	if tmpl != tmpl2 {
		t.Error("Expected LoadTemplates to return cached template on second call")
	}
}

func TestRenderTemplate(t *testing.T) {
	// Reset templates to ensure clean test
	templates = nil

	testData := TemplateData{
		Title:   "Test Title",
		Debug:   true,
		Version: "1.0.0",
	}

	w := httptest.NewRecorder()
	err := RenderTemplate(w, "index.html", testData)

	if err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}

	// Check Content-Type header
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type to contain 'text/html', got %s", contentType)
	}

	// Check that template was rendered with data
	body := w.Body.String()
	if body == "" {
		t.Error("Expected rendered template to have content")
	}

	// Check that template contains expected content
	expectedContent := []string{
		testData.Title,
		testData.Version,
	}

	for _, expected := range expectedContent {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected rendered template to contain %q", expected)
		}
	}

	// Check that template does not contain old CSRF token placeholder
	if strings.Contains(body, "{{.CSRFToken}}") {
		t.Error("Rendered template should not contain raw CSRF token placeholder")
	}
}

func TestRenderTemplateWithEmptyData(t *testing.T) {
	// Reset templates to ensure clean test
	templates = nil

	testData := TemplateData{}

	w := httptest.NewRecorder()
	err := RenderTemplate(w, "index.html", testData)

	if err != nil {
		t.Fatalf("RenderTemplate with empty data returned error: %v", err)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Expected rendered template to have content even with empty data")
	}
}

func TestReloadTemplates(t *testing.T) {
	// Load templates first
	_, err := LoadTemplates()
	if err != nil {
		t.Fatalf("Initial LoadTemplates returned error: %v", err)
	}

	// Reload templates
	err = ReloadTemplates()
	if err != nil {
		t.Fatalf("ReloadTemplates returned error: %v", err)
	}

	// Verify templates are still functional after reload
	testData := TemplateData{
		Title:   "Test After Reload",
		Version: "1.0.0",
	}

	w := httptest.NewRecorder()
	err = RenderTemplate(w, "index.html", testData)

	if err != nil {
		t.Fatalf("RenderTemplate after reload returned error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, testData.Title) {
		t.Error("Expected reloaded template to render correctly")
	}
}

func TestTemplateDataStructure(t *testing.T) {
	// Test that TemplateData struct has expected fields
	data := TemplateData{
		Title:   "Test Title",
		Debug:   true,
		Version: "1.0.0",
	}

	if data.Title != "Test Title" {
		t.Errorf("Expected Title to be 'Test Title', got %s", data.Title)
	}

	if !data.Debug {
		t.Error("Expected Debug to be true")
	}

	if data.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %s", data.Version)
	}
}
