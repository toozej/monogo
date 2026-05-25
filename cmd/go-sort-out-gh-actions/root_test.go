package cmd

import (
	"testing"
)

func TestCommandStructure(t *testing.T) {
	cmd := rootCmd

	subCommands := cmd.Commands()
	commandNames := make(map[string]bool)
	for _, subCmd := range subCommands {
		commandNames[subCmd.Name()] = true
	}

	expectedCommands := []string{"archived", "eol", "outdated", "check", "version", "man"}
	for _, name := range expectedCommands {
		if !commandNames[name] {
			t.Errorf("Expected subcommand %q not found", name)
		}
	}
}

func TestGlobalFlags(t *testing.T) {
	expectedPersistentFlags := []string{"debug", "verbose", "token", "notify", "create-issue", "workflow", "workflows-dir", "repos-dir", "output-format"}
	for _, flagName := range expectedPersistentFlags {
		if rootCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("Expected persistent flag --%s on root command", flagName)
		}
	}
}
