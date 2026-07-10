package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func useFreshRootCommand(t *testing.T) {
	t.Helper()
	originalCmd := rootCmd
	originalConf := conf
	originalDebug := debug
	originalConfigErr := configLoadErr
	rootCmd = newRootCommand()
	t.Cleanup(func() {
		rootCmd = originalCmd
		conf = originalConf
		debug = originalDebug
		configLoadErr = originalConfigErr
	})
}

func TestRootCmdStructure(t *testing.T) {
	if rootCmd.Use != "golang-starter" {
		t.Errorf("expected Use='golang-starter', got '%s'", rootCmd.Use)
	}
	if rootCmd.Short != "golang-starter starter template" {
		t.Errorf("expected Short='golang-starter starter template', got '%s'", rootCmd.Short)
	}
	if rootCmd.Long != "Golang starter template using cobra, logrus, dotenv and env modules" {
		t.Errorf("expected Long='Golang starter template using cobra, logrus, dotenv and env modules', got '%s'", rootCmd.Long)
	}
	if rootCmd.PersistentPreRun == nil {
		t.Error("expected PersistentPreRun to be set, got nil")
	}
	if rootCmd.RunE == nil {
		t.Error("expected RunE to be set, got nil")
	}
}

func TestRootCmdRunReturnsConfigurationError(t *testing.T) {
	originalErr := configLoadErr
	configLoadErr = errors.New("invalid dotenv")
	t.Cleanup(func() { configLoadErr = originalErr })

	if err := rootCmdRun(rootCmd, nil); err == nil {
		t.Fatal("rootCmdRun() error = nil")
	}
}

func TestVersionDoesNotRequireApplicationConfiguration(t *testing.T) {
	useFreshRootCommand(t)
	originalErr := configLoadErr
	configLoadErr = errors.New("invalid dotenv")
	t.Cleanup(func() {
		configLoadErr = originalErr
		rootCmd.SetArgs(nil)
	})

	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command error = %v", err)
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

	for _, name := range []string{"man", "version"} {
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

func TestRootCmdLocalFlags(t *testing.T) {
	flag := rootCmd.Flags().Lookup("username")
	if flag == nil {
		t.Fatal("expected local flag 'username' to be registered")
	}
	if flag.Shorthand != "u" {
		t.Errorf("expected username flag shorthand 'u', got '%s'", flag.Shorthand)
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
	origConf := conf
	defer func() { conf = origConf }()

	conf.Username = "testuser"

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmdRun(rootCmd, []string{})

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(out.String(), "Hello from testuser") {
		t.Errorf("expected output to contain 'Hello from testuser', got %q", out.String())
	}
}

func TestRootCmdRun_EmptyUsername(t *testing.T) {
	origConf := conf
	defer func() { conf = origConf }()

	conf.Username = ""

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmdRun(rootCmd, []string{})

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(out.String(), "Hello from") {
		t.Errorf("expected output to contain 'Hello from', got %q", out.String())
	}
}

func TestExecute(t *testing.T) {
	useFreshRootCommand(t)
	origConf := conf
	defer func() { conf = origConf }()

	conf.Username = "executetest"

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	Execute()

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(out.String(), "Hello from executetest") {
		t.Errorf("expected output to contain 'Hello from executetest', got %q", out.String())
	}
}

func TestVersionSubcommand(t *testing.T) {
	useFreshRootCommand(t)
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
	useFreshRootCommand(t)
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
	useFreshRootCommand(t)
	rootCmd.SetArgs([]string{"invalid-arg"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when invalid args provided")
	}
	rootCmd.SetArgs([]string{})
}

func TestDebugFlagParsing(t *testing.T) {
	useFreshRootCommand(t)
	origDebug := debug
	defer func() { debug = origDebug }()

	rootCmd.SetArgs([]string{"-d", "version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with debug flag, got: %v", err)
	}
	rootCmd.SetArgs([]string{})
}

func TestUsernameFlagParsing(t *testing.T) {
	useFreshRootCommand(t)
	origConf := conf
	defer func() { conf = origConf }()

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"-u", "flaguser"})
	_ = rootCmd.Execute()

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(out.String(), "Hello from flaguser") {
		t.Errorf("expected output to contain 'Hello from flaguser', got %q", out.String())
	}
	rootCmd.SetArgs([]string{})
}

func TestRootCmdIsCobraCommand(t *testing.T) {
	var _ = (*cobra.Command)(rootCmd)
}
