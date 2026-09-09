package ssg

import (
	"testing"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"gopkg.in/yaml.v3"
)

func TestSlugForPrefersAssignedSlug(t *testing.T) {
	note := backend.Note{Title: "A title", Slug: "assigned-slug"}
	if got := SlugFor(note); got != "assigned-slug" {
		t.Errorf("SlugFor() = %q, want %q", got, "assigned-slug")
	}
}

func TestQuoteYAMLPreservesSpecialText(t *testing.T) {
	const value = "title: text\nsecond line"
	var result struct {
		Value string `yaml:"value"`
	}
	if err := yaml.Unmarshal([]byte("value: "+QuoteYAML(value)), &result); err != nil {
		t.Fatalf("parsing quoted YAML: %v", err)
	}
	if result.Value != value {
		t.Errorf("parsed value = %q, want %q", result.Value, value)
	}
}
