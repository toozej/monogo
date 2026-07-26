package readmegen

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/github-readme-gen/internal/readme"
)

type fakeClient struct {
	data *readme.Data
	err  error
	user string
}

func (c *fakeClient) Fetch(_ context.Context, user string) (*readme.Data, error) {
	c.user = user
	return c.data, c.err
}

func TestRunRendersREADME(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "README.md.tpl")
	outputPath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(templatePath, []byte("Hello {{.Username}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeClient{data: &readme.Data{Username: "alice"}}
	var stdout bytes.Buffer

	cmd := newRootCommand(func(_ context.Context, token string) fetcher {
		if token != "secret" {
			t.Errorf("token = %q, want secret", token)
		}
		return client
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--template", templatePath, "--output", outputPath, "--user", "alice", "--token", "secret"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if client.user != "alice" {
		t.Errorf("Fetch user = %q, want alice", client.user)
	}
	if got := stdout.String(); !strings.Contains(got, "Wrote "+outputPath+" from "+templatePath) {
		t.Errorf("stdout = %q", got)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "Hello alice" {
		t.Errorf("README = %q, want Hello alice", got)
	}
}

func TestRunReturnsErrors(t *testing.T) {
	client := &fakeClient{err: errors.New("unavailable")}
	cmd := newRootCommand(func(context.Context, string) fetcher { return client })
	cmd.SetArgs([]string{"--unknown"})
	if err := cmd.Execute(); err == nil {
		t.Error("run accepted an unknown flag")
	}
	cmd = newRootCommand(func(context.Context, string) fetcher { return client })
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "fetch GitHub data") {
		t.Errorf("run error = %v, want fetch error", err)
	}
}

func TestRootCommandUsesGitHubTokenEnvironmentDefault(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "environment-token")
	client := &fakeClient{err: errors.New("unavailable")}
	cmd := newRootCommand(func(_ context.Context, token string) fetcher {
		if token != "environment-token" {
			t.Errorf("token = %q, want environment-token", token)
		}
		return client
	})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Error("root command succeeded despite fetch error")
	}
}

func TestRootCommandIncludesStandardCommandsAndHelp(t *testing.T) {
	cmd := newRootCommand(func(context.Context, string) fetcher { return &fakeClient{} })
	if cmd.Flag("help") == nil {
		t.Fatal("root command is missing the help flag")
	}

	commands := make(map[string]*cobra.Command)
	for _, subcommand := range cmd.Commands() {
		commands[subcommand.Name()] = subcommand
	}
	avatarCommand, ok := commands["avatar"]
	if !ok {
		t.Fatal("root command is missing avatar")
	}
	if !avatarCommand.Hidden {
		t.Error("avatar command should be hidden")
	}
	for _, name := range []string{"man", "version"} {
		if _, ok := commands[name]; !ok {
			t.Errorf("root command is missing %q", name)
		}
	}
}
