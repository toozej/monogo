package note

import (
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("expected NewParser to return non-nil")
	}
}

func TestParseNotes(t *testing.T) {
	input := `+--------------------------------------------------------------+
| Title: Hello World |
| Date: Fri, 01 Sep 2023 02:33:35 |
| Tags: blog, go |
+--------------------------------------------------------------+
# Hello World
This is the body.
+--------------------------------------------------------------+
| Title: Second Note |
| Date: Sat, 02 Sep 2023 10:00:00 |
| Tags: test |
+--------------------------------------------------------------+
Second note body.`

	p := NewParser()
	notes := p.ParseNotes(input)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}

	first := notes[0]
	if first.Title != "Hello World" {
		t.Errorf("expected title 'Hello World', got %q", first.Title)
	}
	if len(first.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(first.Tags))
	}
	if first.Tags[0] != "blog" || first.Tags[1] != "go" {
		t.Errorf("expected tags [blog go], got %v", first.Tags)
	}
	expectedBody := `This is the body.`
	if first.Content != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, first.Content)
	}

	second := notes[1]
	if second.Title != "Second Note" {
		t.Errorf("expected title 'Second Note', got %q", second.Title)
	}
	if len(second.Tags) != 1 || second.Tags[0] != "test" {
		t.Errorf("expected tags [test], got %v", second.Tags)
	}
}

func TestParseNotesNoNotes(t *testing.T) {
	p := NewParser()
	notes := p.ParseNotes("  \n  \n")
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(notes))
	}
}

func TestParseNotesSingleNote(t *testing.T) {
	input := `+-----+
| Title: Single |
+-----+
Body here.`
	p := NewParser()
	notes := p.ParseNotes(input)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Title != "Single" {
		t.Errorf("expected title 'Single', got %q", notes[0].Title)
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input    string
		expected time.Time
	}{
		{"Fri, 01 Sep 2023 02:33:35", time.Date(2023, 9, 1, 2, 33, 35, 0, time.UTC)},
		{"2023-09-01T02:33:35Z", time.Date(2023, 9, 1, 2, 33, 35, 0, time.UTC)},
		{"", time.Time{}},
		{"bad date", time.Time{}},
	}

	for _, tc := range cases {
		got := parseDate(tc.input)
		if tc.expected.IsZero() {
			if !got.IsZero() {
				t.Errorf("parseDate(%q) expected zero time, got %v", tc.input, got)
			}
			continue
		}
		year, month, day := got.Date()
		expYear, expMonth, expDay := tc.expected.Date()
		if year != expYear || month != expMonth || day != expDay {
			t.Errorf("parseDate(%q) date mismatch: got %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":           "hello-world",
		"Test Note!":            "test-note",
		"  Spaces  ":            "spaces",
		"---multiple-dashes---": "multiple-dashes",
	}
	for input, expected := range cases {
		got := Slugify(input)
		if got != expected {
			t.Errorf("Slugify(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestContinuousNote(t *testing.T) {
	note := backend.Note{
		Title:   "Thoughts",
		Content: "First line\n\nSecond line\n",
		Tags:    []string{"blog:thoughts"},
	}
	split := ContinuousNote(note, "blog:thoughts", "thoughts")
	if len(split) != 2 {
		t.Fatalf("expected 2 continuous notes, got %d", len(split))
	}
	if split[0].Title != "Thoughts - 2" {
		t.Errorf("expected title 'Thoughts - 2', got %q", split[0].Title)
	}
	if split[0].Content != "First line" {
		t.Errorf("expected content 'First line', got %q", split[0].Content)
	}
	if split[1].Title != "Thoughts - 1" {
		t.Errorf("expected title 'Thoughts - 1', got %q", split[1].Title)
	}
	if split[1].Content != "Second line" {
		t.Errorf("expected content 'Second line', got %q", split[1].Content)
	}
	for i, splitNote := range split {
		if len(splitNote.Tags) != 1 || splitNote.Tags[0] != "thoughts" {
			t.Errorf("split note %d tags = %v, want [thoughts]", i, splitNote.Tags)
		}
	}
}

func TestContinuousNoteEmptyBody(t *testing.T) {
	note := backend.Note{
		Title:   "Empty",
		Content: "",
	}
	split := ContinuousNote(note, "tag", "replacement")
	if len(split) != 0 {
		t.Errorf("expected 0 notes, got %d", len(split))
	}
}
