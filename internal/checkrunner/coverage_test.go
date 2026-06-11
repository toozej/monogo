package checkrunner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/go-sort-out-gh-actions/pkg/config"
)

func TestNewRunContext_WithNotifier(t *testing.T) {
	notifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer notifyServer.Close()

	conf := config.Config{
		Notification: config.NotificationConfig{
			GotifyEndpoint: notifyServer.URL,
			GotifyToken:    "test-token",
		},
	}

	rc := NewRunContext("dummy-token", conf, true, false, output.FormatText, nil, false, false, 0)
	defer rc.Close()

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	if rc.Notifier == nil {
		t.Error("Expected Notifier to be non-nil when initNotifier=true")
	}
	if rc.IssueCreator != nil {
		t.Error("Expected IssueCreator to be nil when initIssueCreator=false")
	}
}

func TestNewRunContext_WithNotifierAndIssueCreator(t *testing.T) {
	notifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer notifyServer.Close()

	conf := config.Config{
		Notification: config.NotificationConfig{
			GotifyEndpoint: notifyServer.URL,
			GotifyToken:    "test-token",
		},
	}

	rc := NewRunContext("dummy-token", conf, true, true, output.FormatText, nil, false, false, 0)
	defer rc.Close()

	if rc.Notifier == nil {
		t.Error("Expected Notifier to be non-nil when initNotifier=true")
	}
	if rc.IssueCreator == nil {
		t.Error("Expected IssueCreator to be non-nil when initIssueCreator=true")
	}
}

func TestNewRunContext_NoCache(t *testing.T) {
	rc := NewRunContext("dummy-token", config.Config{}, false, false, output.FormatText, nil, true, false, 0)
	defer rc.Close()

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
}

func TestNewRunContext_RefreshCache(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	rc := NewRunContext("dummy-token", config.Config{}, false, false, output.FormatText, nil, false, true, 24*time.Hour)
	defer rc.Close()

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
}

func TestNewRunContext_JSONFormat(t *testing.T) {
	rc := NewRunContext("dummy-token", config.Config{}, false, false, output.FormatJSON, nil, false, false, 0)
	defer rc.Close()

	if rc == nil {
		t.Fatal("Expected non-nil RunContext")
	}
	if rc.OutputWriter == nil {
		t.Error("Expected OutputWriter to be non-nil")
	}
}

func TestClose_NilGHClient(t *testing.T) {
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test",
		GHClient: nil,
	}

	err := rc.Close()
	if err != nil {
		t.Errorf("Expected nil error for nil GHClient, got %v", err)
	}
}

func TestDetectArchived_DebugMode(t *testing.T) {
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/rate_limit") {
			w.WriteHeader(200)
			body, _ := json.Marshal(map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{
						"limit":     5000,
						"remaining": 4999,
						"used":      1,
						"reset":     time.Now().Add(time.Hour).Unix(),
						"resource":  "core",
					},
				},
			})
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(r.URL.Path, "/repos/") {
			w.WriteHeader(200)
			body, _ := json.Marshal(map[string]interface{}{
				"full_name":           "actions/checkout",
				"archived":            false,
				"name":                "checkout",
				"updated_at":          time.Now().Format(time.RFC3339),
				"deprecated":          false,
				"deprecation_warning": "",
			})
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer rateLimitServer.Close()

	client := github.NewClientWithHTTP(rateLimitServer.URL, rateLimitServer.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Debug:    true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestDetectArchived_VerboseWithAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/") {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}
	if len(result.ArchivedActions) != 0 {
		t.Errorf("Expected 0 archived actions on API error, got %d", len(result.ArchivedActions))
	}
}

func TestDetectStale_VerboseWithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}
	archived := map[string]bool{"actions/checkout": false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)
	if len(staleActions) != 0 {
		t.Errorf("Expected 0 stale actions on API error, got %d", len(staleActions))
	}
}

func TestDetectRuntimeEOL_MixedArchivedAndNonArchived(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n using: node12\n main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/12") {
			resp := map[string]interface{}{
				"schema_version": "1.0.0",
				"result": map[string]interface{}{
					"name":    "12",
					"isEol":   true,
					"eolFrom": "2022-04-30",
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"archived/action": true, "actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if len(result) != 1 {
		t.Fatalf("Expected 1 runtime EOL action (non-archived only), got %d", len(result))
	}
	if result[0].OwnerRepo != "actions/checkout" {
		t.Errorf("Expected OwnerRepo actions/checkout, got %s", result[0].OwnerRepo)
	}
}

func TestDetectRuntimeEOL_VerboseWithRuntimeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Verbose:  true,
	}

	uniqueRepo := "eol-err-test-unique/action"
	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: uniqueRepo, Version: "v3", FullRef: uniqueRepo + "@v3"},
			},
		},
	}
	archived := map[string]bool{uniqueRepo: false}
	nonArchivedRepos := []string{uniqueRepo}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if len(result) != 0 {
		t.Errorf("Expected 0 runtime EOL actions on API error, got %d", len(result))
	}
}

func TestDetectRuntimeEOL_DeduplicationBySeen(t *testing.T) {
	actionContent := "name: Test Action\nruns:\n using: node16\n main: dist/index.js\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(actionContent))

	eolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products/nodejs/releases/16") {
			resp := map[string]interface{}{
				"schema_version": "1.0.0",
				"result": map[string]interface{}{
					"name":    "16",
					"isEol":   false,
					"eolFrom": "",
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer eolServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "action.yml") || strings.Contains(r.URL.Path, "action.yaml") {
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"content": "%s"}`, encoded)
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	client.SetEOLClientForTest(eolServer.URL, eolServer.Client())

	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
		{
			Path: ".github/workflows/release.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	_ = DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)
}

func TestDetectOutdated_DebugMode(t *testing.T) {
	rateLimitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/rate_limit") {
			w.WriteHeader(200)
			body, _ := json.Marshal(map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{
						"limit":     5000,
						"remaining": 4999,
						"used":      1,
						"reset":     time.Now().Add(time.Hour).Unix(),
						"resource":  "core",
					},
				},
			})
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(r.URL.Path, "/repos/") && strings.Contains(r.URL.Path, "/releases/latest") {
			parts := strings.Split(r.URL.Path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "v4",
					"name":     "v4",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/v4", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(r.URL.Path, "/repos/") {
			parts := strings.Split(r.URL.Path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer rateLimitServer.Close()

	client := github.NewClientWithHTTP(rateLimitServer.URL, rateLimitServer.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Debug:    true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, releases := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releases == nil {
		t.Fatal("Expected non-nil releases map")
	}
	_ = outdated
}

func TestDetectOutdated_VerboseWithReleaseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, releases := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if outdated != nil {
		t.Errorf("Expected nil outdated on API error, got %v", outdated)
	}
	if len(releases) > 0 {
		t.Errorf("Expected empty releases on API error, got %v", releases)
	}
}

func TestDetectOutdated_NoReleaseForRepo(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}
	archivedRepos := map[string]bool{
		"actions/checkout": false,
		"actions/setup-go": false,
	}

	server := makeGHServer(archivedRepos, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false, "actions/setup-go": false}
	nonArchivedRepos := []string{"actions/checkout", "actions/setup-go"}

	outdated, releaseMap := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releaseMap == nil {
		t.Fatal("Expected non-nil releases map")
	}
	if _, ok := releaseMap["actions/checkout"]; !ok {
		t.Error("Expected release info for actions/checkout")
	}
	_ = outdated
}

func TestDetectOutdated_ArchivedActionSkipped(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}

	server := makeGHServer(nil, releases, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": true}
	nonArchivedRepos := []string{}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if outdated != nil {
		t.Errorf("Expected nil outdated for all-archived, got %v", outdated)
	}
}

func TestDetectOutdated_SameMajorVersionSameSHA(t *testing.T) {
	sha := "abc123def456abc123def456abc123def456abc1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "v5",
					"name":     "v5",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/v5", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(path, "/git/refs/tags/v5") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v5",
				"object": map[string]interface{}{
					"sha":  sha,
					"type": "commit",
					"url":  "https://api.github.com/repos/" + path,
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") && !strings.Contains(path, "/contents/") && !strings.Contains(path, "/git/") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v5", FullRef: "actions/checkout@v5"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) != 0 {
		t.Errorf("Expected 0 outdated actions when SHA is same, got %d", len(outdated))
	}
}

func TestDetectOutdated_SameMajorVersionDifferentSHA(t *testing.T) {
	sha1 := "abc123def456abc123def456abc123def456abc1"
	sha2 := "def456abc123def456abc123def456abc123def4"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "v5.2.0",
					"name":     "v5.2.0",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/v5.2.0", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(path, "/git/refs/tags/v5") && !strings.Contains(path, "v5.2.0") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v5",
				"object": map[string]interface{}{
					"sha":  sha1,
					"type": "commit",
					"url":  "https://api.github.com/repos/sha-diff-test/checkout/git/refs/tags/v5",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/git/refs/tags/v5.2.0") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v5.2.0",
				"object": map[string]interface{}{
					"sha":  sha2,
					"type": "commit",
					"url":  "https://api.github.com/repos/sha-diff-test/checkout/git/refs/tags/v5.2.0",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") && !strings.Contains(path, "/contents/") && !strings.Contains(path, "/git/") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v5", FullRef: "actions/checkout@v5"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) == 0 {
		t.Error("Expected outdated actions when same major version but different SHA")
	}
}

func TestDetectOutdated_CommitSHAAutoUpToDate(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "sha-diff-test/checkout", Version: "v5", FullRef: "sha-diff-test/checkout@v5"},
			},
		},
	}
	archived := map[string]bool{"sha-diff-test/checkout": false}
	nonArchivedRepos := []string{"sha-diff-test/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) != 0 {
		t.Errorf("Expected 0 outdated for commit SHA ref, got %d", len(outdated))
	}
}

func TestCreateArchivedIssues_WithIssueCreatorError(t *testing.T) {
	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "owner/repo")

	ic := issue.NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []issue.ArchivedActionInfo) error {
		return fmt.Errorf("mock issue creation error: %w", fmt.Errorf("API error"))
	})

	rc := &RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/owner/repo",
		IssueCreator: ic,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)
}

func TestCreateArchivedIssues_ThreePartRepoName(t *testing.T) {
	origEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", origEnv)
	os.Setenv("GITHUB_REPOSITORY", "org/subgroup/repo")

	var testCalled bool

	ic := issue.NewTestIssueCreator(func(ctx context.Context, owner, repo string, archivedActions []issue.ArchivedActionInfo) error {
		testCalled = true
		return nil
	})

	rc := &RunContext{
		Ctx:          context.Background(),
		WorkDir:      "/tmp/owner/repo",
		IssueCreator: ic,
	}

	actions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}

	CreateArchivedIssues(rc, actions)

	if testCalled {
		t.Error("Expected IssueCreator NOT to be called for three-part repo name")
	}
}

func TestRunReposMode_VerboseNoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		return false
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if result != false {
		t.Error("Expected false when no repos found")
	}
}

func TestRunReposMode_WithReposNoActions(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "my-repo")
	workflowsDir := filepath.Join(repoDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}
	workflowContent := "name: CI\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hello\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatalf("Failed to write workflow file: %v", err)
	}

	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	var processCalled bool
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		processCalled = true
		return false
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if processCalled {
		t.Error("Expected processFunc NOT to be called when no actions found in workflows")
	}
	if result {
		t.Error("Expected false when no actions found")
	}
}

func TestRunReposMode_MultipleRepos(t *testing.T) {
	tmpDir := t.TempDir()

	for i, name := range []string{"repo1", "repo2"} {
		repoDir := filepath.Join(tmpDir, name)
		workflowsDir := filepath.Join(repoDir, ".github", "workflows")
		if err := os.MkdirAll(workflowsDir, 0755); err != nil {
			t.Fatalf("Failed to create workflows dir: %v", err)
		}
		workflowContent := fmt.Sprintf("name: CI %d\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v%d\n", i, i+3)
		if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowContent), 0644); err != nil {
			t.Fatalf("Failed to write workflow file: %v", err)
		}
	}

	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	var reposProcessed []string
	processFunc := func(rc *RunContext, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
		reposProcessed = append(reposProcessed, filepath.Base(workDir))
		return true
	}

	result := RunReposMode(rc, tmpDir, processFunc)

	if !result {
		t.Error("Expected true when repos have issues")
	}
	if len(reposProcessed) != 2 {
		t.Errorf("Expected 2 repos processed, got %d", len(reposProcessed))
	}
}

func TestWriteResult_JSONWithAllIssueTypes(t *testing.T) {
	var buf bytes.Buffer
	w := &output.Writer{
		Format: output.FormatJSON,
		Output: &buf,
	}

	archivedActions := []issue.ArchivedActionInfo{
		{Repo: "actions/checkout", Workflow: "ci.yml", Uses: "actions/checkout@v3"},
	}
	staleActions := []actioninfo.StaleActionInfo{
		{OwnerRepo: "actions/old-action", FullRef: "actions/old-action@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "Use v2"},
	}
	runtimeEOLActions := []actioninfo.RuntimeEOLActionInfo{
		{OwnerRepo: "actions/setup-node", FullRef: "actions/setup-node@v3", Workflow: "ci.yml", Runtime: "nodejs", Version: "12", EOLDate: time.Date(2022, 4, 30, 0, 0, 0, 0, time.UTC)},
	}
	outdatedActions := []actioninfo.OutdatedActionInfo{
		{OwnerRepo: "actions/setup-go", CurrentRef: "v3", LatestTag: "v4", LatestURL: "https://github.com/actions/setup-go/releases/tag/v4", Workflow: "ci.yml", FullRef: "actions/setup-go@v3"},
	}

	WriteResult(w, archivedActions, []string{"actions/checkout"}, staleActions, runtimeEOLActions, outdatedActions, nil, true, "Multiple issues found", "")

	out := buf.String()
	if !strings.Contains(out, `"archived_actions"`) {
		t.Errorf("Expected JSON to contain archived_actions, got %q", out)
	}
	if !strings.Contains(out, `"stale_actions"`) {
		t.Errorf("Expected JSON to contain stale_actions, got %q", out)
	}
	if !strings.Contains(out, `"runtime_eol_actions"`) {
		t.Errorf("Expected JSON to contain runtime_eol_actions, got %q", out)
	}
	if !strings.Contains(out, `"outdated_actions"`) {
		t.Errorf("Expected JSON to contain outdated_actions, got %q", out)
	}
	if !strings.Contains(out, `"has_issues"`) {
		t.Errorf("Expected JSON to contain has_issues, got %q", out)
	}
}

func TestDetectArchived_MultipleReposMixedStatus(t *testing.T) {
	archivedRepos := map[string]bool{
		"actions/checkout":   true,
		"actions/setup-go":   false,
		"actions/setup-node": true,
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:       "actions/checkout",
			Archived:       true,
			Name:           "checkout",
			UpdatedAt:      time.Now().Format(time.RFC3339),
			Deprecated:     false,
			DeprecationMsg: "",
		},
		"actions/setup-go": {
			FullName:       "actions/setup-go",
			Archived:       false,
			Name:           "setup-go",
			UpdatedAt:      time.Now().Format(time.RFC3339),
			Deprecated:     false,
			DeprecationMsg: "",
		},
		"actions/setup-node": {
			FullName:       "actions/setup-node",
			Archived:       true,
			Name:           "setup-node",
			UpdatedAt:      time.Now().Format(time.RFC3339),
			Deprecated:     false,
			DeprecationMsg: "",
		},
	}

	server := makeGHServer(archivedRepos, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
				{OwnerRepo: "actions/setup-node", Version: "v3", FullRef: "actions/setup-node@v3"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
		{OwnerRepo: "actions/setup-node", Version: "v3", FullRef: "actions/setup-node@v3"},
	}

	result, err := DetectArchived(rc, workflowFiles, allActionRefs)
	if err != nil {
		t.Fatalf("DetectArchived() error = %v", err)
	}

	if len(result.ArchivedActions) != 2 {
		t.Errorf("Expected 2 archived actions, got %d", len(result.ArchivedActions))
	}
	if len(result.ArchivedRepos) != 2 {
		t.Errorf("Expected 2 unique archived repos, got %d", len(result.ArchivedRepos))
	}
	if len(result.NonArchivedRepos) != 1 {
		t.Errorf("Expected 1 non-archived repo, got %d", len(result.NonArchivedRepos))
	}
}

func TestDetectStale_ZeroDays(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:   ownerRepo,
			Archived:   false,
			Deprecated: false,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 0)

	if len(staleActions) != 0 {
		t.Errorf("Expected 0 stale actions for recently updated repo with zero days, got %d", len(staleActions))
	}
}

func TestDetectStale_VerboseWithNoErrors(t *testing.T) {
	ownerRepo := "actions/checkout"
	repoInfo := map[string]*github.RepoInfo{
		ownerRepo: {
			FullName:   ownerRepo,
			Archived:   false,
			Deprecated: false,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{ownerRepo: false}, nil, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)
	rc.Verbose = true

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	allActionRefs := []workflow.ActionRef{
		{OwnerRepo: ownerRepo, Version: "v4", FullRef: "actions/checkout@v4"},
	}
	archived := map[string]bool{ownerRepo: false}

	staleActions := DetectStale(rc, workflowFiles, allActionRefs, archived, 365)

	if len(staleActions) != 0 {
		t.Errorf("Expected 0 stale actions for recently updated repo, got %d", len(staleActions))
	}
}

func TestDetectRuntimeEOL_EmptyNonArchivedRepos(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
			},
		},
	}
	archived := map[string]bool{"archived/action": true}
	nonArchivedRepos := []string{}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if result != nil {
		t.Errorf("Expected nil when nonArchivedRepos is empty, got %v", result)
	}
}

func TestDetectRuntimeEOL_AllRefsArchived(t *testing.T) {
	server := makeGHServer(nil, nil, nil)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "archived/action1", Version: "v1", FullRef: "archived/action1@v1"},
				{OwnerRepo: "archived/action2", Version: "v2", FullRef: "archived/action2@v2"},
			},
		},
	}
	archived := map[string]bool{"archived/action1": true, "archived/action2": true}
	nonArchivedRepos := []string{}

	result := DetectRuntimeEOL(rc, workflowFiles, archived, nonArchivedRepos)

	if result != nil {
		t.Errorf("Expected nil when all refs are archived, got %v", result)
	}
}

func TestDetectOutdated_VerboseLogging(t *testing.T) {
	sha1 := "aaa111bbb222ccc333ddd444eee555fff666777a"
	sha2 := "bbb222ccc333ddd444eee555fff666aaa111bb"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "v5",
					"name":     "v5",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/v5", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/tags") {
			body, _ := json.Marshal([]map[string]interface{}{
				{"name": "v5.0.0"},
				{"name": "v4.0.0"},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/git/refs/tags/v4") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v4",
				"object": map[string]interface{}{
					"sha":  sha1,
					"type": "commit",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/git/refs/tags/v5") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v5",
				"object": map[string]interface{}{
					"sha":  sha2,
					"type": "commit",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") && !strings.Contains(path, "/contents/") && !strings.Contains(path, "/git/") && !strings.Contains(path, "/tags") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Verbose:  true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) == 0 {
		t.Error("Expected outdated actions when v4 and latest is v5")
	}
}

func TestDetectOutdated_SemverFallbackUsed(t *testing.T) {
	sha1 := "aaa111bbb222ccc333ddd444eee555fff666777a"
	sha2 := "bbb222ccc333ddd444eee555fff666aaa111bb"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "latest",
					"name":     "latest",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/latest", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/tags") {
			body, _ := json.Marshal([]map[string]interface{}{
				{"name": "v5.0.0"},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/git/refs/tags/v4") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v4",
				"object": map[string]interface{}{
					"sha":  sha1,
					"type": "commit",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/git/refs/tags/v5.0.0") {
			body, _ := json.Marshal(map[string]interface{}{
				"ref": "refs/tags/v5.0.0",
				"object": map[string]interface{}{
					"sha":  sha2,
					"type": "commit",
				},
			})
			w.WriteHeader(200)
			_, _ = w.Write(body)
			return
		}
		if strings.Contains(path, "/repos/") && !strings.Contains(path, "/releases") && !strings.Contains(path, "/contents/") && !strings.Contains(path, "/git/") && !strings.Contains(path, "/tags") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Verbose:  true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) == 0 {
		t.Error("Expected outdated actions with semver fallback when release tag is not semver")
	}
}

func TestDetectOutdated_CompareRefSHAsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/repos/") && strings.Contains(path, "/releases/latest") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := strings.Split(parts[1], "/releases")[0]
				body, _ := json.Marshal(map[string]interface{}{
					"tag_name": "v4.2.0",
					"name":     "v4.2.0",
					"html_url": fmt.Sprintf("https://github.com/%s/releases/tag/v4.2.0", ownerRepo),
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		if strings.Contains(path, "/git/refs/") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(path, "/repos/") {
			parts := strings.Split(path, "/repos/")
			if len(parts) == 2 {
				ownerRepo := parts[1]
				body, _ := json.Marshal(map[string]interface{}{
					"full_name":           ownerRepo,
					"archived":            false,
					"name":                strings.Split(ownerRepo, "/")[1],
					"updated_at":          time.Now().Format(time.RFC3339),
					"deprecated":          false,
					"deprecation_warning": "",
				})
				w.WriteHeader(200)
				_, _ = w.Write(body)
				return
			}
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := github.NewClientWithHTTP(server.URL, server.Client(), github.WithCache(false, false, 0))
	rc := &RunContext{
		Ctx:      context.Background(),
		WorkDir:  "/tmp/test-repo",
		Parser:   &workflow.WorkflowParser{},
		GHClient: client,
		Verbose:  true,
	}

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) != 0 {
		t.Errorf("Expected 0 outdated when SHA comparison fails, got %d", len(outdated))
	}
}

func TestDetectOutdated_DifferentMajorVersion(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/setup-go": {TagName: "v5.0.0", Name: "v5.0.0", HTMLURL: "https://github.com/actions/setup-go/releases/tag/v5.0.0"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/setup-go": {
			FullName:  "actions/setup-go",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{"actions/setup-go": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/setup-go", Version: "v3", FullRef: "actions/setup-go@v3"},
			},
		},
	}
	archived := map[string]bool{"actions/setup-go": false}
	nonArchivedRepos := []string{"actions/setup-go"}

	outdated, releaseMap := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if releaseMap == nil {
		t.Fatal("Expected non-nil releases map")
	}
	if len(outdated) == 0 {
		t.Error("Expected outdated action for different major version (v3 vs v5)")
	}
	if len(outdated) > 0 && outdated[0].LatestTag != "v5.0.0" {
		t.Errorf("Expected LatestTag v5.0.0, got %s", outdated[0].LatestTag)
	}
}

func TestDetectOutdated_ExactVersionMatch(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
			},
		},
	}
	archived := map[string]bool{"actions/checkout": false}
	nonArchivedRepos := []string{"actions/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) != 0 {
		t.Errorf("Expected 0 outdated for exact version match, got %d", len(outdated))
	}
}

func TestDetectOutdated_BranchNameRef(t *testing.T) {
	releases := map[string]*github.ReleaseInfo{
		"actions/checkout": {TagName: "v4", Name: "v4", HTMLURL: "https://github.com/actions/checkout/releases/tag/v4"},
	}
	repoInfo := map[string]*github.RepoInfo{
		"actions/checkout": {
			FullName:  "actions/checkout",
			Archived:  false,
			UpdatedAt: time.Now().Format(time.RFC3339),
		},
	}

	server := makeGHServer(map[string]bool{"actions/checkout": false}, releases, repoInfo)
	defer server.Close()

	rc := newTestRunContext(server)

	workflowFiles := []*workflow.WorkflowFile{
		{
			Path: ".github/workflows/ci.yml",
			UsesWithVersions: []workflow.ActionRef{
				{OwnerRepo: "sha-same-test/checkout", Version: "v5", FullRef: "sha-same-test/checkout@v5"},
			},
		},
	}
	archived := map[string]bool{"sha-same-test/checkout": false}
	nonArchivedRepos := []string{"sha-same-test/checkout"}

	outdated, _ := DetectOutdated(rc, workflowFiles, archived, nonArchivedRepos)

	if len(outdated) != 0 {
		t.Errorf("Expected 0 outdated for branch name ref, got %d", len(outdated))
	}
}
