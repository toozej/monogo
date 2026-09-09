package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
	"gopkg.in/yaml.v3"
)

func TestNewFormatter(t *testing.T) {
	f := NewFormatter("alice", []string{"thoughts"}, nil)
	if f == nil {
		t.Fatal("expected NewFormatter to return non-nil")
	}
}

func TestFormat(t *testing.T) {
	f := NewFormatter("alice", []string{"thoughts"}, nil)
	note := backend.Note{
		Title:    "Hello World",
		Date:     time.Date(2023, 9, 1, 2, 33, 35, 0, time.UTC),
		Tags:     []string{"blog", "go"},
		Content:  "This is content.",
		Unlisted: false,
	}

	md := f.Format(note)
	if !strings.Contains(md, `title: "Hello World"`) {
		t.Errorf("expected title in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, `author: "alice"`) {
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
	f := NewFormatter("alice", []string{"thoughts"}, nil)
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
	f := NewFormatter("author", []string{}, nil)
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
	f := NewFormatter("author", []string{}, nil)
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

func TestFormatEscapesYAMLValues(t *testing.T) {
	f := NewFormatter("alice: admin", nil, []Substitution{{Find: "note", Replace: "summary: text"}})
	note := backend.Note{
		Title:   "A: note",
		Date:    time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
		Tags:    []string{"category: name", "#private"},
		Content: "body",
	}

	frontMatter := parseFrontMatter(t, f.Format(note))
	if frontMatter["title"] != "A: note" {
		t.Errorf("title = %#v, want %q", frontMatter["title"], "A: note")
	}
	if frontMatter["author"] != "alice: admin" {
		t.Errorf("author = %#v, want %q", frontMatter["author"], "alice: admin")
	}
	if frontMatter["summary"] != "summary: text" {
		t.Errorf("summary = %#v, want %q", frontMatter["summary"], "summary: text")
	}
	categories, ok := frontMatter["categories"].([]interface{})
	if !ok || len(categories) != 2 || categories[0] != "category: name" || categories[1] != "#private" {
		t.Errorf("categories = %#v, want escaped source tags", frontMatter["categories"])
	}
}

func TestComputeSummaryUsesFirstMatchingSubstitution(t *testing.T) {
	f := NewFormatter("alice", nil, []Substitution{
		{Find: "thoughts", Replace: "first"},
		{Find: "thought", Replace: "second"},
	})
	if summary := f.computeSummary(backend.Note{Title: "Thoughts"}); summary != "first" {
		t.Errorf("summary = %q, want %q", summary, "first")
	}
}

func TestWriteFileUsesAssignedSlug(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter("author", nil, nil)
	note := backend.Note{Title: "思考", Slug: "note-1234", Content: "body"}

	if err := f.WriteFile(note, dir); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "note-1234.md")); err != nil {
		t.Fatalf("assigned slug file was not written: %v", err)
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

func parseFrontMatter(t *testing.T, markdown string) map[string]interface{} {
	t.Helper()
	trimmed := strings.TrimPrefix(markdown, "---\n")
	parts := strings.SplitN(trimmed, "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("Markdown does not contain front matter: %q", markdown)
	}
	frontMatter := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(parts[0]), &frontMatter); err != nil {
		t.Fatalf("parsing front matter: %v", err)
	}
	return frontMatter
}
