package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
)

func TestCheckCommandFlags(t *testing.T) {
	cmd := newCheckCmd()

	if cmd.Name() != "check" {
		t.Errorf("Expected command name 'check', got %q", cmd.Name())
	}

	if cmd.Flags().Lookup("write") == nil {
		t.Error("Expected --write flag on check command")
	}
	flag := cmd.Flags().Lookup("write")
	if flag.Shorthand != "w" {
		t.Errorf("Expected -w shorthand for --write flag on check command, got %q", flag.Shorthand)
	}
	if cmd.Flags().Lookup("stale-days") == nil {
		t.Error("Expected --stale-days flag on check command")
	}
}

func TestCheckCommandDescriptions(t *testing.T) {
	cmd := newCheckCmd()

	if cmd.Short != "Run all checks: archived, eol, and outdated" {
		t.Errorf("Expected Short %q, got %q", "Run all checks: archived, eol, and outdated", cmd.Short)
	}
	expectedLong := `Run archived, eol, and outdated checks in order.
Use --write/-w to automatically apply updates: runs archived check, then eol with --update, then outdated with --update.`
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long %q, got %q", expectedLong, cmd.Long)
	}
}

func TestCheckCommandFlagDefaults(t *testing.T) {
	cmd := newCheckCmd()

	writeVal, err := cmd.Flags().GetBool("write")
	if err != nil {
		t.Fatalf("Failed to get write flag: %v", err)
	}
	if writeVal != false {
		t.Errorf("Expected write default false, got %v", writeVal)
	}

	staleDaysVal, err := cmd.Flags().GetInt("stale-days")
	if err != nil {
		t.Fatalf("Failed to get stale-days flag: %v", err)
	}
	if staleDaysVal != actioninfo.DefaultStaleDays {
		t.Errorf("Expected stale-days default %d, got %d", actioninfo.DefaultStaleDays, staleDaysVal)
	}
}

func TestCheckCommandNoArgs(t *testing.T) {
	cmd := newCheckCmd()
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

func TestCheckCommandHasRun(t *testing.T) {
	cmd := newCheckCmd()
	if cmd.Run == nil {
		t.Error("Expected Run function to be set on check command")
	}
}
