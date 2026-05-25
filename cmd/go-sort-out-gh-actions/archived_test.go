package cmd

import (
	"testing"
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
