package slug

import (
	"strings"
	"testing"
)

func TestFromTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "Readability", "readability"},
		{"spaces", "Hello World", "hello-world"},
		{"special chars", "What's New? (2024!)", "what-s-new-2024"},
		{"empty", "", "untitled"},
		{"dashes", "--leading-and-trailing--", "leading-and-trailing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromTitle(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFromTitle_LongTitle(t *testing.T) {
	input := strings.Repeat("a", 200)
	got := FromTitle(input)
	if len(got) > 80 {
		t.Errorf("slug exceeds 80 chars: %d", len(got))
	}
	if got == "untitled" {
		t.Error("long title should not become untitled")
	}
}

func TestFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"https url", "https://en.wikipedia.org/wiki/Readability", "en-wikipedia-org-wiki-readability"},
		{"http url", "http://example.com/article", "example-com-article"},
		{"trailing slash", "https://example.com/path/", "example-com-path"},
		{"empty after strip", "https://", "untitled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromURL(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFromFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"html file", "/home/user/articles/essay.html", "essay"},
		{"txt file", "/tmp/readme.txt", "readme"},
		{"no extension", "/path/to/document", "document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromFilePath(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
