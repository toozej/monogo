package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func testRunCommand(t *testing.T, dir, output string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("dir", dir, "")
	cmd.Flags().String("output", output, "")
	if err := cmd.Flags().Set("dir", dir); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("output", output); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestRunRejectsUnknownOutput(t *testing.T) {
	err := run(testRunCommand(t, t.TempDir(), "json"), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("run() error = %v, want unsupported-output error", err)
	}
}

func TestRunRejectsMissingInputDirectory(t *testing.T) {
	err := run(testRunCommand(t, t.TempDir()+"/missing", "html"), nil)
	if err == nil || !strings.Contains(err.Error(), "walk image directory") {
		t.Fatalf("run() error = %v, want missing-directory error", err)
	}
}

func TestRunRejectsDirectoryWithoutGPSImages(t *testing.T) {
	err := run(testRunCommand(t, t.TempDir(), "html"), nil)
	if err == nil || !strings.Contains(err.Error(), "no GPS data") {
		t.Fatalf("run() error = %v, want no-GPS-data error", err)
	}
}

func TestRootCmdPreRunPropagatesConfigErrors(t *testing.T) {
	t.Setenv("PHOTOS2MAP_DEBUG", "not-a-boolean")
	if err := rootCmdPreRun(&cobra.Command{}, nil); err == nil || !strings.Contains(err.Error(), "load configuration") {
		t.Fatalf("rootCmdPreRun() error = %v, want configuration error", err)
	}
}

func TestRunPropagatesOutputErrors(t *testing.T) {
	inputDir, err := filepath.Abs(filepath.Join("..", "..", "internal", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	workingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workingDir, "out"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	err = run(testRunCommand(t, inputDir, "html"), nil)
	if err == nil || !strings.Contains(err.Error(), "create map output") {
		t.Fatalf("run() error = %v, want output-creation error", err)
	}
}
