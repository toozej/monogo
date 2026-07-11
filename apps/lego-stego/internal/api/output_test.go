package api_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/toozej/monogo/apps/lego-stego/internal/api"
)

func TestWriteFileAtomicWritesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")
	want := []byte("atomic payload")

	if err := api.WriteFileAtomic(path, want, 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	got, err := os.ReadFile(path) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content mismatch: got %q, want %q", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode mismatch: got %v, want %v", info.Mode().Perm(), os.FileMode(0600))
	}

	// no temporary files should be left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one file, found %d: %v", len(entries), entries)
	}
}

func TestWriteFileAtomicOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if err := os.WriteFile(path, []byte("stale contents that are longer"), 0644); err != nil {
		t.Fatalf("seed existing file failed: %v", err)
	}

	want := []byte("fresh")
	if err := api.WriteFileAtomic(path, want, 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	got, err := os.ReadFile(path) // #nosec G304 -- test-controlled path
	if err != nil {
		t.Fatalf("read output failed: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content not replaced: got %q, want %q", got, want)
	}
}

func TestWriteFileAtomicFailureLeavesTargetUntouched(t *testing.T) {
	dir := t.TempDir()

	// A regular file stands in for the parent directory, so creating the
	// temp file under it fails with ENOTDIR before the target is touched.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("seed blocker failed: %v", err)
	}

	target := filepath.Join(blocker, "nested", "out.bin")
	if err := api.WriteFileAtomic(target, []byte("data"), 0600); err == nil {
		t.Fatal("expected error writing under a non-directory path")
	}

	// The failed write must not have produced or corrupted any real file.
	if _, err := os.ReadFile(target); err == nil { // #nosec G304 -- test-controlled path
		t.Fatal("target should not be readable after failed write")
	}
	if got, err := os.ReadFile(blocker); err != nil || string(got) != "x" { // #nosec G304 -- test-controlled path
		t.Fatalf("pre-existing file was altered by failed write: got %q err %v", got, err)
	}
}
