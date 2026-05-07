package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	gh "github.com/toozej/go-find-archived-gh-actions/internal/github"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
)

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string(nil),
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all duplicates",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDuplicates(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("removeDuplicates() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetRepoName(t *testing.T) {
	originalEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", originalEnv)

	os.Unsetenv("GITHUB_REPOSITORY")
	result := getRepoName("/some/fake/path")
	expected := "current-repo"
	if result != expected {
		t.Errorf("getRepoName() = %v, want %v", result, expected)
	}

	os.Setenv("GITHUB_REPOSITORY", "owner/repo")
	result = getRepoName("/some/fake/path")
	expected = "owner/repo"
	if result != expected {
		t.Errorf("getRepoName() = %v, want %v", result, expected)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get user home directory: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		workDir  string
		expected string
	}{
		{
			name:     "tilde expansion",
			path:     "~/some/path",
			workDir:  "/some/workdir",
			expected: filepath.Join(home, "some/path"),
		},
		{
			name:     "tilde with subdirectory",
			path:     "~/src/github/repo/.github/workflows",
			workDir:  "/current/dir",
			expected: filepath.Join(home, "src/github/repo/.github/workflows"),
		},
		{
			name:     "absolute path unchanged",
			path:     "/absolute/path/to/dir",
			workDir:  "/some/workdir",
			expected: "/absolute/path/to/dir",
		},
		{
			name:     "relative path joined with workDir",
			path:     "relative/path",
			workDir:  "/base/dir",
			expected: "/base/dir/relative/path",
		},
		{
			name:     "simple relative path",
			path:     ".github/workflows",
			workDir:  "/home/user/repo",
			expected: "/home/user/repo/.github/workflows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.path, tt.workDir)
			if result != tt.expected {
				t.Errorf("expandPath(%q, %q) = %q, want %q", tt.path, tt.workDir, result, tt.expected)
			}
		})
	}
}

func TestStaleActionInfo(t *testing.T) {
	info := StaleActionInfo{
		OwnerRepo:          "owner/repo",
		FullRef:            "owner/repo@v1",
		Workflow:           "ci.yml",
		Deprecated:         true,
		DeprecationMessage: "Node.js 16 is deprecated",
		StaleByAge:         false,
	}

	if info.OwnerRepo != "owner/repo" {
		t.Errorf("Expected OwnerRepo owner/repo, got %s", info.OwnerRepo)
	}
	if !info.Deprecated {
		t.Error("Expected Deprecated to be true")
	}
	if info.DeprecationMessage != "Node.js 16 is deprecated" {
		t.Errorf("Expected deprecation message, got %s", info.DeprecationMessage)
	}

	info2 := StaleActionInfo{
		OwnerRepo:   "owner/old-repo",
		FullRef:     "owner/old-repo@v2",
		Workflow:    "release.yml",
		Deprecated:  false,
		StaleByAge:  true,
		LastUpdated: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if !info2.StaleByAge {
		t.Error("Expected StaleByAge to be true")
	}
	if info2.Deprecated {
		t.Error("Expected Deprecated to be false")
	}
}

func TestOutdatedActionInfo(t *testing.T) {
	info := OutdatedActionInfo{
		OwnerRepo:  "actions/checkout",
		CurrentRef: "v3",
		LatestTag:  "v4.0.0",
		LatestURL:  "https://github.com/actions/checkout/releases/tag/v4.0.0",
		Workflow:   "ci.yml",
		FullRef:    "actions/checkout@v3",
	}

	if info.OwnerRepo != "actions/checkout" {
		t.Errorf("Expected OwnerRepo actions/checkout, got %s", info.OwnerRepo)
	}
	if info.CurrentRef != "v3" {
		t.Errorf("Expected CurrentRef v3, got %s", info.CurrentRef)
	}
	if info.LatestTag != "v4.0.0" {
		t.Errorf("Expected LatestTag v4.0.0, got %s", info.LatestTag)
	}
}

func newTestGithubClient(server *httptest.Server) *gh.Client {
	return gh.NewClientWithHTTP(server.URL, server.Client())
}

func TestCheckOutdatedActions_FloatingMajorTagSHAComparison(t *testing.T) {
	tests := []struct {
		name              string
		currentRef        string
		latestTag         string
		currentSHA        string
		latestSHA         string
		expectOutdated    bool
		expectOutdatedRef string
	}{
		{
			name:           "floating major v2 same SHA as latest v2.3.9 - not outdated",
			currentRef:     "v2",
			latestTag:      "v2.3.9",
			currentSHA:     "sameCommitSHA",
			latestSHA:      "sameCommitSHA",
			expectOutdated: false,
		},
		{
			name:              "floating major v2 different SHA from latest v2.3.9 - outdated",
			currentRef:        "v2",
			latestTag:         "v2.3.9",
			currentSHA:        "oldCommitSHA",
			latestSHA:         "newCommitSHA",
			expectOutdated:    true,
			expectOutdatedRef: "v2",
		},
		{
			name:           "floating major v1 same SHA as latest v1.2.1 - not outdated",
			currentRef:     "v1",
			latestTag:      "v1.2.1",
			currentSHA:     "sameSHA",
			latestSHA:      "sameSHA",
			expectOutdated: false,
		},
		{
			name:              "floating major v1 different SHA from latest v1.2.1 - outdated",
			currentRef:        "v1",
			latestTag:         "v1.2.1",
			currentSHA:        "oldSHA",
			latestSHA:         "newSHA",
			expectOutdated:    true,
			expectOutdatedRef: "v1",
		},
		{
			name:              "floating major v2 different major from latest v3.0.0 - outdated",
			currentRef:        "v2",
			latestTag:         "v3.0.0",
			currentSHA:        "anySHA",
			latestSHA:         "anySHA2",
			expectOutdated:    true,
			expectOutdatedRef: "v2",
		},
		{
			name:              "pinned minor.patch outdated in same major",
			currentRef:        "v4.1.1",
			latestTag:         "v4.1.2",
			currentSHA:        "old",
			latestSHA:         "new",
			expectOutdated:    true,
			expectOutdatedRef: "v4.1.1",
		},
		{
			name:           "pinned minor.patch current in same major",
			currentRef:     "v4.1.2",
			latestTag:      "v4.1.2",
			currentSHA:     "same",
			latestSHA:      "same",
			expectOutdated: false,
		},
		{
			name:              "pinned minor outdated across majors",
			currentRef:        "v2.1.0",
			latestTag:         "v3.0.3",
			currentSHA:        "any",
			latestSHA:         "any2",
			expectOutdated:    true,
			expectOutdatedRef: "v2.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/repos/owner/repo/releases/latest":
					w.WriteHeader(200)
					if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
						TagName: tt.latestTag,
						HTMLURL: fmt.Sprintf("https://github.com/owner/repo/releases/tag/%s", tt.latestTag),
					}); err != nil {
						t.Errorf("failed to encode release info: %v", err)
					}
				case r.URL.Path == fmt.Sprintf("/repos/owner/repo/git/refs/tags/%s", tt.latestTag):
					w.WriteHeader(200)
					if _, err := w.Write([]byte(fmt.Sprintf(`{"ref":"refs/tags/%s","object":{"sha":"%s","type":"commit"}}`, tt.latestTag, tt.latestSHA))); err != nil {
						t.Errorf("failed to write response: %v", err)
					}
				case r.URL.Path == fmt.Sprintf("/repos/owner/repo/git/refs/tags/%s", tt.currentRef):
					w.WriteHeader(200)
					if _, err := w.Write([]byte(fmt.Sprintf(`{"ref":"refs/tags/%s","object":{"sha":"%s","type":"commit"}}`, tt.currentRef, tt.currentSHA))); err != nil {
						t.Errorf("failed to write response: %v", err)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()

			client := newTestGithubClient(server)
			ctx := context.Background()

			wf := &workflow.WorkflowFile{
				Path: "ci.yaml",
				UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "owner/repo", Version: tt.currentRef, FullRef: "owner/repo@" + tt.currentRef},
				},
			}

			releases := map[string]*gh.ReleaseInfo{
				"owner/repo": {TagName: tt.latestTag, HTMLURL: fmt.Sprintf("https://github.com/owner/repo/releases/tag/%s", tt.latestTag)},
			}
			archived := map[string]bool{"owner/repo": false}

			result := checkOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases)

			if tt.expectOutdated {
				if len(result) != 1 {
					t.Errorf("expected 1 outdated action, got %d", len(result))
				} else if result[0].CurrentRef != tt.expectOutdatedRef {
					t.Errorf("expected outdated ref %s, got %s", tt.expectOutdatedRef, result[0].CurrentRef)
				}
			} else {
				if len(result) != 0 {
					t.Errorf("expected 0 outdated actions, got %d: %+v", len(result), result)
				}
			}
		})
	}
}

func TestCheckOutdatedActions_FloatingMajorTagAnnotatedDereference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v2.3.9",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v2.3.9",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v2.3.9":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2.3.9","object":{"sha":"sameCommitSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagObjSHA","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagObjSHA"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/tags/tagObjSHA":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"object":{"sha":"sameCommitSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	wf := &workflow.WorkflowFile{
		Path: "ci.yaml",
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v2", FullRef: "owner/repo@v2"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v2.3.9", HTMLURL: "https://github.com/owner/repo/releases/tag/v2.3.9"},
	}
	archived := map[string]bool{"owner/repo": false}

	result := checkOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases)
	if len(result) != 0 {
		t.Errorf("expected 0 outdated actions (annotated v2 tag dereferences to same commit as v2.3.9), got %d: %+v", len(result), result)
	}
}

func TestCheckOutdatedActions_FloatingMajorTagStaleAnnotatedTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v2.3.9",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v2.3.9",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v2.3.9":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2.3.9","object":{"sha":"newCommitSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v2","object":{"sha":"tagObjSHA","type":"tag","url":"https://api.github.com/repos/owner/repo/git/tags/tagObjSHA"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/tags/tagObjSHA":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"object":{"sha":"oldCommitSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	wf := &workflow.WorkflowFile{
		Path: "ci.yaml",
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v2", FullRef: "owner/repo@v2"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v2.3.9", HTMLURL: "https://github.com/owner/repo/releases/tag/v2.3.9"},
	}
	archived := map[string]bool{"owner/repo": false}

	result := checkOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases)
	if len(result) != 1 {
		t.Fatalf("expected 1 outdated action (annotated v2 tag points to old commit), got %d", len(result))
	}
	if result[0].CurrentRef != "v2" {
		t.Errorf("expected outdated ref v2, got %s", result[0].CurrentRef)
	}
	if result[0].LatestTag != "v2.3.9" {
		t.Errorf("expected latest tag v2.3.9, got %s", result[0].LatestTag)
	}
}
