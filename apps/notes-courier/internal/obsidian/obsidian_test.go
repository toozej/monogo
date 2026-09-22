package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	imported := []backend.Note{{ID: "simplenote:key-1", Title: "Imported note", Slug: "imported-note", Content: "body", Tags: []string{"work"}}}
	if err := client.WriteNotes(context.Background(), imported); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "Imported", "imported-note.md")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "#work") || !strings.Contains(string(first), markerPrefix) {
		t.Fatalf("missing metadata: %s", first)
	}
	if err := client.WriteNotes(context.Background(), imported); err != nil {
		t.Fatal(err)
	}
	imported[0].Content = "updated"
	if err := client.WriteNotes(context.Background(), imported); err != nil {
		t.Fatal(err)
	}
	imported[0].Title = "Renamed note"
	imported[0].Slug = "renamed-note"
	if err := client.WriteNotes(context.Background(), imported); err != nil {
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
	path := filepath.Join(vault, "owned.md")
	if err := os.WriteFile(path, []byte("personal"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := client.WriteNotes(context.Background(), []backend.Note{{Title: "Owned", Slug: "owned", Content: "incoming"}}); err == nil {
		t.Fatal("overwrote unmarked file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "personal" {
		t.Fatal("changed unmarked file")
	}
}
