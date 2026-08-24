package vite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
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
	if !strings.Contains(md, "slug: my-post") {
		t.Errorf("expected slug, got: %s", md)
	}
	if !strings.Contains(md, "date: 2024-07-04") {
		t.Errorf("expected date, got: %s", md)
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
