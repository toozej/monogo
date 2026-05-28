package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
)

func TestArchivedCommandFlags(t *testing.T) {
	cmd := newArchivedCmd()

	if cmd.Name() != "archived" {
		t.Errorf("Expected command name 'archived', got %q", cmd.Name())
	}

	if cmd.Flags().Lookup("stale-days") == nil {
		t.Error("Expected --stale-days flag on archived command")
	}
}

func TestArchivedCommandDescriptions(t *testing.T) {
	cmd := newArchivedCmd()

	if cmd.Short != "Display archived GitHub Actions" {
		t.Errorf("Expected Short %q, got %q", "Display archived GitHub Actions", cmd.Short)
	}
	expectedLong := `Scan workflow files and display GitHub Actions that have been archived upstream. Also checks for stale/deprecated actions.`
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long %q, got %q", expectedLong, cmd.Long)
	}
}

func TestArchivedCommandStaleDaysDefault(t *testing.T) {
	cmd := newArchivedCmd()
	flag := cmd.Flags().Lookup("stale-days")
	if flag == nil {
		t.Fatal("Expected --stale-days flag")
	}
	if flag.DefValue != "365" {
		t.Errorf("Expected default 365, got %q", flag.DefValue)
	}
	val, err := cmd.Flags().GetInt("stale-days")
	if err != nil {
		t.Fatalf("Failed to get stale-days: %v", err)
	}
	if val != actioninfo.DefaultStaleDays {
		t.Errorf("Expected %d, got %d", actioninfo.DefaultStaleDays, val)
	}
}

func TestArchivedCommandNoArgs(t *testing.T) {
	cmd := newArchivedCmd()
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

func TestArchivedCommandHasRun(t *testing.T) {
	cmd := newArchivedCmd()
	if cmd.Run == nil {
		t.Error("Expected Run function to be set on archived command")
	}
}
