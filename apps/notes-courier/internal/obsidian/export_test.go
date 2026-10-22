package obsidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

func TestExportStableIdentityAndNoUnchangedWrites(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter()
	notes := []backend.Note{
		{ID: "abc", Source: "simplenote", Title: "Same", Content: "first"},
		{ID: "def", Source: "simplenote", Title: "Same", Content: "second"},
	}
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "simplenote-abc.md")
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range []string{path, filepath.Join(dir, manifestName)} {
		if err := os.Chtimes(name, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, filepath.Join(dir, manifestName)} {
		info, err := os.Stat(name)
		if err != nil || !info.ModTime().Equal(old) {
			t.Fatalf("unchanged file was rewritten: %s, %v", name, err)
		}
	}
	notes[0].Title, notes[0].Content = "Renamed", "updated"
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, path); !strings.Contains(got, "Renamed") || !strings.HasSuffix(got, "updated") {
		t.Fatalf("source update missing: %s", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("expected two notes and one manifest: %v, %v", entries, err)
	}
}

func TestExportConflictsStopAllWrites(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter()
	notes := []backend.Note{
		{ID: "abc", Source: "simplenote", Title: "First", Content: "source"},
		{ID: "def", Source: "simplenote", Title: "Second", Content: "source"},
	}
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "simplenote-abc.md")
	second := filepath.Join(dir, "simplenote-def.md")
	before := readTestFile(t, first)
	stateBefore := readTestFile(t, filepath.Join(dir, manifestName))
	writeTestFile(t, second, "local edit")
	notes[0].Content = "changed source"
	if err := f.Export(context.Background(), dir, notes, false); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected a local edit conflict, got %v", err)
	}
	if readTestFile(t, first) != before || readTestFile(t, second) != "local edit" || readTestFile(t, filepath.Join(dir, manifestName)) != stateBefore {
		t.Fatal("conflicted export changed files or state")
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatalf("export did not recover after conflict resolution: %v", err)
	}
}

func TestExportRetainsMissingNotesUnlessPruningIsEnabled(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter()
	n := backend.Note{ID: "abc", Source: "simplenote", Title: "Note", Content: "body"}
	if err := f.Export(context.Background(), dir, []backend.Note{n}, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "simplenote-abc.md")
	userPath := filepath.Join(dir, "personal.md")
	writeTestFile(t, userPath, "personal")
	if err := f.Export(context.Background(), dir, nil, false); err != nil {
		t.Fatal(err)
	}
	_ = readTestFile(t, path)
	writeTestFile(t, path, "local edit")
	if err := f.Export(context.Background(), dir, nil, true); err == nil {
		t.Fatal("pruning removed a local edit")
	}
	writeTestFile(t, path, f.Format(n))
	if err := f.Export(context.Background(), dir, nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing source note was not removed: %v", err)
	}
	if readTestFile(t, userPath) != "personal" {
		t.Fatal("pruning changed an unrelated file")
	}
}

func TestExportRejectsUntrackedFilesAndInvalidState(t *testing.T) {
	notes := []backend.Note{{ID: "abc", Source: "simplenote", Content: "body"}}
	for _, tc := range []struct{ name, file, content string }{
		{"untracked file", "simplenote-abc.md", "personal content"},
		{"invalid JSON", manifestName, "{"},
		{"unknown version", manifestName, `{"version":2,"files":{}}`},
		{"missing files", manifestName, `{"version":1,"files":null}`},
		{"absent files", manifestName, `{"version":1}`},
		{"absent version", manifestName, `{"files":{}}`},
		{"path traversal", manifestName, `{"version":1,"files":{"../outside.md":"bad"}}`},
		{"invalid hash", manifestName, `{"version":1,"files":{"simplenote-abc.md":"bad"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.file)
			writeTestFile(t, path, tc.content)
			if err := NewFormatter().Export(context.Background(), dir, notes, true); err == nil {
				t.Fatal("expected an export error")
			}
			if readTestFile(t, path) != tc.content {
				t.Fatal("failed export changed an existing file")
			}
		})
	}
}

func TestExportRejectsUnsafeIdentityAndDuplicateNotes(t *testing.T) {
	for _, notes := range [][]backend.Note{
		{{ID: "../escape", Source: "simplenote"}},
		{{ID: "abc", Source: "other"}},
		{{Source: "simplenote"}},
		{{ID: "ABC", Source: "simplenote"}},
		{{ID: "abc", Source: "simplenote"}, {ID: "abc", Source: "simplenote"}},
	} {
		if err := NewFormatter().Export(context.Background(), t.TempDir(), notes, false); err == nil {
			t.Fatalf("accepted invalid identity: %#v", notes)
		}
	}
}

func TestExportRejectsSymlinksAndConcurrentExport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the helper requires POSIX symlinks")
	}
	for _, name := range []string{"simplenote-abc.md", manifestName, lockName} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			outside := filepath.Join(t.TempDir(), "outside")
			writeTestFile(t, outside, "untouched")
			if err := os.Symlink(outside, filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			if err := NewFormatter().Export(context.Background(), dir, []backend.Note{{ID: "abc", Source: "simplenote"}}, false); err == nil {
				t.Fatal("accepted a symlink")
			}
			if readTestFile(t, outside) != "untouched" {
				t.Fatal("export changed a symlink target")
			}
		})
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, lockName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := NewFormatter().Export(context.Background(), dir, nil, false); err == nil {
		t.Fatal("accepted an existing export lock")
	}
}

func TestExportRecoversAfterStateWriteWasInterrupted(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter()
	n := backend.Note{ID: "abc", Source: "simplenote", Content: "body"}
	writeTestFile(t, filepath.Join(dir, "simplenote-abc.md"), f.Format(n))
	if err := f.Export(context.Background(), dir, []backend.Note{n}, false); err != nil {
		t.Fatal(err)
	}
	var state manifest
	if err := json.Unmarshal([]byte(readTestFile(t, filepath.Join(dir, manifestName))), &state); err != nil || len(state.Files) != 1 {
		t.Fatalf("state recovery failed: %v, %v", state, err)
	}
}

func TestExportCancellationDoesNotWriteNotes(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewFormatter().Export(ctx, dir, []backend.Note{{ID: "abc", Source: "simplenote"}}, false)
	if err == nil {
		t.Fatal("expected cancellation")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("canceled export left files or locks: %v, %v", entries, err)
	}
}

func TestWriteFileChecksOwnership(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "imports")
	f := NewFormatter()
	n := backend.Note{ID: "abc", Source: "simplenote", Content: "body"}
	if err := f.WriteFile(n, dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "simplenote-abc.md")
	writeTestFile(t, path, "local edit")
	if err := f.WriteFile(n, dir); err == nil || readTestFile(t, path) != "local edit" {
		t.Fatal("WriteFile did not preserve a local edit")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestExportOtherSourcesPreservesIdentityAndState(t *testing.T) {
	dir := t.TempDir()
	f := NewFormatter()
	notes := []backend.Note{
		{ID: "memos/one", Source: "memos", Title: "Same", Content: "memo"},
		{ID: "memos/one", Source: "obsidian", Title: "Same", Content: "vault"},
		{ID: "旅行/note.md", Source: "obsidian", Title: "Same", Content: "Unicode"},
	}
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	state, err := readManifest(dir)
	if err != nil || len(state.Files) != len(notes) {
		t.Fatalf("source identities collided: %v, %v", state, err)
	}
	for name := range state.Files {
		data := readTestFile(t, filepath.Join(dir, name))
		if !strings.Contains(data, "source_id:") || strings.Contains(data, "simplenote_id:") {
			t.Fatalf("incorrect source properties: %s", data)
		}
	}
	notes[0].Title, notes[0].Content = "Renamed", "updated"
	if err := f.Export(context.Background(), dir, notes, false); err != nil {
		t.Fatal(err)
	}
	if err := f.Export(context.Background(), dir, nil, true); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != manifestName {
		t.Fatalf("pruning left notes: %v, %v", entries, err)
	}
}
