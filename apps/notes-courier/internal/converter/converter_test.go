package converter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
	"github.com/toozej/monogo/apps/notes-courier/internal/config"
	"github.com/toozej/monogo/apps/notes-courier/internal/hugo"
	"github.com/toozej/monogo/apps/notes-courier/internal/obsidian"
	"github.com/toozej/monogo/apps/notes-courier/internal/ssg"
	"github.com/toozej/monogo/apps/notes-courier/internal/vite"
)

type mockBackend struct {
	notes     []backend.Note
	err       error
	fetchFunc func(context.Context, string) ([]backend.Note, error)
}

type mockDestination struct {
	notes  []backend.Note
	called int
}

func (m *mockDestination) WriteNotes(_ context.Context, notes []backend.Note) error {
	m.called++
	m.notes = notes
	return nil
}

func TestRunToNotesPlatform(t *testing.T) {
	destination := &mockDestination{}
	app := newAppWithDeps(config.Config{Backend: "simplenote", Destination: "memos"}, &mockBackend{
		notes: []backend.Note{{ID: "key", Title: "One", Content: "body"}},
	})
	app.destination = destination
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if destination.called != 1 || len(destination.notes) != 1 || destination.notes[0].ID != "simplenote:key" {
		t.Fatalf("wrong destination notes: %+v", destination.notes)
	}
}

func TestRunToNotesPlatformStopsOnEmptySource(t *testing.T) {
	destination := &mockDestination{}
	app := newAppWithDeps(config.Config{Destination: "memos"}, &mockBackend{})
	app.destination = destination
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected empty source error")
	}
	if destination.called != 0 {
		t.Fatal("wrote an empty source")
	}
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

func TestNewAppDestinations(t *testing.T) {
	for _, destination := range []string{"hugo", "vite", "memos", "obsidian"} {
		t.Run(destination, func(t *testing.T) {
			app, err := NewApp(config.Config{Backend: "simplenote", Destination: destination, ObsidianVaultPath: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			if (app.destination != nil) != (destination == "memos") {
				t.Fatalf("wrong destination for %q", destination)
			}
		})
	}
	if _, err := NewApp(config.Config{Backend: "simplenote", Destination: "unknown"}); err == nil {
		t.Fatal("accepted unsupported destination")
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

func TestRunMigratesOldManifest(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "stale.md")
	if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyOutputManifestName), []byte(`{"files":["stale.md"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	app := newAppWithDeps(config.Config{OutputDir: dir}, &mockBackend{notes: []backend.Note{{Title: "Current", Content: "body"}}})
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("old generated file remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, outputManifestName)); err != nil {
		t.Fatalf("new manifest missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyOutputManifestName)); !os.IsNotExist(err) {
		t.Fatalf("old manifest remains: %v", err)
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
			name: "obsidian",
			cfg:  config.Config{SSGType: "obsidian"},
			matcher: func(f ssg.Formatter) bool {
				_, ok := f.(*obsidian.Formatter)
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

func TestSimplenoteObsidianMigration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sncli helper requires a POSIX shell")
	}
	for _, mode := range []string{"legacy", "destination"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			fixture := filepath.Join(dir, "snapshot.json")
			t.Setenv("NOTES2SSG_TEST_EXPORT", fixture)
			sncli := filepath.Join(dir, "sncli")
			script := "#!/bin/sh\n[ \"$#\" -eq 2 ] && [ \"$1\" = '--config=/dev/null' ] && [ \"$2\" = export ] || exit 2\ncat \"$NOTES2SSG_TEST_EXPORT\"\n"
			if err := os.WriteFile(sncli, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			vault := t.TempDir()
			output := filepath.Join(vault, "Simplenote")
			cfg := config.Config{
				Backend: "simplenote", SSGType: "obsidian", SNCliPath: sncli, OutputDir: output,
				ContinuousNoteTag: "blog:go", UnlistedTags: "blog:go",
			}
			if mode == "destination" {
				cfg.Destination = "Obsidian"
				cfg.SSGType = "hugo"
				cfg.ObsidianVaultPath = vault
				cfg.ObsidianFolder = "Simplenote"
				cfg.OutputDir = filepath.Join(dir, "unused")
			}
			app, err := NewApp(cfg)
			if err != nil {
				t.Fatal(err)
			}
			for _, body := range []string{"# Original\n\n| a | b |\n|---|---|\n", "# Renamed\n\nUpdated"} {
				// JSON string escapes keep the shell helper independent of note contents.
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				content := `[{"key":"abc123","content":` + string(encoded) + `,"tags":["blog:go"],"creationDate":1693535615,"modificationDate":1693535616}]`
				if err := os.WriteFile(fixture, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := app.Run(context.Background()); err != nil {
					t.Fatal(err)
				}
				files := markdownFiles(t, output)
				if len(files) != 1 || files[0] != "simplenote-abc123.md" {
					t.Fatalf("source update changed identity or split the note: %v", files)
				}
				data, err := os.ReadFile(filepath.Join(output, files[0]))
				if err != nil || !strings.HasSuffix(string(data), body) || !strings.Contains(string(data), `  - "blog/go"`) {
					t.Fatalf("migration changed Markdown or tags: %q, %v", data, err)
				}
			}
			statePath := filepath.Join(output, ".notes2ssg-obsidian.json")
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fixture, []byte("malformed JSON"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background()); err == nil {
				t.Fatal("accepted a malformed source export")
			}
			after, err := os.ReadFile(statePath)
			if err != nil || string(after) != string(before) || len(markdownFiles(t, output)) != 1 {
				t.Fatal("a malformed source export changed files or state")
			}
		})
	}
}

func TestObsidianRequiresOutputPath(t *testing.T) {
	if _, err := NewApp(config.Config{Backend: "usememos", SSGType: "obsidian"}); err == nil {
		t.Fatal("expected missing Obsidian output path error")
	}
}

func TestObsidianPollingRecoversAfterFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	calls := 0
	source := &mockBackend{fetchFunc: func(context.Context, string) ([]backend.Note, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("temporary source failure")
		}
		return []backend.Note{{ID: "abc", Source: "simplenote", Title: "Recovered", Content: "body"}}, nil
	}}
	dir := t.TempDir()
	app := newAppWithDeps(config.Config{Destination: "obsidian", SSGType: "hugo", OutputDir: dir, PollingCycle: 1}, source)
	if err := app.RunPolling(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(markdownFiles(t, dir)) != 1 {
		t.Fatalf("polling did not recover: %d calls", calls)
	}
	app.config.PollingCycle = 0
	source.err = errors.New("source failure")
	source.fetchFunc = nil
	if err := app.RunPolling(context.Background()); err == nil {
		t.Fatal("one-shot mode did not return the source failure")
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

func TestMemosObsidianMigration(t *testing.T) {
	for _, setting := range []string{"destination", "legacy"} {
		t.Run(setting, func(t *testing.T) {
			content := "# Same title\n\nfirst\n"
			empty := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/memos" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if empty {
					_, _ = w.Write([]byte(`{"memos":[]}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"memos": []map[string]any{
					{"name": "memos/one", "content": content, "tags": []string{"blog:go"}, "createTime": "2024-01-02T03:04:05Z"},
					{"name": "memos/two", "content": "# Same title\n\nsecond"},
				}})
			}))
			defer server.Close()
			vault := t.TempDir()
			cfg := config.Config{
				Backend: "memos", Destination: "obsidian", SSGType: "hugo",
				MemosURL: server.URL, MemosToken: "test-token",
				ObsidianVaultPath: vault, ObsidianFolder: "Imports",
				ContinuousNoteTag: "blog:go",
			}
			if setting == "legacy" {
				cfg.Destination = ""
				cfg.SSGType = "obsidian"
			}
			app, err := NewApp(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(vault, "Imports")
			files := markdownFiles(t, dir)
			if len(files) != 2 {
				t.Fatalf("duplicate titles lost notes: %v", files)
			}
			var path string
			for _, name := range files {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), `source_id: "memos/one"`) {
					path = filepath.Join(dir, name)
					if !strings.HasSuffix(string(data), content) || !strings.Contains(string(data), `  - "blog/go"`) || !strings.Contains(string(data), "created: 2024-01-02T03:04:05Z") {
						t.Fatalf("export lost content or metadata: %s", data)
					}
				}
			}
			if path == "" {
				t.Fatal("export lost Memos identity")
			}
			content = "# Renamed\n\nchanged source"
			if err := app.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || !strings.HasSuffix(string(data), content) || len(markdownFiles(t, dir)) != 2 {
				t.Fatalf("rename changed identity: %s, %v", data, err)
			}
			if err := os.WriteFile(path, []byte("local edit"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background()); err == nil {
				t.Fatal("export accepted a local edit")
			}
			empty = true
			if err := app.Run(context.Background()); err != nil || len(markdownFiles(t, dir)) != 2 {
				t.Fatalf("default empty export removed imports: %v", err)
			}
			app.config.ObsidianDeleteMissing = true
			if err := app.Run(context.Background()); err == nil {
				t.Fatal("pruning accepted a local edit")
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(context.Background()); err != nil || len(markdownFiles(t, dir)) != 0 {
				t.Fatalf("explicit pruning did not remove imports: %v", err)
			}
		})
	}
}
