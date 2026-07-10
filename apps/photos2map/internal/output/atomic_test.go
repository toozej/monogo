package output

import (
	"os"
	"path/filepath"
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
}
