package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandReturnsConfigLoadError(t *testing.T) {
	const helperEnv = "GO_WANT_GHRELEASES2RSS_CONFIG_ERROR_HELPER"
	if os.Getenv(helperEnv) == "1" {
		rootCmd.SetArgs([]string{"--file", "repos.txt"})
		err := rootCmd.Execute()
		if err == nil {
			t.Fatal("expected malformed .env to fail")
		}
		_, _ = fmt.Fprintln(os.Stdout, err)
		return
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MINIFLUX_API_KEY='unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRootCommandReturnsConfigLoadError$")
	cmd.Dir = dir
	cmd.Env = append(environmentWithout("MINIFLUX_API_KEY", "MINIFLUX_URL"), helperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("configuration error exited inside pre-run: %v; output = %q", err, output)
	}
	if !strings.Contains(string(output), "error loading .env file") {
		t.Fatalf("command did not return configuration error: %q", output)
	}
}

func environmentWithout(names ...string) []string {
	excluded := make(map[string]struct{}, len(names))
	for _, name := range names {
		excluded[name] = struct{}{}
	}

	environment := make([]string, 0, len(os.Environ()))
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if _, ok := excluded[name]; !ok {
			environment = append(environment, variable)
		}
	}
	return environment
}
