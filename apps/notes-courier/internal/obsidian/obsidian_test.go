package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/notes-courier/internal/backend"
)

func TestVaultReadWrite(t *testing.T) {
	vault := t.TempDir()
	if err := os.Mkdir(filepath.Join(vault, ".obsidian"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, ".obsidian", "settings.md"), []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	content := "---\ntags:\n  - work\n---\n# Source title\nText #daily"
	if err := os.WriteFile(filepath.Join(vault, "source.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(vault, "Imported")
	if err != nil {
		t.Fatal(err)
	}
	notes, err := client.FetchNotes(context.Background(), "work")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != "source.md" || notes[0].Title != "Source title" || len(notes[0].Tags) != 2 {
		t.Fatalf("wrong notes: %+v", notes)
	}
	imported := []backend.Note{{ID: "key-1", Source: "simplenote", Title: "Imported note", Slug: "imported-note", Content: "body", Tags: []string{"work"}}}
	output, err := client.ExportDirectory()
	if err != nil {
		t.Fatal(err)
	}
	formatter := NewFormatter()
	if err := formatter.Export(context.Background(), output, imported, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "Imported", "simplenote-key-1.md")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), `  - "work"`) || !strings.Contains(string(first), `simplenote_id: "key-1"`) {
		t.Fatalf("missing metadata: %s", first)
	}
	if err := formatter.Export(context.Background(), output, imported, false); err != nil {
		t.Fatal(err)
	}
	imported[0].Content = "updated"
	if err := formatter.Export(context.Background(), output, imported, false); err != nil {
		t.Fatal(err)
	}
	imported[0].Title = "Renamed note"
	imported[0].Slug = "renamed-note"
	if err := formatter.Export(context.Background(), output, imported, false); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(updated), "updated") || !strings.Contains(string(updated), "Renamed note") {
		t.Fatalf("note was not updated: %s, %v", updated, err)
	}
	if _, err := os.Stat(filepath.Join(vault, "Imported", "renamed-note.md")); !os.IsNotExist(err) {
		t.Fatal("renamed note created a duplicate file")
	}
}

func TestVaultRejectsUnsafePathsAndUnownedFiles(t *testing.T) {
	vault := t.TempDir()
	if _, err := NewClient(vault, "../other"); err == nil {
		t.Fatal("accepted parent path")
	}
	client, err := NewClient(vault, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "simplenote-owned.md")
	if err := os.WriteFile(path, []byte("personal"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := client.ExportDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if err := NewFormatter().Export(context.Background(), output, []backend.Note{{ID: "owned", Source: "simplenote", Title: "Owned", Content: "incoming"}}, false); err == nil {
		t.Fatal("overwrote untracked file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "personal" {
		t.Fatal("changed untracked file")
	}
}

func TestVaultReadsSharedExportProperties(t *testing.T) {
	vault := t.TempDir()
	original := backend.Note{
		ID: "abc", Source: "simplenote", Title: "Quoted: \"旅行\"",
		Date:     time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		Modified: time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC),
		Content:  "Title without heading\n\n  code\n", Tags: []string{"two words", "blog:go"},
	}
	if err := NewFormatter().Export(context.Background(), vault, []backend.Note{original}, false); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(vault, "")
	if err != nil {
		t.Fatal(err)
	}
	notes, err := client.FetchNotes(context.Background(), "blog/go")
	if err != nil || len(notes) != 1 {
		t.Fatalf("cannot read shared export: %v, %v", notes, err)
	}
	n := notes[0]
	if n.ID != "simplenote-abc.md" || n.Source != "obsidian" || n.Title != original.Title || n.Content != original.Content || !n.Date.Equal(original.Date) || !n.Modified.Equal(original.Modified) {
		t.Fatalf("vault reader lost properties or Markdown: %+v", n)
	}
}

func TestVaultRejectsInvalidProperties(t *testing.T) {
	for _, raw := range []string{"---\ntags: [\n---\nbody", "---\ntags: {work: true}\n---\nbody"} {
		vault := t.TempDir()
		writeTestFile(t, filepath.Join(vault, "invalid.md"), raw)
		client, err := NewClient(vault, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.FetchNotes(context.Background(), ""); err == nil {
			t.Fatal("accepted invalid properties")
		}
	}
}

func TestVaultRejectsSymlinkImportFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the helper requires POSIX symlinks")
	}
	vault := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(vault, "Imports")); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(vault, "Imports/Notes")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ExportDirectory(); err == nil {
		t.Fatal("accepted a symlink import folder")
	}
}
