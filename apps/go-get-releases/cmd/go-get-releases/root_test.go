package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/go-get-releases/internal/releases"
)

func TestUtilityCommands(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"version"}} {
		c := newRootCommand()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
		if args[0] == "--help" && !strings.Contains(out.String(), "--keep") {
			t.Fatal(out.String())
		}
	}
}

func TestInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"--keep", "0", "a/b"}, {"--retries", "0", "a/b"}, {"--target", "invalid", "a/b"}, {"--timeout", "0", "a/b"}, {"--directory", "", "a/b"}, {"invalid"}, {"--config", "/does/not/exist"}} {
		c := newRootCommand()
		c.SetArgs(args)
		if err := c.Execute(); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestConfigAndFlagOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("keep: 0\nrepositories:\n  - name: a/b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c := newRootCommand()
	c.SetArgs([]string{"--config", path, "--keep", "3", "--repo", "a/b"})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "duplicate repository") {
		t.Fatal(err)
	}
}

func TestCommandOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "directory: /from/config\nkeep: 5\nretries: 1\nrequire_all: true\nrepositories:\n  - name: first/repo\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		called := false
		c := newCommand(func(ctx context.Context, got releases.Config, _, _ io.Writer) error {
			called = true
			if len(got.Repositories) != 3 || got.Repositories[0].Name != "first/repo" || got.Repositories[1].Name != "second/repo" || got.Repositories[2].Name != "third/repo" {
				t.Fatal(got.Repositories)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("timeout is missing")
			}
			if override {
				if got.Directory != "/from/cli" || got.Keep != 2 || got.Retries != 3 || got.RequireAll || len(got.Targets) != 1 || got.Targets[0].Arch != "arm64" {
					t.Fatal(got)
				}
			} else if got.Directory != "/from/config" || got.Keep != 5 || got.Retries != 1 || !got.RequireAll || len(got.Targets) != 5 {
				t.Fatal(got)
			}
			return nil
		})
		args := []string{"--config", path, "--repo", "second/repo", "third/repo"}
		if override {
			args = append(args, "--directory", "/from/cli", "--keep", "2", "--retries", "3", "--require-all=false", "--target", "linux/aarch64:appimage")
		}
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("download function did not run")
		}
	}
}
