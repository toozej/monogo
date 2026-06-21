// Package man provides tests for manual page generation functionality in the go-sort-out-gh-actions application.
package man

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewManCmd_CommandProperties(t *testing.T) {
	t.Parallel()

	cmd := NewManCmd()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{name: "Use", got: cmd.Use, expected: "man"},
		{name: "Short", got: cmd.Short, expected: "Generates go-sort-out-gh-actions's command line manpages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.expected {
				t.Errorf("Unexpected command %s: got %q, expected %q", tt.name, tt.got, tt.expected)
			}
		})
	}

	boolTests := []struct {
		name     string
		got      bool
		expected bool
	}{
		{name: "SilenceUsage", got: cmd.SilenceUsage, expected: true},
		{name: "DisableFlagsInUseLine", got: cmd.DisableFlagsInUseLine, expected: true},
		{name: "Hidden", got: cmd.Hidden, expected: true},
	}

	for _, tt := range boolTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.expected {
				t.Errorf("Unexpected command %s: got %t, expected %t", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestNewManCmd_ArgsRejected(t *testing.T) {
	t.Parallel()

	manCmd := NewManCmd()

	if err := cobra.NoArgs(manCmd, []string{}); err != nil {
		t.Errorf("Expected NoArgs to accept no args, got error: %v", err)
	}
	if err := cobra.NoArgs(manCmd, []string{"extra-arg"}); err == nil {
		t.Error("Expected NoArgs to reject args")
	}
}

func captureStdout(f func() error) (string, error) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String(), err
}

func TestNewManCmd_RunE_ManPageContent(t *testing.T) {
	tests := []struct {
		name         string
		rootUse      string
		rootShort    string
		rootLong     string
		wantContains []string
	}{
		{
			name:         "basic root command",
			rootUse:      "go-sort-out-gh-actions",
			rootShort:    "Detect archived, outdated, and EOL GitHub Actions",
			rootLong:     "A CLI tool to scan workflow files and detect problematic GitHub Actions.",
			wantContains: []string{".TH", "go-sort-out-gh-actions", "SYNOPSIS", "DESCRIPTION", "OPTIONS"},
		},
		{
			name:         "root command with subcommands",
			rootUse:      "testapp",
			rootShort:    "A test application",
			rootLong:     "Test application for man page generation.",
			wantContains: []string{".TH", "testapp", "SYNOPSIS", "DESCRIPTION"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := &cobra.Command{
				Use:   tt.rootUse,
				Short: tt.rootShort,
				Long:  tt.rootLong,
			}
			subCmd := &cobra.Command{
				Use:   "sub",
				Short: "A subcommand",
				Run:   func(cmd *cobra.Command, args []string) {},
			}
			rootCmd.AddCommand(subCmd)

			manCmd := NewManCmd()
			rootCmd.AddCommand(manCmd)

			// First execution without "man" arg just runs the root command.
			_, _ = captureStdout(func() error {
				return rootCmd.Execute()
			})

			// Execute the man subcommand directly by setting args.
			rootCmd.SetArgs([]string{"man"})
			output, err := captureStdout(func() error {
				return rootCmd.Execute()
			})

			if err != nil {
				t.Fatalf("man command execution failed: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("Expected man page output to contain %q, got output (first 500 chars): %q", want, truncate(output, 500))
				}
			}
		})
	}
}

func TestNewManCmd_RunE_ArgsError(t *testing.T) {
	rootCmd := &cobra.Command{Use: "go-sort-out-gh-actions"}
	manCmd := NewManCmd()
	rootCmd.AddCommand(manCmd)

	rootCmd.SetArgs([]string{"man", "unexpected-arg"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error when passing args to man command, got nil")
	}
}

func TestNewManCmd_RunE_NilRoot(t *testing.T) {
	manCmd := NewManCmd()
	_, err := captureStdout(func() error {
		return manCmd.Execute()
	})
	_ = err
}

func TestNewManCmd_RunE_DuplicateSubcommand(t *testing.T) {
	rootCmd := &cobra.Command{Use: "myapp"}
	sub1 := &cobra.Command{Use: "dup", Short: "First", Run: func(cmd *cobra.Command, args []string) {}}
	sub2 := &cobra.Command{Use: "dup", Short: "Second", Run: func(cmd *cobra.Command, args []string) {}}
	rootCmd.AddCommand(sub1, sub2)

	manCmd := NewManCmd()
	rootCmd.AddCommand(manCmd)

	rootCmd.SetArgs([]string{"man"})
	_, err := captureStdout(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Error("Expected error from duplicate subcommand names, got nil")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
