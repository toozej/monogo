package cmd

import (
	"testing"
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
