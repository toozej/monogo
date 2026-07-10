package cmd

import (
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
