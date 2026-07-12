package output

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicFileAbortPreservesExistingOutput(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output.gpx")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	file, _, err := createAtomic(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("replacement"); err != nil {
		t.Fatal(err)
	}
	file.abort()

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("output = %q, want original", got)
	}
	assertNoTemporaryFiles(t, filepath.Dir(target))
}

func TestAtomicFileCommitReplacesOutputAndPreservesMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output.gpx")
	if err := os.WriteFile(target, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}

	file, commit, err := createAtomic(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.abort()
	if _, err := file.WriteString("replacement"); err != nil {
		t.Fatal(err)
	}
	if err := commit(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replacement" {
		t.Fatalf("output = %q, want replacement", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
			t.Fatalf("output mode = %o, want %o", got, want)
		}
	}
	assertNoTemporaryFiles(t, filepath.Dir(target))
}

func TestAtomicFileCommitCreatesParentAndMatchesCreateMode(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "map.html")
	reference := filepath.Join(filepath.Dir(target), "reference")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	referenceFile, err := os.Create(reference)
	if err != nil {
		t.Fatal(err)
	}
	if err := referenceFile.Close(); err != nil {
		t.Fatal(err)
	}

	file, commit, err := createAtomic(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.abort()
	if _, err := file.WriteString("map"); err != nil {
		t.Fatal(err)
	}
	if err := commit(); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		referenceInfo, err := os.Stat(reference)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), referenceInfo.Mode().Perm(); got != want {
			t.Fatalf("output mode = %o, want %o", got, want)
		}
	}
	assertNoTemporaryFiles(t, filepath.Dir(target))
}

func assertNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".photos2map-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
