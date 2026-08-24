package converter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/config"
	"github.com/toozej/monogo/apps/notes2ssg/internal/hugo"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
	"github.com/toozej/monogo/apps/notes2ssg/internal/vite"
)

type mockBackend struct {
	notes []backend.Note
	err   error
}

func (m *mockBackend) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	return m.notes, m.err
}

func TestNewApp(t *testing.T) {
	cfg := config.Config{
		Backend: "simplenote",
		Author:  "alice",
	}
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
	if app.config.Author != "alice" {
		t.Errorf("expected author alice, got %q", app.config.Author)
	}
}

func TestNewAppUnsupportedBackend(t *testing.T) {
	cfg := config.Config{Backend: "unknown"}
	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Backend:   "mock",
		OutputDir: dir,
		Author:    "alice",
	}
	notes := []backend.Note{
		{Title: "Note 1", Date: zeroTime(), Tags: []string{"blog"}, Content: "body 1"},
		{Title: "Note 2", Date: zeroTime(), Tags: []string{"blog"}, Content: "body 2"},
	}
	app := newAppWithDeps(cfg, &mockBackend{notes: notes})

	err := app.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading output dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 output files, got %d", len(entries))
	}
}

func TestRunFetchError(t *testing.T) {
	cfg := config.Config{Backend: "mock"}
	app := newAppWithDeps(cfg, &mockBackend{err: os.ErrNotExist})

	err := app.Run(context.Background())
	if err == nil {
		t.Fatal("expected error when backend fails")
	}
}

func TestProcessNotesUnlisted(t *testing.T) {
	cfg := config.Config{UnlistedTags: "thoughts"}
	app := newAppWithDeps(cfg, &mockBackend{})

	note := backend.Note{Title: "A", Tags: []string{"thoughts"}}
	processed := app.processNotes([]backend.Note{note})
	if len(processed) != 1 {
		t.Fatalf("expected 1 processed note, got %d", len(processed))
	}
	if !processed[0].Unlisted {
		t.Error("expected note to be unlisted")
	}
}

func TestProcessNotesContinuous(t *testing.T) {
	cfg := config.Config{
		ContinuousNoteTag: "blog:thoughts",
	}
	app := newAppWithDeps(cfg, &mockBackend{})

	note := backend.Note{Title: "Thoughts", Tags: []string{"blog:thoughts"}, Content: "Line 1\nLine 2"}
	processed := app.processNotes([]backend.Note{note})
	if len(processed) != 2 {
		t.Fatalf("expected 2 processed notes, got %d", len(processed))
	}
}

func TestParseList(t *testing.T) {
	got := parseList("a, b, c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parseList mismatch at %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseSubstitutions(t *testing.T) {
	m := parseSubstitutions("find:replace, a: b")
	if m["find"] != "replace" {
		t.Errorf("expected find->replace, got %q", m["find"])
	}
	if m["a"] != "b" {
		t.Errorf("expected a->b, got %q", m["a"])
	}
}

func TestFormatterSelection(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.Config
		matcher func(ssg.Formatter) bool
	}{
		{
			name: "default",
			cfg:  config.Config{Author: "alice"},
			matcher: func(f ssg.Formatter) bool {
				_, ok := f.(*hugo.Formatter)
				return ok
			},
		},
		{
			name: "explicit hugo",
			cfg:  config.Config{Author: "alice", SSGType: "hugo"},
			matcher: func(f ssg.Formatter) bool {
				_, ok := f.(*hugo.Formatter)
				return ok
			},
		},
		{
			name: "vite",
			cfg:  config.Config{SSGType: "vite", ViteSubtitle: "sub"},
			matcher: func(f ssg.Formatter) bool {
				_, ok := f.(*vite.Formatter)
				return ok
			},
		},
		{
			name: "unknown fallback",
			cfg:  config.Config{SSGType: "unknown"},
			matcher: func(f ssg.Formatter) bool {
				_, ok := f.(*hugo.Formatter)
				return ok
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newAppWithDeps(tc.cfg, &mockBackend{})
			if app.formatter == nil {
				t.Fatalf("expected formatter to be set")
			}
			if !tc.matcher(app.formatter) {
				t.Fatalf("formatter type mismatch for %s", tc.name)
			}
		})
	}
}

func zeroTime() time.Time {
	return time.Time{}
}
