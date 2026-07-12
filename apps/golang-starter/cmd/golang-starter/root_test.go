package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func useFreshRootCommand(t *testing.T) {
	t.Helper()
	originalCmd := rootCmd
	originalLogLevel := log.GetLevel()
	rootCmd = newRootCommand()
	t.Cleanup(func() {
		rootCmd = originalCmd
		log.SetLevel(originalLogLevel)
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
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".env", []byte("USERNAME='unterminated\n"), 0o600); err != nil {
		t.Fatalf("write invalid .env: %v", err)
	}

	err := rootCmdRun(rootCmd, nil, "")
	if err == nil {
		t.Fatal("rootCmdRun() error = nil")
	}
	if !strings.Contains(err.Error(), "load configuration: error loading .env file") {
		t.Fatalf("rootCmdRun() error = %q", err)
	}
}

func TestVersionDoesNotLoadApplicationConfiguration(t *testing.T) {
	const helperEnv = "GO_WANT_VERSION_WITHOUT_CONFIG_HELPER"
	if os.Getenv(helperEnv) == "1" {
		rootCmd.SetArgs([]string{"version"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("version command error = %v", err)
		}
		if username, ok := os.LookupEnv("USERNAME"); ok {
			t.Fatalf("version command loaded USERNAME=%q from .env", username)
		}
		return
	}

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("USERNAME=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestVersionDoesNotLoadApplicationConfiguration$")
	cmd.Dir = dir
	cmd.Env = append(environmentWithout("USERNAME"), helperEnv+"=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("version helper failed: %v\n%s", err, output)
	}
}

func TestExecuteReportsConfigurationErrorOnceToStderr(t *testing.T) {
	const helperEnv = "GO_WANT_EXECUTE_CONFIG_ERROR_HELPER"
	if os.Getenv(helperEnv) == "1" {
		rootCmd.SetArgs(nil)
		Execute()
		return
	}

	dir := t.TempDir()
	if err := os.WriteFile(dir+"/.env", []byte("USERNAME='unterminated\n"), 0o600); err != nil {
		t.Fatalf("write invalid .env: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteReportsConfigurationErrorOnceToStderr$")
	cmd.Dir = dir
	cmd.Env = append(environmentWithout("USERNAME"), helperEnv+"=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("Execute() error = %v, stderr = %q", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("Execute() stdout = %q, want empty", stdout.String())
	}
	if count := strings.Count(stderr.String(), "load configuration:"); count != 1 {
		t.Errorf("configuration error count = %d, stderr = %q", count, stderr.String())
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("Execute() printed usage for a configuration error: %q", stderr.String())
	}
}

func environmentWithout(name string) []string {
	prefix := name + "="
	environment := os.Environ()
	filtered := environment[:0]
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
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

func TestNewRootCommandsDoNotShareFlagState(t *testing.T) {
	first := newRootCommand()
	second := newRootCommand()

	if err := first.PersistentFlags().Set("debug", "true"); err != nil {
		t.Fatalf("set first debug flag: %v", err)
	}
	if err := first.Flags().Set("username", "first-user"); err != nil {
		t.Fatalf("set first username flag: %v", err)
	}

	debug, err := second.PersistentFlags().GetBool("debug")
	if err != nil {
		t.Fatalf("get second debug flag: %v", err)
	}
	if debug {
		t.Error("second command inherited the first command's debug flag")
	}
	username, err := second.Flags().GetString("username")
	if err != nil {
		t.Fatalf("get second username flag: %v", err)
	}
	if username != "" {
		t.Errorf("second command inherited username %q from the first command", username)
	}
}

func TestRootCmdPreRun_DebugFalse(t *testing.T) {
	origLevel := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	defer func() {
		log.SetLevel(origLevel)
	}()

	rootCmdPreRun(false)
	if got := log.GetLevel(); got != log.InfoLevel {
		t.Errorf("log level = %s, want %s", got, log.InfoLevel)
	}
}

func TestRootCmdPreRun_DebugTrue(t *testing.T) {
	origLevel := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	defer func() {
		log.SetLevel(origLevel)
	}()

	rootCmdPreRun(true)
	if got := log.GetLevel(); got != log.DebugLevel {
		t.Errorf("log level = %s, want %s", got, log.DebugLevel)
	}
}

func TestRootCmdRun(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("USERNAME", "testuser")

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	if err := rootCmdRun(rootCmd, []string{}, ""); err != nil {
		t.Fatalf("rootCmdRun() error = %v", err)
	}

	_ = w.Close()
	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(out.String(), "Hello from testuser") {
		t.Errorf("expected output to contain 'Hello from testuser', got %q", out.String())
	}
}

func TestRootCmdRun_EmptyUsername(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("USERNAME", "")

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	if err := rootCmdRun(rootCmd, []string{}, ""); err != nil {
		t.Fatalf("rootCmdRun() error = %v", err)
	}

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
	t.Chdir(t.TempDir())
	t.Setenv("USERNAME", "executetest")

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

	rootCmd.SetArgs([]string{"-d", "version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with debug flag, got: %v", err)
	}
	rootCmd.SetArgs([]string{})
}

func TestUsernameFlagParsing(t *testing.T) {
	useFreshRootCommand(t)
	t.Chdir(t.TempDir())
	t.Setenv("USERNAME", "environment-user")

	old := os.Stdout
	defer func() { os.Stdout = old }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"-u", "flaguser"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("root command error = %v", err)
	}

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
