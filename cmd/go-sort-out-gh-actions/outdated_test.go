package cmd

import (
	"testing"
)

func TestOutdatedCommandFlags(t *testing.T) {
	cmd := newOutdatedCmd()

	if cmd.Name() != "outdated" {
		t.Errorf("Expected command name 'outdated', got %q", cmd.Name())
	}

	if cmd.Flags().Lookup("update") == nil {
		t.Error("Expected --update flag on outdated command")
	}
	if cmd.Flags().Lookup("pin") == nil {
		t.Error("Expected --pin flag on outdated command")
	}
	if cmd.Flags().Lookup("semver") == nil {
		t.Error("Expected --semver flag on outdated command")
	}
}
