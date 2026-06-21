package checkrunner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/monogo/pkg/go-sort-out-gh-actions/config"
)

func TestRunContext_CacheReuseAcrossRuns(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":false,"name":"repo","full_name":"owner/repo","updated_at":"2025-01-01T00:00:00Z"}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	// First run: populate cache via NewClient with default caching enabled
	client1 := github.NewClientWithHTTP(server.URL, server.Client())
	defer func() { _ = client1.Close() }()

	rc1 := NewRunContext("token", config.Config{}, false, false, output.FormatText, nil, false, false, 24*time.Hour)
	rc1.GHClient = client1

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
	}

	_, err := DetectArchived(rc1, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("First DetectArchived failed: %v", err)
	}

	if err := client1.Close(); err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("Expected 1 API call on first run, got %d", callCount)
	}

	// Second run: should load from disk cache and make 0 API calls
	callCount = 0
	client2 := github.NewClientWithHTTP(server.URL, server.Client())
	defer func() { _ = client2.Close() }()

	rc2 := NewRunContext("token", config.Config{}, false, false, output.FormatText, nil, false, false, 24*time.Hour)
	rc2.GHClient = client2

	_, err = DetectArchived(rc2, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("Second DetectArchived failed: %v", err)
	}

	if callCount != 0 {
		t.Errorf("Expected 0 API calls on second run (disk cache), got %d", callCount)
	}
}

func TestRunContext_NoCacheMakesExtraCalls(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		if _, err := w.Write([]byte(`{"archived":false,"name":"repo","full_name":"owner/repo","updated_at":"2025-01-01T00:00:00Z"}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Run with no-cache enabled
	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 24*time.Hour))
	defer func() { _ = client.Close() }()

	rc := NewRunContext("token", config.Config{}, false, false, output.FormatText, nil, true, false, 24*time.Hour)
	rc.GHClient = client

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
	}

	_, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived failed: %v", err)
	}

	_, err = DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("Second DetectArchived failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 API calls with cache disabled, got %d", callCount)
	}
}

func TestRunContext_Close(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	rc := NewRunContext("token", config.Config{}, false, false, output.FormatText, nil, false, false, 24*time.Hour)
	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}

	if err := rc.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify cache directory was created
	cacheDir := tmpDir + "/go-sort-out-gh-actions"
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Expected cache directory to exist after Close")
	}
}
