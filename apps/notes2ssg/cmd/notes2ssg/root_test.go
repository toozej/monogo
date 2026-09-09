package cmd

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/notes2ssg/internal/config"
)

func TestRootCmdStructure(t *testing.T) {
	if rootCmd.Use != "notes2ssg" {
		t.Errorf("expected Use='notes2ssg', got '%s'", rootCmd.Use)
	}
	if rootCmd.Short != "Convert notes to Hugo-formatted Markdown" {
		t.Errorf("expected Short='Convert notes to Hugo-formatted Markdown', got '%s'", rootCmd.Short)
	}
	if rootCmd.PersistentPreRun == nil {
		t.Error("expected PersistentPreRun to be set, got nil")
	}
	if rootCmd.RunE == nil {
		t.Error("expected RunE to be set, got nil")
	}
}

func TestRootCmdExactArgs(t *testing.T) {
	if err := rootCmd.Args(rootCmd, []string{}); err != nil {
		t.Errorf("expected no error with zero args, got: %v", err)
	}
	if err := rootCmd.Args(rootCmd, []string{"extra"}); err == nil {
		t.Error("expected error when args provided, got nil")
	}
}

func TestRootCmdHasSubcommands(t *testing.T) {
	subcommandNames := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		subcommandNames[cmd.Name()] = true
	}

	for _, name := range []string{"man", "version", "avatar"} {
		if !subcommandNames[name] {
			t.Errorf("expected subcommand '%s' to be registered", name)
		}
	}
}

func TestRootCmdPersistentFlags(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("debug")
	if flag == nil {
		t.Fatal("expected persistent flag 'debug' to be registered")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected debug flag default 'false', got '%s'", flag.DefValue)
	}
	if flag.Shorthand != "d" {
		t.Errorf("expected debug flag shorthand 'd', got '%s'", flag.Shorthand)
	}
}

func TestRootCmdPreRun_DebugFalse(t *testing.T) {
	origDebug := debug
	debug = false
	defer func() { debug = origDebug }()

	rootCmdPreRun(rootCmd, []string{})
}

func TestRootCmdPreRun_DebugTrue(t *testing.T) {
	origDebug := debug
	debug = true
	defer func() { debug = origDebug }()

	rootCmdPreRun(rootCmd, []string{})
}

func TestRootCmdRun(t *testing.T) {
	origRun := runConverter
	defer func() { runConverter = origRun }()

	runCalled := false
	runConverter = func(cfg config.Config) error {
		runCalled = true
		return nil
	}

	if err := rootCmdRun(rootCmd, []string{}); err != nil {
		t.Fatalf("root command returned an error: %v", err)
	}
	if !runCalled {
		t.Error("expected runConverter to be called")
	}
}

func TestRootCmdRunReturnsConverterError(t *testing.T) {
	origRun := runConverter
	defer func() { runConverter = origRun }()

	want := errors.New("conversion failed")
	runConverter = func(cfg config.Config) error { return want }

	if err := rootCmdRun(rootCmd, []string{}); !errors.Is(err, want) {
		t.Errorf("root command error = %v, want %v", err, want)
	}
}

func TestExecute(t *testing.T) {
	origRun := runConverter
	defer func() { runConverter = origRun }()

	runConverter = func(cfg config.Config) error {
		return nil
	}

	old := os.Stdout
	defer func() { os.Stdout = old }()
	r, w, _ := os.Pipe()
	os.Stdout = w

	Execute()

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
}

func TestVersionSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version subcommand execution failed: %v", err)
	}

	if stdout.String() == "" {
		t.Error("expected version subcommand to produce output")
	}
}

func TestManSubcommand(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"man"})
	err := rootCmd.Execute()

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("man subcommand execution failed: %v", err)
	}
}

func TestRootCmdRejectsArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"invalid-arg"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when invalid args provided")
	}
	rootCmd.SetArgs([]string{})
}

func TestDebugFlagParsing(t *testing.T) {
	origDebug := debug
	defer func() { debug = origDebug }()

	rootCmd.SetArgs([]string{"-d", "version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with debug flag, got: %v", err)
	}
	rootCmd.SetArgs([]string{})
}

func TestRootCmdIsCobraCommand(t *testing.T) {
	var _ = (*cobra.Command)(rootCmd)
}
