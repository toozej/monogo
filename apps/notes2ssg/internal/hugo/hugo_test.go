package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
)

func TestNewFormatter(t *testing.T) {
	f := NewFormatter("alice", []string{"thoughts"}, map[string]string{})
	if f == nil {
		t.Fatal("expected NewFormatter to return non-nil")
	}
}

func TestFormat(t *testing.T) {
	f := NewFormatter("alice", []string{"thoughts"}, map[string]string{})
	note := backend.Note{
		Title:    "Hello World",
		Date:     time.Date(2023, 9, 1, 2, 33, 35, 0, time.UTC),
		Tags:     []string{"blog", "go"},
		Content:  "This is content.",
		Unlisted: false,
	}

	md := f.Format(note)
	if !strings.Contains(md, "title: Hello World") {
		t.Errorf("expected title in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "author: alice") {
		t.Errorf("expected author in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "unlisted: false") {
		t.Errorf("expected unlisted false in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "This is content.") {
		t.Errorf("expected body in markdown, got:\n%s", md)
	}
}

func TestFormatUnlisted(t *testing.T) {
	f := NewFormatter("alice", []string{"thoughts"}, map[string]string{})
	note := backend.Note{
		Title:    "Secret",
		Date:     time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
		Tags:     []string{"thoughts"},
		Content:  "...",
		Unlisted: true,
	}

	md := f.Format(note)
	if !strings.Contains(md, "unlisted: true") {
		t.Errorf("expected unlisted true, got:\n%s", md)
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter("author", []string{}, map[string]string{})
	note := backend.Note{
		Title:   "Hello World",
		Date:    time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
		Tags:    []string{"blog"},
		Content: "body",
	}

	err := f.WriteFile(note, dir)
	if err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	path := filepath.Join(dir, "hello-world.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if !strings.Contains(string(content), "Hello World") {
		t.Errorf("expected written file to contain title")
	}
}

func TestWriteFileSkipsIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter("author", []string{}, map[string]string{})
	note := backend.Note{
		Title:   "Same",
		Date:    time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
		Tags:    []string{"blog"},
		Content: "body",
	}

	if err := f.WriteFile(note, dir); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := f.WriteFile(note, dir); err != nil {
		t.Fatalf("WriteFile error on second write: %v", err)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Test!", "test"},
		{" Multi  Space ", "multi-space"},
	}
	for _, tc := range cases {
		got := ssg.Slugify(tc.input)
		if got != tc.expected {
			t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
