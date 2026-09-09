package converter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes2ssg/internal/backend"
	"github.com/toozej/monogo/apps/notes2ssg/internal/config"
	"github.com/toozej/monogo/apps/notes2ssg/internal/hugo"
	"github.com/toozej/monogo/apps/notes2ssg/internal/ssg"
	"github.com/toozej/monogo/apps/notes2ssg/internal/vite"
)

type mockBackend struct {
	notes     []backend.Note
	err       error
	fetchFunc func(context.Context, string) ([]backend.Note, error)
}

func (m *mockBackend) FetchNotes(ctx context.Context, tag string) ([]backend.Note, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, tag)
	}
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

	if files := markdownFiles(t, dir); len(files) != 2 {
		t.Fatalf("expected 2 Markdown files, got %d", len(files))
	}
}

func TestRunIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	app := newAppWithDeps(config.Config{OutputDir: dir}, &mockBackend{
		notes: []backend.Note{{Title: "Repeat", Content: "body"}},
	})

	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if files := markdownFiles(t, dir); len(files) != 1 {
		t.Fatalf("expected 1 Markdown file, got %d", len(files))
	}
}

func TestRunRemovesOnlyStaleGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	userFile := filepath.Join(dir, "user-file.md")
	if err := os.WriteFile(userFile, []byte("user content"), 0o600); err != nil {
		t.Fatalf("writing user file: %v", err)
	}
	source := &mockBackend{
		notes: []backend.Note{{Title: "Generated", Content: "first\nsecond", Tags: []string{"blog:thoughts"}}},
	}
	app := newAppWithDeps(config.Config{
		ContinuousNoteTag: "blog:thoughts",
		OutputDir:         dir,
	}, source)

	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	source.notes = []backend.Note{{Title: "Generated", Content: "replacement", Tags: []string{"blog:thoughts"}}}
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	if _, err := os.Stat(userFile); err != nil {
		t.Fatalf("user file was removed: %v", err)
	}
	if files := markdownFiles(t, dir); len(files) != 2 {
		t.Fatalf("expected one generated file and one user file, got %d Markdown files", len(files))
	}
}

func TestRunAssignsUniqueSlugs(t *testing.T) {
	dir := t.TempDir()
	app := newAppWithDeps(config.Config{OutputDir: dir}, &mockBackend{
		notes: []backend.Note{
			{Title: "Same", Content: "first"},
			{Title: "Same!", Content: "second"},
			{Title: "思考", Content: "third"},
		},
	})

	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	files := markdownFiles(t, dir)
	if len(files) != 3 {
		t.Fatalf("expected 3 unique Markdown files, got %d", len(files))
	}
	for _, file := range files {
		if file == ".md" {
			t.Fatal("generated an empty slug filename")
		}
	}
	combined := ""
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("reading generated file %q: %v", file, err)
		}
		combined += string(data)
	}
	for _, content := range []string{"first", "second", "third"} {
		if !strings.Contains(combined, content) {
			t.Errorf("generated files do not contain %q", content)
		}
	}
}

func TestRunRequiresOutputDirectory(t *testing.T) {
	app := newAppWithDeps(config.Config{}, &mockBackend{})
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected an error when OUTPUT_DIR is empty")
	}
}

func TestRunFetchError(t *testing.T) {
	cfg := config.Config{Backend: "mock", OutputDir: t.TempDir()}
	app := newAppWithDeps(cfg, &mockBackend{err: os.ErrNotExist})

	err := app.Run(context.Background())
	if err == nil {
		t.Fatal("expected error when backend fails")
	}
}

func TestRunPollingStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	backend := &mockBackend{
		fetchFunc: func(context.Context, string) ([]backend.Note, error) {
			calls++
			cancel()
			return []backend.Note{{Title: "Polling", Content: "body"}}, nil
		},
	}
	app := newAppWithDeps(config.Config{OutputDir: t.TempDir(), PollingCycle: 1}, backend)

	if err := app.RunPolling(ctx); err != nil {
		t.Fatalf("polling run failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 export before cancellation, got %d", calls)
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

func TestProcessNotesMultipleContinuousCategories(t *testing.T) {
	cfg := config.Config{
		ContinuousNoteTag: "blog:thoughts, blog:camping",
	}
	app := newAppWithDeps(cfg, &mockBackend{})

	notes := []backend.Note{
		{Title: "Thoughts", Tags: []string{"blog:thoughts"}, Content: "First thought\nSecond thought"},
		{Title: "Camping", Tags: []string{"blog:camping"}, Content: "First trip\nSecond trip"},
	}
	processed := app.processNotes(notes)
	if len(processed) != 4 {
		t.Fatalf("expected 4 processed notes, got %d", len(processed))
	}

	for _, index := range []int{0, 1} {
		if len(processed[index].Tags) != 1 || processed[index].Tags[0] != "thoughts" {
			t.Errorf("thought note %d tags = %v, want [thoughts]", index, processed[index].Tags)
		}
	}
	for _, index := range []int{2, 3} {
		if len(processed[index].Tags) != 1 || processed[index].Tags[0] != "camping" {
			t.Errorf("camping note %d tags = %v, want [camping]", index, processed[index].Tags)
		}
	}
}

func TestMatchingContinuousTag(t *testing.T) {
	tag, replacement, ok := matchingContinuousTag(
		[]string{"blog:camping"},
		[]string{"blog:thoughts", "blog:camping"},
	)
	if !ok {
		t.Fatal("expected a matching continuous-note tag")
	}
	if tag != "blog:camping" {
		t.Errorf("tag = %q, want %q", tag, "blog:camping")
	}
	if replacement != "camping" {
		t.Errorf("replacement = %q, want %q", replacement, "camping")
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
	substitutions := parseSubstitutions("find:replace, a: b, :ignored")
	if len(substitutions) != 2 {
		t.Fatalf("expected 2 substitutions, got %d", len(substitutions))
	}
	if substitutions[0].Find != "find" || substitutions[0].Replace != "replace" {
		t.Errorf("first substitution = %+v, want find->replace", substitutions[0])
	}
	if substitutions[1].Find != "a" || substitutions[1].Replace != "b" {
		t.Errorf("second substitution = %+v, want a->b", substitutions[1])
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

func markdownFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading output directory: %v", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			files = append(files, entry.Name())
		}
	}
	return files
}
