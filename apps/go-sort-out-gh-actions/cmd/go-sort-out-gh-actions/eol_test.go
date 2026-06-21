package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
)

func TestEOLCommandFlags(t *testing.T) {
	cmd := newEOLCmd()

	if cmd.Name() != "eol" {
		t.Errorf("Expected command name 'eol', got %q", cmd.Name())
	}

	if cmd.Flags().Lookup("update") == nil {
		t.Error("Expected --update flag on eol command")
	}
	if cmd.Flags().Lookup("stale-days") == nil {
		t.Error("Expected --stale-days flag on eol command")
	}
}

func TestEOLCommandDescriptions(t *testing.T) {
	cmd := newEOLCmd()

	if cmd.Short != "Display GitHub Actions using end-of-life languages and runtimes" {
		t.Errorf("Expected Short %q, got %q", "Display GitHub Actions using end-of-life languages and runtimes", cmd.Short)
	}
	expectedLong := `Scan workflow files and display GitHub Actions that rely on or use end-of-life languages and runtimes. With --update, writes updated versions to affected workflow files.`
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long %q, got %q", expectedLong, cmd.Long)
	}
}

func TestEOLCommandFlagDefaults(t *testing.T) {
	cmd := newEOLCmd()

	updateVal, err := cmd.Flags().GetBool("update")
	if err != nil {
		t.Fatalf("Failed to get update flag: %v", err)
	}
	if updateVal != false {
		t.Errorf("Expected update default false, got %v", updateVal)
	}

	staleDaysVal, err := cmd.Flags().GetInt("stale-days")
	if err != nil {
		t.Fatalf("Failed to get stale-days flag: %v", err)
	}
	if staleDaysVal != actioninfo.DefaultStaleDays {
		t.Errorf("Expected stale-days default %d, got %d", actioninfo.DefaultStaleDays, staleDaysVal)
	}
}

func TestEOLCommandNoArgs(t *testing.T) {
	cmd := newEOLCmd()
	if cmd.Args == nil {
		t.Error("Expected Args to be set")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("Expected NoArgs to accept no args, got error: %v", err)
	}
	if err := cobra.NoArgs(cmd, []string{"extra"}); err == nil {
		t.Error("Expected NoArgs to reject args")
	}
}
