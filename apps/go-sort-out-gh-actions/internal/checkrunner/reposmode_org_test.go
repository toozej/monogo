package checkrunner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func TestRunOrgMode(t *testing.T) {
	t.Parallel()

	workflowYAML := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	encoded := base64.StdEncoding.EncodeToString([]byte(workflowYAML))

	orgRepos := []github.RepoEntry{
		{FullName: "org/active-repo", Name: "active-repo", Archived: false, Fork: false},
		{FullName: "org/archived-repo", Name: "archived-repo", Archived: true, Fork: false},
		{FullName: "org/forked-repo", Name: "forked-repo", Archived: false, Fork: true},
	}

	var processCalled bool
	var calledRepo string
	var testMu sync.Mutex
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		testMu.Lock()
		processCalled = true
		calledRepo = workDir
		testMu.Unlock()
		return true
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/orgs/testorg/repos"):
			w.WriteHeader(200)
			body, _ := json.Marshal(orgRepos)
			_, _ = w.Write(body)
		case isWorkflowsDirRequest(path):
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.ContentEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			})
			_, _ = w.Write(body)
		case isWorkflowFileRequest(path):
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestCheckRunnerClient(server)
	rc := &RunContext{
		Ctx:          context.Background(),
		Parser:       workflow.NewParser(),
		GHClient:     client,
		OutputWriter: output.NewWriterWithOptionalCSV(output.FormatText, nil),
	}

	result, err := RunOrgMode(rc, "testorg", false, processFunc)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected RunOrgMode to return true when processFunc returns true")
	}
	testMu.Lock()
	wasCalled := processCalled
	repo := calledRepo
	testMu.Unlock()
	if !wasCalled {
		t.Error("Expected processFunc to be called")
	}
	if repo != "org/active-repo" {
		t.Errorf("Expected calledRepo to be org/active-repo, got %s", repo)
	}
}

func TestRunOrgMode_SkipsArchivedAndForks(t *testing.T) {
	t.Parallel()

	workflowYAML := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	encoded := base64.StdEncoding.EncodeToString([]byte(workflowYAML))

	orgRepos := []github.RepoEntry{
		{FullName: "org/active-repo", Name: "active-repo", Archived: false, Fork: false},
		{FullName: "org/archived-repo", Name: "archived-repo", Archived: true, Fork: false},
		{FullName: "org/forked-repo", Name: "forked-repo", Archived: false, Fork: true},
	}

	var scannedRepos []string
	var testMu sync.Mutex
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		testMu.Lock()
		scannedRepos = append(scannedRepos, workDir)
		testMu.Unlock()
		return false
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/orgs/testorg/repos"):
			w.WriteHeader(200)
			body, _ := json.Marshal(orgRepos)
			_, _ = w.Write(body)
		case isWorkflowsDirRequest(path):
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.ContentEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			})
			_, _ = w.Write(body)
		case isWorkflowFileRequest(path):
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestCheckRunnerClient(server)
	rc := &RunContext{
		Ctx:          context.Background(),
		Parser:       workflow.NewParser(),
		GHClient:     client,
		OutputWriter: output.NewWriterWithOptionalCSV(output.FormatText, nil),
	}

	_, _ = RunOrgMode(rc, "testorg", false, processFunc)

	for _, repo := range scannedRepos {
		if repo == "org/archived-repo" {
			t.Error("Archived repo should not be scanned")
		}
		if repo == "org/forked-repo" {
			t.Error("Forked repo should not be scanned when includeForks=false")
		}
	}
}

func TestRunOrgMode_IncludeForks(t *testing.T) {
	t.Parallel()

	workflowYAML := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	encoded := base64.StdEncoding.EncodeToString([]byte(workflowYAML))

	orgRepos := []github.RepoEntry{
		{FullName: "org/active-repo", Name: "active-repo", Archived: false, Fork: false},
		{FullName: "org/forked-repo", Name: "forked-repo", Archived: false, Fork: true},
	}

	var scannedRepos []string
	var testMu sync.Mutex
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		testMu.Lock()
		scannedRepos = append(scannedRepos, workDir)
		testMu.Unlock()
		return false
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/orgs/testorg/repos"):
			w.WriteHeader(200)
			body, _ := json.Marshal(orgRepos)
			_, _ = w.Write(body)
		case isWorkflowsDirRequest(path):
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.ContentEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			})
			_, _ = w.Write(body)
		case isWorkflowFileRequest(path):
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestCheckRunnerClient(server)
	rc := &RunContext{
		Ctx:          context.Background(),
		Parser:       workflow.NewParser(),
		GHClient:     client,
		OutputWriter: output.NewWriterWithOptionalCSV(output.FormatText, nil),
	}

	_, _ = RunOrgMode(rc, "testorg", true, processFunc)

	foundFork := false
	for _, repo := range scannedRepos {
		if repo == "org/forked-repo" {
			foundFork = true
		}
	}
	if !foundFork {
		t.Error("Forked repo should be scanned when includeForks=true")
	}
}

func TestRunUserMode(t *testing.T) {
	t.Parallel()

	workflowYAML := `name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`
	encoded := base64.StdEncoding.EncodeToString([]byte(workflowYAML))

	userRepos := []github.RepoEntry{
		{FullName: "user/active-repo", Name: "active-repo", Archived: false, Fork: false},
	}

	var processCalled bool
	var testMu sync.Mutex
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		testMu.Lock()
		processCalled = true
		testMu.Unlock()
		return true
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/users/testuser/repos"):
			w.WriteHeader(200)
			body, _ := json.Marshal(userRepos)
			_, _ = w.Write(body)
		case isWorkflowsDirRequest(path):
			w.WriteHeader(200)
			body, _ := json.Marshal([]github.ContentEntry{
				{Name: "ci.yml", Path: ".github/workflows/ci.yml", Type: "file"},
			})
			_, _ = w.Write(body)
		case isWorkflowFileRequest(path):
			w.WriteHeader(200)
			_, _ = fmt.Fprintf(w, `{"content": "%s"}`, encoded)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestCheckRunnerClient(server)
	rc := &RunContext{
		Ctx:          context.Background(),
		Parser:       workflow.NewParser(),
		GHClient:     client,
		OutputWriter: output.NewWriterWithOptionalCSV(output.FormatText, nil),
	}

	result, err := RunUserMode(rc, "testuser", false, processFunc)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected RunUserMode to return true when processFunc returns true")
	}
	testMu.Lock()
	wasCalled := processCalled
	testMu.Unlock()
	if !wasCalled {
		t.Error("Expected processFunc to be called")
	}
}

func TestRunOrgMode_NoWorkflows(t *testing.T) {
	t.Parallel()

	orgRepos := []github.RepoEntry{
		{FullName: "org/empty-repo", Name: "empty-repo", Archived: false, Fork: false},
	}

	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		t.Error("processFunc should not be called for repo with no workflows")
		return false
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/orgs/testorg/repos"):
			w.WriteHeader(200)
			body, _ := json.Marshal(orgRepos)
			_, _ = w.Write(body)
		case isWorkflowsDirRequest(path):
			w.WriteHeader(404)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestCheckRunnerClient(server)
	rc := &RunContext{
		Ctx:          context.Background(),
		Parser:       workflow.NewParser(),
		GHClient:     client,
		OutputWriter: output.NewWriterWithOptionalCSV(output.FormatText, nil),
	}

	result, err := RunOrgMode(rc, "testorg", false, processFunc)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result {
		t.Error("Expected RunOrgMode to return false when no repos have workflows with issues")
	}
}

func newTestCheckRunnerClient(server *httptest.Server) *github.Client {
	return github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
}

func isWorkflowsDirRequest(path string) bool {
	return strings.HasSuffix(path, "/contents/.github/workflows")
}

func isWorkflowFileRequest(path string) bool {
	return strings.Contains(path, "/contents/.github/workflows/") &&
		(strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml"))
}
