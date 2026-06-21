package web

import (
	"html/template"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// TemplateData represents the data structure passed to HTML templates
type TemplateData struct {
	Title   string // Page title
	Debug   bool   // Debug mode flag
	Version string // Application version
}

var (
	templates *template.Template
)

// LoadTemplates loads and parses embedded HTML templates
func LoadTemplates() (*template.Template, error) {
	if templates != nil {
		return templates, nil
	}

	// Get the index.html content from embedded assets
	indexHTML, err := GetAsset("index.html")
	if err != nil {
		return nil, err
	}

	// Parse the template
	tmpl, err := template.New("index.html").Parse(string(indexHTML))
	if err != nil {
		return nil, err
	}

	templates = tmpl
	return templates, nil
}

// RenderTemplate renders the specified template with the provided data
func RenderTemplate(w http.ResponseWriter, name string, data TemplateData) error {
	// Load templates if not already loaded
	tmpl, err := LoadTemplates()
	if err != nil {
		log.Errorf("Error loading templates: %v", err)
		return err
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template
	if err := tmpl.Execute(w, data); err != nil {
		log.Errorf("Error executing template %s: %v", name, err)
		return err
	}

	return nil
}

// ReloadTemplates forces a reload of templates (useful for development)
func ReloadTemplates() error {
	templates = nil
	_, err := LoadTemplates()
	return err
}
