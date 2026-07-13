package tts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicOutputUsesNormalCreationPermissions(t *testing.T) {
	dir := t.TempDir()
	controlPath := filepath.Join(dir, "control.mp3")
	control, err := os.OpenFile(controlPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := control.Close(); err != nil {
		t.Fatal(err)
	}
	controlInfo, err := os.Stat(controlPath)
	if err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(dir, "output.mp3")
	output, err := newAtomicOutput(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer output.abort()
	if _, err := output.file.WriteString("audio"); err != nil {
		t.Fatal(err)
	}
	if err := output.commit(); err != nil {
		t.Fatal(err)
	}
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := outputInfo.Mode().Perm(), controlInfo.Mode().Perm(); got != want {
		t.Fatalf("output permissions = %04o, want normal creation permissions %04o", got, want)
	}
}

func TestAtomicOutputPreservesPermissionlessExistingTarget(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "output.mp3")
	if err := os.WriteFile(outputPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outputPath, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(outputPath, 0600) }()

	output, err := newAtomicOutput(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer output.abort()
	if _, err := output.file.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	if err := output.commit(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0 {
		t.Fatalf("output permissions = %04o, want preserved 0000", got)
	}
}

func TestAtomicOutputSupportsLongValidDestinationName(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), strings.Repeat("a", 240)+".mp3")
	output, err := newAtomicOutput(outputPath)
	if err != nil {
		t.Fatalf("newAtomicOutput rejected valid long destination name: %v", err)
	}
	output.abort()
}
