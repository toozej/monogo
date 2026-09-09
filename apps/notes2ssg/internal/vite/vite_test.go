package vite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"gopkg.in/yaml.v3"
)

func TestFormat(t *testing.T) {
	f := NewFormatter("")
	note := backend.Note{
		Title:   "My Post",
		Date:    time.Date(2024, 7, 4, 0, 0, 0, 0, time.UTC),
		Content: "body",
	}

	md := f.Format(note)
	if !strings.Contains(md, "template:") {
		t.Fatalf("expected template front matter, got: %s", md)
	}
	if !strings.Contains(md, `slug: "my-post"`) {
		t.Errorf("expected slug, got: %s", md)
	}
	if !strings.Contains(md, `date: "2024-07-04"`) {
		t.Errorf("expected date, got: %s", md)
	}
}

func TestFormatEscapesYAMLValues(t *testing.T) {
	f := NewFormatter("subtitle: text")
	note := backend.Note{Title: "Title: text", Slug: "title", Content: "body"}

	parts := strings.SplitN(f.Format(note), "\n\n", 2)
	if len(parts) != 2 {
		t.Fatal("formatted note does not contain a body separator")
	}
	var frontMatter map[string]map[string]interface{}
	if err := yaml.Unmarshal([]byte(parts[0]), &frontMatter); err != nil {
		t.Fatalf("parsing front matter: %v", err)
	}
	template := frontMatter["template"]
	if template["title"] != "Title: text" {
		t.Errorf("title = %#v, want %q", template["title"], "Title: text")
	}
	if template["subtitle"] != "subtitle: text" {
		t.Errorf("subtitle = %#v, want %q", template["subtitle"], "subtitle: text")
	}
}

func TestWriteFile(t *testing.T) {
	f := NewFormatter("subtitle")
	note := backend.Note{
		Title:   "Hello",
		Date:    time.Date(2024, 7, 4, 0, 0, 0, 0, time.UTC),
		Content: "content",
	}

	dir := t.TempDir()
	if err := f.WriteFile(note, dir); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path := filepath.Join(dir, "hello.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.Contains(string(data), "subtitle") {
		t.Errorf("expected subtitle in output, got: %s", data)
	}
}
