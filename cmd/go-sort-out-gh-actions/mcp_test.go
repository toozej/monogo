package cmd

import (
	"testing"
)

func TestNewMCPCmd_ReturnsCommand(t *testing.T) {
	cmd := newMCPCmd()

	if cmd == nil {
		t.Fatal("Expected non-nil command, got nil")
	}

	if cmd.Use != "mcp" {
		t.Errorf("Expected Use %q, got %q", "mcp", cmd.Use)
	}
}

func TestNewMCPCmd_HasSubcommands(t *testing.T) {
	cmd := newMCPCmd()

	subs := cmd.Commands()
	if len(subs) == 0 {
		t.Error("Expected mcp command to have at least one subcommand, got none")
	}
}

func TestNewMCPCmd_IsRegistered(t *testing.T) {
	cmd := newMCPCmd()

	found := false
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == cmd.Name() {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected %q to be registered as a subcommand of root", cmd.Name())
	}
}
