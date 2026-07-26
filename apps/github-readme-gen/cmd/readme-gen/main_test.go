package readmegen

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	err := run([]string{"-template", templatePath, "-output", outputPath, "-user", "alice", "-token", "secret"}, &stdout, func(_ context.Context, token string) fetcher {
		if token != "secret" {
			t.Errorf("token = %q, want secret", token)
		}
		return client
	})
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
	if err := run([]string{"-unknown"}, &bytes.Buffer{}, func(context.Context, string) fetcher { return client }); err == nil {
		t.Error("run accepted an unknown flag")
	}
	if err := run(nil, &bytes.Buffer{}, func(context.Context, string) fetcher { return client }); err == nil || !strings.Contains(err.Error(), "fetch GitHub data") {
		t.Errorf("run error = %v, want fetch error", err)
	}
}
