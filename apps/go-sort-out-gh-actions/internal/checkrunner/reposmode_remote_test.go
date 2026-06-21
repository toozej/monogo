package checkrunner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func TestRunRemoteRepoMode(t *testing.T) {
	fileContent := "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(fileContent))

	t.Run("calls processFunc with action refs from remote repo", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/contents/.github/workflows") && !strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				resp := `[{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"}]`
				if _, err := w.Write([]byte(resp)); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			if strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				if _, err := fmt.Fprintf(w, `{"content": "%s", "encoding": "base64"}`, encoded); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			w.WriteHeader(404)
		}))
		defer server.Close()

		ghClient := github.NewClientWithHTTP(server.URL, server.Client())
		parser := workflow.NewParser()
		var buf bytes.Buffer

		writer := output.NewWriter(output.FormatText)
		writer.Output = &buf

		rc := &RunContext{
			Ctx:          context.Background(),
			WorkDir:      "/tmp",
			Parser:       parser,
			GHClient:     ghClient,
			OutputWriter: writer,
		}

		processFuncCalled := false
		var receivedOwnerRepo string

		processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
			processFuncCalled = true
			receivedOwnerRepo = workDir
			return false
		}

		result := RunRemoteRepoMode(rc, "owner/repo", "main", processFunc)

		if !processFuncCalled {
			t.Error("Expected processFunc to be called")
		}
		if receivedOwnerRepo != "owner/repo" {
			t.Errorf("Expected workDir 'owner/repo', got %s", receivedOwnerRepo)
		}
		if result {
			t.Error("Expected RunRemoteRepoMode to return false when processFunc returns false")
		}
	})

	t.Run("returns true when processFunc returns true", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/contents/.github/workflows") && !strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				resp := `[{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"}]`
				if _, err := w.Write([]byte(resp)); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			if strings.Contains(r.URL.Path, "/ci.yml") {
				w.WriteHeader(200)
				if _, err := fmt.Fprintf(w, `{"content": "%s", "encoding": "base64"}`, encoded); err != nil {
					t.Errorf("failed to write response body: %v", err)
				}
				return
			}
			w.WriteHeader(404)
		}))
		defer server.Close()

		ghClient := github.NewClientWithHTTP(server.URL, server.Client())
		parser := workflow.NewParser()
		var buf bytes.Buffer

		writer := output.NewWriter(output.FormatText)
		writer.Output = &buf

		rc := &RunContext{
			Ctx:          context.Background(),
			WorkDir:      "/tmp",
			Parser:       parser,
			GHClient:     ghClient,
			OutputWriter: writer,
		}

		processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
			return true
		}

		result := RunRemoteRepoMode(rc, "owner/repo", "main", processFunc)

		if !result {
			t.Error("Expected RunRemoteRepoMode to return true when processFunc returns true")
		}
	})

	t.Run("returns false when no workflows found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer server.Close()

		ghClient := github.NewClientWithHTTP(server.URL, server.Client())
		parser := workflow.NewParser()
		var buf bytes.Buffer

		writer := output.NewWriter(output.FormatText)
		writer.Output = &buf

		rc := &RunContext{
			Ctx:          context.Background(),
			WorkDir:      "/tmp",
			Parser:       parser,
			GHClient:     ghClient,
			OutputWriter: writer,
		}

		processFuncCalled := false
		processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
			processFuncCalled = true
			return false
		}

		result := RunRemoteRepoMode(rc, "owner/repo", "main", processFunc)

		if processFuncCalled {
			t.Error("Expected processFunc not to be called when no workflows found")
		}
		if result {
			t.Error("Expected RunRemoteRepoMode to return false when no workflows found")
		}
	})

	t.Run("returns false when API error occurs", func(t *testing.T) {
		ghClient := github.NewClientWithHTTP("http://127.0.0.1:0", &http.Client{})
		parser := workflow.NewParser()
		var buf bytes.Buffer

		writer := output.NewWriter(output.FormatText)
		writer.Output = &buf

		rc := &RunContext{
			Ctx:          context.Background(),
			WorkDir:      "/tmp",
			Parser:       parser,
			GHClient:     ghClient,
			OutputWriter: writer,
		}

		processFuncCalled := false
		processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
			processFuncCalled = true
			return false
		}

		result := RunRemoteRepoMode(rc, "owner/repo", "main", processFunc)

		if processFuncCalled {
			t.Error("Expected processFunc not to be called on API error")
		}
		if result {
			t.Error("Expected RunRemoteRepoMode to return false on API error")
		}
	})
}
