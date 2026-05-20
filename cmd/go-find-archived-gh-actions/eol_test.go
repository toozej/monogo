package cmd

import (
	"testing"
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
