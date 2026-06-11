package actioninfo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gh "github.com/toozej/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
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
			result := RemoveDuplicates(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("RemoveDuplicates() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeStaleDays(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{name: "negative value returns default", input: -1, expected: DefaultStaleDays},
		{name: "zero returns default", input: 0, expected: DefaultStaleDays},
		{name: "normal value is kept", input: 180, expected: 180},
		{name: "default value is kept", input: 365, expected: 365},
		{name: "large value is capped", input: 999999, expected: MaxStaleDays},
		{name: "max value is kept", input: MaxStaleDays, expected: MaxStaleDays},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeStaleDays(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeStaleDays(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetRepoName(t *testing.T) {
	originalEnv := os.Getenv("GITHUB_REPOSITORY")
	defer os.Setenv("GITHUB_REPOSITORY", originalEnv)

	os.Unsetenv("GITHUB_REPOSITORY")
	result := GetRepoName("/some/fake/path")
	expected := "current-repo"
	if result != expected {
		t.Errorf("GetRepoName() = %v, want %v", result, expected)
	}

	os.Setenv("GITHUB_REPOSITORY", "owner/repo")
	result = GetRepoName("/some/fake/path")
	expected = "owner/repo"
	if result != expected {
		t.Errorf("GetRepoName() = %v, want %v", result, expected)
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
			result := ExpandPath(tt.path, tt.workDir)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q, %q) = %q, want %q", tt.path, tt.workDir, result, tt.expected)
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

			result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)

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

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
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

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
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

func TestCheckOutdatedActions_ActionPathAction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/github/codeql-action/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v4.35.4",
				HTMLURL: "https://github.com/github/codeql-action/releases/tag/v4.35.4",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/github/codeql-action/git/refs/tags/v4.35.2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.35.2","object":{"sha":"oldSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/github/codeql-action/git/refs/tags/v4.35.4":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.35.4","object":{"sha":"newSHA","type":"commit"}}`)); err != nil {
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
			{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"github/codeql-action": {TagName: "v4.35.4", HTMLURL: "https://github.com/github/codeql-action/releases/tag/v4.35.4"},
	}
	archived := map[string]bool{"github/codeql-action": false}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
	if len(result) != 1 {
		t.Fatalf("expected 1 outdated subpath action, got %d", len(result))
	}
	if result[0].OwnerRepo != "github/codeql-action" {
		t.Errorf("expected OwnerRepo github/codeql-action, got %s", result[0].OwnerRepo)
	}
	if result[0].ActionPath != "init" {
		t.Errorf("expected ActionPath init, got %s", result[0].ActionPath)
	}
	if result[0].CurrentRef != "v4.35.2" {
		t.Errorf("expected CurrentRef v4.35.2, got %s", result[0].CurrentRef)
	}
	if result[0].LatestTag != "v4.35.4" {
		t.Errorf("expected LatestTag v4.35.4, got %s", result[0].LatestTag)
	}
}

func TestCheckOutdatedActions_NonSemverReleaseFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/github/codeql-action/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "codeql-bundle-v2.25.5",
				HTMLURL: "https://github.com/github/codeql-action/releases/tag/codeql-bundle-v2.25.5",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case "/repos/github/codeql-action/tags":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`[{"name":"codeql-bundle-v2.25.5"},{"name":"v4.36.0"}]`)); err != nil {
				t.Errorf("failed to write tags response: %v", err)
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
			{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v3", FullRef: "github/codeql-action/init@v3"},
			{OwnerRepo: "github/codeql-action", ActionPath: "analyze", Version: "v3", FullRef: "github/codeql-action/analyze@v3"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"github/codeql-action": {TagName: "codeql-bundle-v2.25.5", HTMLURL: "https://github.com/github/codeql-action/releases/tag/codeql-bundle-v2.25.5"},
	}
	archived := map[string]bool{"github/codeql-action": false}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, true)
	if len(result) != 2 {
		t.Fatalf("expected 2 outdated action entries, got %d", len(result))
	}

	for _, item := range result {
		if item.LatestTag != "v4.36.0" {
			t.Errorf("expected fallback latest tag v4.36.0, got %s", item.LatestTag)
		}
	}
}

func TestReplaceUsesLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		oldUse   string
		newUse   string
		expected string
	}{
		{
			name:     "standard - uses: format",
			content:  " - uses: actions/checkout@v3\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@abc123 # v3",
			expected: " - uses: actions/checkout@abc123 # v3\n",
		},
		{
			name:     "indented uses: format",
			content:  " uses: actions/setup-go@v4\n",
			oldUse:   "actions/setup-go@v4",
			newUse:   "actions/setup-go@def456 # v4",
			expected: " uses: actions/setup-go@def456 # v4\n",
		},
		{
			name:     "multiple lines with match",
			content:  " - uses: actions/checkout@v3\n - uses: actions/setup-go@v4\n",
			oldUse:   "actions/setup-go@v4",
			newUse:   "actions/setup-go@def456 # v4",
			expected: " - uses: actions/checkout@v3\n - uses: actions/setup-go@def456 # v4\n",
		},
		{
			name:     "same action used multiple times",
			content:  " - uses: actions/checkout@v3\n - uses: actions/setup-go@v4\n - uses: actions/checkout@v3\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@abc123 # v3",
			expected: " - uses: actions/checkout@abc123 # v3\n - uses: actions/setup-go@v4\n - uses: actions/checkout@abc123 # v3\n",
		},
		{
			name:     "no match",
			content:  " - uses: actions/checkout@v3\n",
			oldUse:   "actions/setup-go@v4",
			newUse:   "actions/setup-go@def456 # v4",
			expected: " - uses: actions/checkout@v3\n",
		},
		{
			name:     "SHA pin with nosemgrep comment preserved",
			content:  "  - uses: gitleaks/gitleaks-action@v2 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha.third-party-action-not-pinned-to-commit-sha\n",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@abc123def # v2.3.9",
			expected: "  - uses: gitleaks/gitleaks-action@abc123def # v2.3.9 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha.third-party-action-not-pinned-to-commit-sha\n",
		},
		{
			name:     "SHA pin with short nosemgrep comment",
			content:  "  - uses: github/codeql-action/init@v4.35.2 # nosemgrep: some-comment\n",
			oldUse:   "github/codeql-action/init@v4.35.2",
			newUse:   "github/codeql-action/init@sha456 # v4.35.4",
			expected: "  - uses: github/codeql-action/init@sha456 # v4.35.4 # nosemgrep: some-comment\n",
		},
		{
			name:     "semver mode preserves nosemgrep comment",
			content:  "  - uses: gitleaks/gitleaks-action@v2 # nosemgrep: some-comment\n",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@v2.3.9",
			expected: "  - uses: gitleaks/gitleaks-action@v2.3.9 # nosemgrep: some-comment\n",
		},
		{
			name:     "re-pin SHA replaces old semver comment only",
			content:  "  - uses: actions/checkout@oldSHA # v3\n",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: "  - uses: actions/checkout@newSHA # v4\n",
		},
		{
			name:     "re-pin SHA with nosemgrep replaces old semver comment keeps nosemgrep",
			content:  "  - uses: actions/checkout@oldSHA # v3 # nosemgrep: some-rule\n",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: "  - uses: actions/checkout@newSHA # v4 # nosemgrep: some-rule\n",
		},
		{
			name:     "SHA pin with simple custom comment",
			content:  "  - uses: actions/checkout@v3 # pin this version\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3.1.0",
			expected: "  - uses: actions/checkout@sha789 # v3.1.0 # pin this version\n",
		},
		{
			name:     "SHA pin with comment not starting with semver pattern",
			content:  "  - uses: actions/checkout@v3 # see: https://example.com\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3.1.0",
			expected: "  - uses: actions/checkout@sha789 # v3.1.0 # see: https://example.com\n",
		},
		{
			name:     "indented uses: with trailing comment",
			content:  "      uses: docker/setup-buildx-action@v4 # nosemgrep: some-comment\n",
			oldUse:   "docker/setup-buildx-action@v4",
			newUse:   "docker/setup-buildx-action@newSHA # v4.1.0",
			expected: "      uses: docker/setup-buildx-action@newSHA # v4.1.0 # nosemgrep: some-comment\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReplaceUsesLine([]byte(tt.content), tt.oldUse, tt.newUse)
			if string(result) != tt.expected {
				t.Errorf("ReplaceUsesLine() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestParseNewUse(t *testing.T) {
	tests := []struct {
		name              string
		newUse            string
		wantRefPart       string
		wantSemverComment string
	}{
		{
			name:              "SHA mode with semver comment",
			newUse:            "actions/checkout@abc123 # v3",
			wantRefPart:       "actions/checkout@abc123",
			wantSemverComment: "v3",
		},
		{
			name:              "SHA mode with full semver comment",
			newUse:            "owner/repo@deadbeef # v4.1.2",
			wantRefPart:       "owner/repo@deadbeef",
			wantSemverComment: "v4.1.2",
		},
		{
			name:              "semver mode without comment",
			newUse:            "actions/checkout@v4.1.2",
			wantRefPart:       "actions/checkout@v4.1.2",
			wantSemverComment: "",
		},
		{
			name:              "subpath with SHA and semver comment",
			newUse:            "github/codeql-action/init@sha123 # v4.35.4",
			wantRefPart:       "github/codeql-action/init@sha123",
			wantSemverComment: "v4.35.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refPart, semverComment := parseNewUse(tt.newUse)
			if refPart != tt.wantRefPart {
				t.Errorf("parseNewUse() refPart = %q, want %q", refPart, tt.wantRefPart)
			}
			if semverComment != tt.wantSemverComment {
				t.Errorf("parseNewUse() semverComment = %q, want %q", semverComment, tt.wantSemverComment)
			}
		})
	}
}

func TestBuildReplacementLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		oldUse   string
		newUse   string
		expected string
	}{
		{
			name:     "no trailing comment - SHA mode",
			line:     "  - uses: actions/checkout@v3",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3",
			expected: "  - uses: actions/checkout@sha789 # v3",
		},
		{
			name:     "no trailing comment - semver mode",
			line:     "  - uses: actions/checkout@v3",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4",
			expected: "  - uses: actions/checkout@v4",
		},
		{
			name:     "nosemgrep comment preserved - SHA mode",
			line:     "  - uses: gitleaks/gitleaks-action@v2 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@abc123 # v2.3.9",
			expected: "  - uses: gitleaks/gitleaks-action@abc123 # v2.3.9 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha",
		},
		{
			name:     "nosemgrep comment preserved - semver mode",
			line:     "  - uses: gitleaks/gitleaks-action@v2 # nosemgrep: some-comment",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@v2.3.9",
			expected: "  - uses: gitleaks/gitleaks-action@v2.3.9 # nosemgrep: some-comment",
		},
		{
			name:     "re-pin replaces old semver comment only",
			line:     "  - uses: actions/checkout@oldSHA # v3",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: "  - uses: actions/checkout@newSHA # v4",
		},
		{
			name:     "re-pin replaces old semver comment and keeps other comment",
			line:     "  - uses: actions/checkout@oldSHA # v3 # nosemgrep: rule",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: "  - uses: actions/checkout@newSHA # v4 # nosemgrep: rule",
		},
		{
			name:     "non-semver comment gets semver comment prepended",
			line:     "  - uses: actions/checkout@v3 # see: https://example.com",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3.1.0",
			expected: "  - uses: actions/checkout@sha789 # v3.1.0 # see: https://example.com",
		},
		{
			name:     "subpath with comment preserved",
			line:     "  - uses: github/codeql-action/init@v4.35.2 # nosemgrep: some-comment",
			oldUse:   "github/codeql-action/init@v4.35.2",
			newUse:   "github/codeql-action/init@sha456 # v4.35.4",
			expected: "  - uses: github/codeql-action/init@sha456 # v4.35.4 # nosemgrep: some-comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildReplacementLine(tt.line, tt.oldUse, tt.newUse)
			if result != tt.expected {
				t.Errorf("buildReplacementLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestApplyUpdatesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: actions/checkout@v3\n - uses: actions/setup-go@v4\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	updates := []FileUpdate{
		{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123def # v3"},
		{OldUse: "actions/setup-go@v4", NewUse: "actions/setup-go@def456abc # v4"},
	}

	if err := ApplyUpdatesToFile(filePath, updates); err != nil {
		t.Fatalf("ApplyUpdatesToFile failed: %v", err)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "actions/checkout@abc123def # v3") {
		t.Errorf("Expected checkout to be pinned to SHA, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "actions/setup-go@def456abc # v4") {
		t.Errorf("Expected setup-go to be pinned to SHA, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "actions/checkout@v3") {
		t.Errorf("Expected old ref to be replaced, got: %s", resultStr)
	}
}

func TestApplyUpdatesToFile_WithComments(t *testing.T) {
	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: actions/checkout@v3 # nosemgrep: some-rule\n - uses: gitleaks/gitleaks-action@v2 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	updates := []FileUpdate{
		{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123def # v4.1.2"},
		{OldUse: "gitleaks/gitleaks-action@v2", NewUse: "gitleaks/gitleaks-action@def456abc # v2.3.9"},
	}

	if err := ApplyUpdatesToFile(filePath, updates); err != nil {
		t.Fatalf("ApplyUpdatesToFile failed: %v", err)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "actions/checkout@abc123def # v4.1.2 # nosemgrep: some-rule") {
		t.Errorf("Expected checkout SHA pin with semver comment before nosemgrep comment, got: %s", resultStr)
	}
	if !strings.Contains(resultStr, "gitleaks/gitleaks-action@def456abc # v2.3.9 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha") {
		t.Errorf("Expected gitleaks SHA pin with semver comment before nosemgrep comment, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "actions/checkout@v3") {
		t.Errorf("Expected old checkout ref to be replaced, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "gitleaks/gitleaks-action@v2") {
		t.Errorf("Expected old gitleaks ref to be replaced, got: %s", resultStr)
	}
}

func TestApplyUpdatesToFile_RePinWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	// Simulate a file that was previously pinned with SHA + semver comment + nosemgrep comment
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: actions/checkout@oldSHA # v3 # nosemgrep: some-rule\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	updates := []FileUpdate{
		{OldUse: "actions/checkout@oldSHA", NewUse: "actions/checkout@newSHA # v4"},
	}

	if err := ApplyUpdatesToFile(filePath, updates); err != nil {
		t.Fatalf("ApplyUpdatesToFile failed: %v", err)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "actions/checkout@newSHA # v4 # nosemgrep: some-rule") {
		t.Errorf("Expected re-pinned action with updated semver comment and preserved nosemgrep, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "# v3") {
		t.Errorf("Expected old semver comment to be replaced, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "oldSHA") {
		t.Errorf("Expected old SHA to be replaced, got: %s", resultStr)
	}
}

func TestWriteOutdatedActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v4.1.2",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v4.1.2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.1.2","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "owner/repo@abc123def456 # v4.1.2") {
		t.Errorf("Expected action to be pinned to SHA with semver comment, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "owner/repo@v3") {
		t.Errorf("Expected old ref to be replaced, got: %s", resultStr)
	}
}

func TestWriteOutdatedActions_Semver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v4.1.2",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, true, false)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "owner/repo@v4.1.2") {
		t.Errorf("Expected action to use semver version string, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "owner/repo@v3") {
		t.Errorf("Expected old ref to be replaced, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "# v4.1.2") {
		t.Errorf("Expected no SHA comment with semver mode, got: %s", resultStr)
	}
}

func TestWriteOutdatedActions_ActionPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/github/codeql-action/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v4.35.4",
				HTMLURL: "https://github.com/github/codeql-action/releases/tag/v4.35.4",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/github/codeql-action/git/refs/tags/v4.35.4":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.35.4","object":{"sha":"deadbeef1234","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: github/codeql-action/init@v4.35.2\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "github/codeql-action", ActionPath: "init", CurrentRef: "v4.35.2", LatestTag: "v4.35.4", Workflow: "ci.yaml", FullRef: "github/codeql-action/init@v4.35.2"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"github/codeql-action": {TagName: "v4.35.4", HTMLURL: "https://github.com/github/codeql-action/releases/tag/v4.35.4"},
	}

	// Test SHA mode preserves subpath
	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "github/codeql-action/init@deadbeef1234 # v4.35.4") {
		t.Errorf("Expected subpath action to be pinned to SHA with semver comment, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "github/codeql-action/init@v4.35.2") {
		t.Errorf("Expected old ref to be replaced, got: %s", resultStr)
	}

	// Reset file and test semver mode preserves subpath
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to reset test file: %v", err)
	}
	report = WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, true, false)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}

	result, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr = string(result)
	if !strings.Contains(resultStr, "github/codeql-action/init@v4.35.4") {
		t.Errorf("Expected subpath action to use semver version string, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "github/codeql-action@v4.35.4") {
		t.Errorf("Expected subpath to be preserved, got plain repo ref: %s", resultStr)
	}
}

func TestGetOwnerRepos(t *testing.T) {
	refs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
		{OwnerRepo: "actions/checkout", Version: "v4", FullRef: "actions/checkout@v4"},
	}

	result := GetOwnerRepos(refs)
	if len(result) != 2 {
		t.Errorf("Expected 2 unique owner/repos, got %d", len(result))
	}
}

func TestGetNonArchivedRepos(t *testing.T) {
	refs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
		{OwnerRepo: "actions/setup-go", Version: "v4", FullRef: "actions/setup-go@v4"},
	}

	archived := map[string]bool{
		"actions/checkout": false,
		"archived/action":  true,
		"actions/setup-go": false,
	}

	result := GetNonArchivedRepos(refs, archived)
	if len(result) != 2 {
		t.Errorf("Expected 2 non-archived repos, got %d: %v", len(result), result)
	}
	for _, repo := range result {
		if repo == "archived/action" {
			t.Error("Expected archived/action to be excluded")
		}
	}
}

func TestEmoji(t *testing.T) {
	original := IsTTY
	defer func() { IsTTY = original }()

	IsTTY = true
	if got := Emoji("✅ ", "[OK] "); got != "✅ " {
		t.Errorf("Emoji() with IsTTY=true = %q, want %q", got, "✅ ")
	}

	IsTTY = false
	if got := Emoji("✅ ", "[OK] "); got != "[OK] " {
		t.Errorf("Emoji() with IsTTY=false = %q, want %q", got, "[OK] ")
	}
}

func TestWriteActionOutput_WithEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "github_output")
	if err := os.WriteFile(outputFile, []byte{}, 0600); err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}

	origEnv := os.Getenv("GITHUB_OUTPUT")
	defer os.Setenv("GITHUB_OUTPUT", origEnv)

	os.Setenv("GITHUB_OUTPUT", outputFile)
	WriteActionOutput("test-key", "test-value")

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	expected := "test-key=test-value\n"
	if string(data) != expected {
		t.Errorf("WriteActionOutput() wrote %q, want %q", string(data), expected)
	}
}

func TestWriteActionOutput_NoEnvVar(t *testing.T) {
	origEnv := os.Getenv("GITHUB_OUTPUT")
	defer os.Setenv("GITHUB_OUTPUT", origEnv)

	os.Unsetenv("GITHUB_OUTPUT")
	WriteActionOutput("key", "value")
}

func TestWriteActionOutput_OpenRootError(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "/nonexistent/deeply/nested/dir/file")
	WriteActionOutput("key", "value")
}

func TestWriteActionOutput_OpenFileError(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "github_output")

	t.Setenv("GITHUB_OUTPUT", outputFile)
	if err := os.WriteFile(outputFile, []byte{}, 0000); err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	_ = os.Chmod(outputFile, 0000)
	defer func() {
		_ = os.Chmod(outputFile, 0600)
	}()

	WriteActionOutput("key", "value")
}

func TestLogWorkflowInfo_Verbose(t *testing.T) {
	var buf bytes.Buffer

	workflows := []*workflow.WorkflowFile{
		{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		}},
	}
	refs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	LogWorkflowInfo(&buf, true, workflows, refs)

	output := buf.String()
	if !strings.Contains(output, "Found 1 workflow files") {
		t.Errorf("Expected output to contain workflow count, got: %q", output)
	}
	if !strings.Contains(output, "ci.yaml") {
		t.Errorf("Expected output to contain workflow path, got: %q", output)
	}
	if !strings.Contains(output, "actions/checkout@v3") {
		t.Errorf("Expected output to contain action ref, got: %q", output)
	}
}

func TestLogWorkflowInfo_NotVerbose(t *testing.T) {
	var buf bytes.Buffer

	workflows := []*workflow.WorkflowFile{
		{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
		}},
	}
	refs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
	}

	LogWorkflowInfo(&buf, false, workflows, refs)

	if buf.Len() != 0 {
		t.Errorf("Expected no output when verbose=false, got: %q", buf.String())
	}
}

func TestLogWorkflowInfo_EmptyFullRef(t *testing.T) {
	var buf bytes.Buffer

	workflows := []*workflow.WorkflowFile{
		{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "actions/checkout", Version: "v3", FullRef: ""},
		}},
	}
	refs := []workflow.ActionRef{
		{OwnerRepo: "actions/checkout", Version: "v3", FullRef: ""},
	}

	LogWorkflowInfo(&buf, true, workflows, refs)

	output := buf.String()
	if !strings.Contains(output, "actions/checkout@v3") {
		t.Errorf("Expected fallback owner@version format when FullRef is empty, got: %q", output)
	}
}

func TestMatchesUsesLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		oldUse string
		want   bool
	}{
		{name: "standard match", line: "  - uses: actions/checkout@v3", oldUse: "actions/checkout@v3", want: true},
		{name: "match with trailing comment", line: "  - uses: actions/checkout@v3 # nosemgrep", oldUse: "actions/checkout@v3", want: true},
		{name: "no match different ref", line: "  - uses: actions/setup-go@v4", oldUse: "actions/checkout@v3", want: false},
		{name: "partial match is not a match", line: "  - uses: actions/checkout@v3-extra", oldUse: "actions/checkout@v3", want: false},
		{name: "no uses keyword", line: "  - run: echo hello", oldUse: "actions/checkout@v3", want: false},
		{name: "empty line", line: "", oldUse: "actions/checkout@v3", want: false},
		{name: "exact match at end of line", line: "  - uses: actions/checkout@v3", oldUse: "actions/checkout@v3", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesUsesLine(tt.line, tt.oldUse)
			if got != tt.want {
				t.Errorf("matchesUsesLine(%q, %q) = %v, want %v", tt.line, tt.oldUse, got, tt.want)
			}
		})
	}
}

func TestCheckOutdatedActions_ArchivedExcluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	wf := &workflow.WorkflowFile{
		Path: "ci.yaml",
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "archived/repo", Version: "v1", FullRef: "archived/repo@v1"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"archived/repo": {TagName: "v2.0.0", HTMLURL: "https://github.com/archived/repo/releases/tag/v2.0.0"},
	}
	archived := map[string]bool{"archived/repo": true}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
	if len(result) != 0 {
		t.Errorf("Expected 0 outdated actions for archived repo, got %d: %+v", len(result), result)
	}
}

func TestCheckOutdatedActions_CompareRefSHAsError(t *testing.T) {
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

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, true)
	if len(result) != 0 {
		t.Errorf("Expected 0 outdated actions when SHA comparison fails, got %d: %+v", len(result), result)
	}
}

func TestCheckOutdatedActions_VerboseCompareRefSHAsError(t *testing.T) {
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

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, true)
	if len(result) != 0 {
		t.Errorf("Expected 0 outdated actions, got %d", len(result))
	}
}

func TestCheckOutdatedActions_IsVersionOutdatedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "not-a-semver-tag",
				HTMLURL: "https://github.com/owner/repo/releases/tag/not-a-semver-tag",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
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
			{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "not-a-semver-tag", HTMLURL: "https://github.com/owner/repo/releases/tag/not-a-semver-tag"},
	}
	archived := map[string]bool{"owner/repo": false}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
	if len(result) != 0 {
		t.Errorf("Expected 0 outdated actions when version comparison fails for non-major tag, got %d: %+v", len(result), result)
	}
}

func TestCheckOutdatedActions_DuplicateCacheKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v5.0.0",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v5.0.0",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v3":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v3","object":{"sha":"oldSHA","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v5.0.0":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v5.0.0","object":{"sha":"newSHA","type":"commit"}}`)); err != nil {
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
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v5.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v5.0.0"},
	}
	archived := map[string]bool{"owner/repo": false}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
	if len(result) != 1 {
		t.Errorf("Expected 1 outdated action (duplicate cached), got %d: %+v", len(result), result)
	}
}

func TestCheckOutdatedActions_NoRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	wf := &workflow.WorkflowFile{
		Path: "ci.yaml",
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v1", FullRef: "owner/repo@v1"},
		},
	}

	releases := map[string]*gh.ReleaseInfo{}
	archived := map[string]bool{"owner/repo": false}

	result := CheckOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, archived, releases, false)
	if len(result) != 0 {
		t.Errorf("Expected 0 outdated actions when no release exists, got %d: %+v", len(result), result)
	}
}

func TestWriteOutdatedActions_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	report := WriteOutdatedActions(ctx, client, nil, []OutdatedActionInfo{}, nil, false, false)

	if got := OutdatedUpdateCount(report); got != 0 {
		t.Fatalf("OutdatedUpdateCount() = %d, want 0", got)
	}
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("OutdatedUpdateFailureCount() = %d, want 0", got)
	}
}

func TestReplaceUsesLine_NoTrailingNewline(t *testing.T) {
	content := []byte("  - uses: actions/checkout@v3")
	result := ReplaceUsesLine(content, "actions/checkout@v3", "actions/checkout@abc123 # v3")
	resultStr := string(result)
	if strings.HasSuffix(resultStr, "\n") {
		t.Errorf("Expected result without trailing newline, got: %q", resultStr)
	}
	if !strings.Contains(resultStr, "actions/checkout@abc123 # v3") {
		t.Errorf("Expected replacement to occur, got: %q", resultStr)
	}
}

func TestRuntimeEOLActionInfo_Fields(t *testing.T) {
	info := RuntimeEOLActionInfo{
		OwnerRepo: "actions/runner",
		FullRef:   "actions/runner@v2",
		Workflow:  "ci.yaml",
		Runtime:   "node",
		Version:   "16",
		EOLDate:   time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC),
	}
	if info.OwnerRepo != "actions/runner" {
		t.Errorf("Expected OwnerRepo 'actions/runner', got %s", info.OwnerRepo)
	}
	if info.Runtime != "node" {
		t.Errorf("Expected Runtime 'node', got %s", info.Runtime)
	}
	if info.Version != "16" {
		t.Errorf("Expected Version '16', got %s", info.Version)
	}
	if !info.EOLDate.Equal(time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Expected EOLDate 2023-09-01, got %v", info.EOLDate)
	}
}

func TestFileUpdate_Fields(t *testing.T) {
	u := FileUpdate{
		OldUse: "actions/checkout@v3",
		NewUse: "actions/checkout@abc123 # v3",
	}
	if u.OldUse != "actions/checkout@v3" {
		t.Errorf("Expected OldUse 'actions/checkout@v3', got %s", u.OldUse)
	}
	if u.NewUse != "actions/checkout@abc123 # v3" {
		t.Errorf("Expected NewUse 'actions/checkout@abc123 # v3', got %s", u.NewUse)
	}
}

func TestConstants(t *testing.T) {
	if DefaultStaleDays != 365 {
		t.Errorf("Expected DefaultStaleDays = 365, got %d", DefaultStaleDays)
	}
	if MaxStaleDays != 3650 {
		t.Errorf("Expected MaxStaleDays = 3650, got %d", MaxStaleDays)
	}
}

func TestApplyUpdatesToFile_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	updates := []FileUpdate{
		{OldUse: "actions/setup-go@v4", NewUse: "actions/setup-go@def456 # v4"},
	}

	if err := ApplyUpdatesToFile(filePath, updates); err != nil {
		t.Fatalf("ApplyUpdatesToFile failed: %v", err)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(result) != content {
		t.Errorf("Expected file content unchanged, got: %q", string(result))
	}
}

func TestWriteOutdatedActions_SHAFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.0.0", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.0.0", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.0.0"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false)
	if got := OutdatedUpdateFailureCount(report); got == 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want > 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(result) != content {
		t.Errorf("Expected file content unchanged after SHA fetch failure, got: %q", string(result))
	}
}

func TestPrintOutdatedUpdateReport_WithUpdates(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123 # v3"},
			},
		},
	}

	var buf bytes.Buffer
	PrintOutdatedUpdateReport(&buf, report)

	output := buf.String()
	if !strings.Contains(output, "Updated 1") {
		t.Errorf("Expected update count in output, got: %q", output)
	}
	if !strings.Contains(output, "ci.yaml") {
		t.Errorf("Expected file path in output, got: %q", output)
	}
	if !strings.Contains(output, "actions/checkout@v3") {
		t.Errorf("Expected old use in output, got: %q", output)
	}
	if !strings.Contains(output, "actions/checkout@abc123 # v3") {
		t.Errorf("Expected new use in output, got: %q", output)
	}
}

func TestPrintOutdatedUpdateReport_WithFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		FailedUpdates: []OutdatedUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "actions/checkout@v3", NewUse: "actions/checkout@v4", Reason: "SHA resolution failed"},
		},
	}

	var buf bytes.Buffer
	PrintOutdatedUpdateReport(&buf, report)

	output := buf.String()
	if !strings.Contains(output, "Could not update 1") {
		t.Errorf("Expected failure count in output, got: %q", output)
	}
	if !strings.Contains(output, "SHA resolution failed") {
		t.Errorf("Expected failure reason in output, got: %q", output)
	}
}

func TestPrintOutdatedUpdateReport_Empty(t *testing.T) {
	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{},
	}

	var buf bytes.Buffer
	PrintOutdatedUpdateReport(&buf, report)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty report, got: %q", buf.String())
	}
}

func TestPrintOutdatedUpdateReport_WithUpdatesAndFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123 # v3"},
			},
		},
		FailedUpdates: []OutdatedUpdateFailure{
			{WorkflowFile: "release.yaml", OldUse: "b@v1", NewUse: "b@v2", Reason: "SHA resolution failed"},
		},
	}

	var buf bytes.Buffer
	PrintOutdatedUpdateReport(&buf, report)

	output := buf.String()
	if !strings.Contains(output, "Updated 1") {
		t.Errorf("Expected update count in output, got: %q", output)
	}
	if !strings.Contains(output, "Could not update 1") {
		t.Errorf("Expected failure count in output, got: %q", output)
	}
}

func TestBuildOutdatedUpdateSummary_UpdatedOnly(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@v2"}},
		},
	}
	summary := BuildOutdatedUpdateSummary(report)
	if !strings.Contains(summary, "Updated 1") {
		t.Errorf("Expected updated count in summary, got: %q", summary)
	}
}

func TestBuildOutdatedUpdateSummary_UpdatedAndFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@v2"}},
		},
		FailedUpdates: []OutdatedUpdateFailure{
			{WorkflowFile: "release.yaml", OldUse: "b@v1", NewUse: "b@v2", Reason: "failed"},
		},
	}
	summary := BuildOutdatedUpdateSummary(report)
	if !strings.Contains(summary, "Updated 1") {
		t.Errorf("Expected updated count in summary, got: %q", summary)
	}
	if !strings.Contains(summary, "1 could not be updated") {
		t.Errorf("Expected failure count in summary, got: %q", summary)
	}
}

func TestBuildOutdatedUpdateSummary_FailuresOnly(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		FailedUpdates: []OutdatedUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "a@v1", NewUse: "a@v2", Reason: "failed"},
		},
	}
	summary := BuildOutdatedUpdateSummary(report)
	if !strings.Contains(summary, "could not be updated") {
		t.Errorf("Expected failure message in summary, got: %q", summary)
	}
}

func TestBuildOutdatedUpdateSummary_NoUpdatesNoFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := OutdatedUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{},
	}
	summary := BuildOutdatedUpdateSummary(report)
	if !strings.Contains(summary, "no updates were applied") {
		t.Errorf("Expected no-updates message in summary, got: %q", summary)
	}
}

func TestOutdatedUpdateCount(t *testing.T) {
	tests := []struct {
		name     string
		report   OutdatedUpdateReport
		expected int
	}{
		{name: "empty report", report: OutdatedUpdateReport{UpdatedByFile: map[string][]FileUpdate{}}, expected: 0},
		{name: "single file single update", report: OutdatedUpdateReport{UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@v2"}},
		}}, expected: 1},
		{name: "single file multiple updates", report: OutdatedUpdateReport{UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@v2"}, {OldUse: "b@v1", NewUse: "b@v2"}},
		}}, expected: 2},
		{name: "multiple files", report: OutdatedUpdateReport{UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml":      {{OldUse: "a@v1", NewUse: "a@v2"}},
			"release.yaml": {{OldUse: "b@v1", NewUse: "b@v2"}, {OldUse: "c@v1", NewUse: "c@v2"}},
		}}, expected: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OutdatedUpdateCount(tt.report)
			if got != tt.expected {
				t.Errorf("OutdatedUpdateCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestOutdatedUpdateFailureCount(t *testing.T) {
	report := OutdatedUpdateReport{
		FailedUpdates: []OutdatedUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "a@v1", NewUse: "a@v2", Reason: "failed"},
			{WorkflowFile: "ci.yaml", OldUse: "b@v1", NewUse: "b@v2", Reason: "failed"},
		},
	}
	if got := OutdatedUpdateFailureCount(report); got != 2 {
		t.Errorf("OutdatedUpdateFailureCount() = %d, want 2", got)
	}
}

func TestCheckTTY_NO_COLOR(t *testing.T) {
	orig := os.Getenv("NO_COLOR")
	defer os.Setenv("NO_COLOR", orig)

	os.Setenv("NO_COLOR", "1")
	result := checkTTY()
	if result {
		t.Error("Expected checkTTY to return false when NO_COLOR is set")
	}
}

func TestCheckTTY_TERMDumb(t *testing.T) {
	orig := os.Getenv("TERM")
	defer os.Setenv("TERM", orig)

	os.Setenv("TERM", "dumb")
	result := checkTTY()
	if result {
		t.Error("Expected checkTTY to return false when TERM=dumb")
	}
}

func TestCheckTTY_CI(t *testing.T) {
	orig := os.Getenv("CI")
	defer os.Setenv("CI", orig)

	os.Setenv("CI", "true")
	result := checkTTY()
	if result {
		t.Error("Expected checkTTY to return false when CI is set")
	}
}

func TestCheckTTY_GitHubActions(t *testing.T) {
	orig := os.Getenv("GITHUB_ACTIONS")
	defer os.Setenv("GITHUB_ACTIONS", orig)

	os.Setenv("GITHUB_ACTIONS", "true")
	result := checkTTY()
	if result {
		t.Error("Expected checkTTY to return false when GITHUB_ACTIONS is set")
	}
}

func TestWriteActionOutput_InvalidDir(t *testing.T) {
	origEnv := os.Getenv("GITHUB_OUTPUT")
	defer os.Setenv("GITHUB_OUTPUT", origEnv)

	os.Setenv("GITHUB_OUTPUT", "/nonexistent/path/that/does/not/exist/file")
	WriteActionOutput("key", "value")
}

func TestApplyUpdatesToFile_NonexistentDir(t *testing.T) {
	err := ApplyUpdatesToFile("/nonexistent/dir/file.yaml", []FileUpdate{
		{OldUse: "a@v1", NewUse: "a@v2"},
	})
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestApplyUpdatesToFile_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := ApplyUpdatesToFile(filepath.Join(tmpDir, "nonexistent.yaml"), []FileUpdate{
		{OldUse: "a@v1", NewUse: "a@v2"},
	})
	if err == nil {
		t.Error("Expected error for nonexistent file in existing directory")
	}
}

func TestApplyUpdatesToFile_StatError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: actions/checkout@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	_ = os.Chmod(filePath, 0000)
	defer func() {
		_ = os.Chmod(filePath, 0644)
	}()

	err := ApplyUpdatesToFile(filePath, []FileUpdate{
		{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123 # v3"},
	})
	if err == nil {
		t.Error("Expected error when file has no read permissions")
	}
}

func TestApplyUpdatesToFile_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: actions/checkout@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0444); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	err := ApplyUpdatesToFile(filePath, []FileUpdate{
		{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123 # v3"},
	})
	if err == nil {
		t.Error("Expected error when directory is read-only")
	}
}

func TestReadAll_Error(t *testing.T) {
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	r.Close()

	result, err := readAll(r)
	if err == nil {
		t.Errorf("readAll() expected error, got nil (result=%q)", string(result))
	}
}

func TestReadAll_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "readall_success_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString("hello world"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	result, err := readAll(tmpFile)
	if err != nil {
		t.Fatalf("readAll() unexpected error: %v", err)
	}
	if string(result) != "hello world" {
		t.Errorf("readAll() = %q, want %q", string(result), "hello world")
	}
}

func TestWriteOutdatedActions_WorkflowFileMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "nonexistent.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, nil, outdated, releases, true, false)
	if got := OutdatedUpdateFailureCount(report); got == 0 {
		t.Error("Expected failures for workflow file that doesn't match any WorkflowFile path")
	}
}

func TestWriteOutdatedActions_VerboseSHAResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v4.1.2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.1.2","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, true)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}
}

func TestWriteOutdatedActions_ApplyUpdatesToFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v4.1.2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.1.2","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0444); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false)
	if got := OutdatedUpdateFailureCount(report); got == 0 {
		t.Error("Expected failures when file write fails due to permissions")
	}
}

func TestWriteOutdatedActions_EmptyFullRef(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
	}

	report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, true, false)
	if got := OutdatedUpdateFailureCount(report); got != 0 {
		t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if !strings.Contains(string(result), "owner/repo@v4.1.2") {
		t.Errorf("Expected semver update with empty FullRef, got: %s", string(result))
	}
}

func TestDetectPinnableActions(t *testing.T) {
	tests := []struct {
		name          string
		workflowFiles []*workflow.WorkflowFile
		archived      map[string]bool
		expectedCount int
	}{
		{
			name: "version tag is pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				}},
			},
			archived:      map[string]bool{"actions/checkout": false},
			expectedCount: 1,
		},
		{
			name: "commit SHA is not pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "actions/checkout", Version: "abc123def456", FullRef: "actions/checkout@abc123def456"},
				}},
			},
			archived:      map[string]bool{"actions/checkout": false},
			expectedCount: 0,
		},
		{
			name: "archived action is not pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				}},
			},
			archived:      map[string]bool{"archived/action": true},
			expectedCount: 0,
		},
		{
			name: "mixed actions only non-archived non-SHA are pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
					{OwnerRepo: "actions/setup-go", Version: "abc123def", FullRef: "actions/setup-go@abc123def"},
					{OwnerRepo: "archived/action", Version: "v1", FullRef: "archived/action@v1"},
				}},
			},
			archived:      map[string]bool{"actions/checkout": false, "actions/setup-go": false, "archived/action": true},
			expectedCount: 1,
		},
		{
			name: "duplicate same file same ref counted once",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
					{OwnerRepo: "actions/checkout", Version: "v3", FullRef: "actions/checkout@v3"},
				}},
			},
			archived:      map[string]bool{"actions/checkout": false},
			expectedCount: 1,
		},
		{
			name: "action with subpath is pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
				}},
			},
			archived:      map[string]bool{"github/codeql-action": false},
			expectedCount: 1,
		},
		{
			name: "empty workflow files returns nothing",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{}},
			},
			archived:      map[string]bool{},
			expectedCount: 0,
		},
		{
			name: "branch-name ref is not pinnable",
			workflowFiles: []*workflow.WorkflowFile{
				{Path: "ci.yaml", UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "actions/checkout", Version: "main", FullRef: "actions/checkout@main"},
				}},
			},
			archived:      map[string]bool{"actions/checkout": false},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPinnableActions(tt.workflowFiles, tt.archived)
			if len(result) != tt.expectedCount {
				t.Errorf("DetectPinnableActions() = %d pinnable, want %d: %+v", len(result), tt.expectedCount, result)
			}
		})
	}
}

func TestPinActions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v3":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v3","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	pinnable := []PinActionInfo{
		{OwnerRepo: "owner/repo", Version: "v3", Workflow: filePath, FullRef: "owner/repo@v3"},
	}

	report := PinActions(ctx, client, []*workflow.WorkflowFile{wf}, pinnable, false)
	if got := PinUpdateFailureCount(report); got != 0 {
		t.Fatalf("PinActions() failure count = %d, want 0", got)
	}
	if got := PinUpdateCount(report); got != 1 {
		t.Fatalf("PinActions() update count = %d, want 1", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "owner/repo@abc123def456 # v3") {
		t.Errorf("Expected action to be pinned to SHA with version comment, got: %s", resultStr)
	}
	if strings.Contains(resultStr, "owner/repo@v3\n") {
		t.Errorf("Expected old ref to be replaced, got: %s", resultStr)
	}
}

func TestPinActions_ActionPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/github/codeql-action/git/refs/tags/v4.35.4":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.35.4","object":{"sha":"deadbeef1234","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: github/codeql-action/init@v4.35.4\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v4.35.4", FullRef: "github/codeql-action/init@v4.35.4"},
		},
	}

	pinnable := []PinActionInfo{
		{OwnerRepo: "github/codeql-action", ActionPath: "init", Version: "v4.35.4", Workflow: filePath, FullRef: "github/codeql-action/init@v4.35.4"},
	}

	report := PinActions(ctx, client, []*workflow.WorkflowFile{wf}, pinnable, false)
	if got := PinUpdateFailureCount(report); got != 0 {
		t.Fatalf("PinActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "github/codeql-action/init@deadbeef1234 # v4.35.4") {
		t.Errorf("Expected subpath action to be pinned to SHA with version comment, got: %s", resultStr)
	}
}

func TestPinActions_SHAFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	pinnable := []PinActionInfo{
		{OwnerRepo: "owner/repo", Version: "v3", Workflow: filePath, FullRef: "owner/repo@v3"},
	}

	report := PinActions(ctx, client, []*workflow.WorkflowFile{wf}, pinnable, false)
	if got := PinUpdateFailureCount(report); got == 0 {
		t.Fatal("PinActions() expected failures when SHA resolution fails, got 0")
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(result) != content {
		t.Errorf("Expected file content unchanged after SHA fetch failure, got: %q", string(result))
	}
}

func TestPinActions_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	report := PinActions(ctx, client, nil, []PinActionInfo{}, false)
	if got := PinUpdateCount(report); got != 0 {
		t.Fatalf("PinUpdateCount() = %d, want 0", got)
	}
	if got := PinUpdateFailureCount(report); got != 0 {
		t.Fatalf("PinUpdateFailureCount() = %d, want 0", got)
	}
}

func TestPinActions_WithComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v3":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v3","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tmpDir := t.TempDir()
	content := "name: CI\non: push\njobs:\n test:\n steps:\n - uses: owner/repo@v3 # nosemgrep: some-rule\n"
	filePath := filepath.Join(tmpDir, "ci.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	wf := &workflow.WorkflowFile{
		Path: filePath,
		UsesWithVersions: []workflow.ActionRef{
			{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
		},
	}

	pinnable := []PinActionInfo{
		{OwnerRepo: "owner/repo", Version: "v3", Workflow: filePath, FullRef: "owner/repo@v3"},
	}

	report := PinActions(ctx, client, []*workflow.WorkflowFile{wf}, pinnable, false)
	if got := PinUpdateFailureCount(report); got != 0 {
		t.Fatalf("PinActions() failure count = %d, want 0", got)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "owner/repo@abc123def456 # v3 # nosemgrep: some-rule") {
		t.Errorf("Expected pin with semver comment before nosemgrep comment, got: %s", resultStr)
	}
}

func TestPinUpdateCount(t *testing.T) {
	tests := []struct {
		name     string
		report   PinUpdateReport
		expected int
	}{
		{name: "empty report", report: PinUpdateReport{UpdatedByFile: map[string][]FileUpdate{}}, expected: 0},
		{name: "single file single update", report: PinUpdateReport{UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@sha1 # v1"}},
		}}, expected: 1},
		{name: "multiple files", report: PinUpdateReport{UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml":      {{OldUse: "a@v1", NewUse: "a@sha1 # v1"}},
			"release.yaml": {{OldUse: "b@v2", NewUse: "b@sha2 # v2"}, {OldUse: "c@v3", NewUse: "c@sha3 # v3"}},
		}}, expected: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PinUpdateCount(tt.report)
			if got != tt.expected {
				t.Errorf("PinUpdateCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestPinUpdateFailureCount(t *testing.T) {
	report := PinUpdateReport{
		FailedUpdates: []PinUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "a@v1", NewUse: "a@sha1 # v1", Reason: "failed"},
			{WorkflowFile: "ci.yaml", OldUse: "b@v1", NewUse: "b@sha2 # v1", Reason: "failed"},
		},
	}
	if got := PinUpdateFailureCount(report); got != 2 {
		t.Errorf("PinUpdateFailureCount() = %d, want 2", got)
	}
}

func TestPrintPinUpdateReport_WithUpdates(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123 # v3"},
			},
		},
	}

	var buf bytes.Buffer
	PrintPinUpdateReport(&buf, report)

	output := buf.String()
	if !strings.Contains(output, "Pinned 1") {
		t.Errorf("Expected pin count in output, got: %q", output)
	}
	if !strings.Contains(output, "ci.yaml") {
		t.Errorf("Expected file path in output, got: %q", output)
	}
}

func TestPrintPinUpdateReport_WithFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		FailedUpdates: []PinUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "actions/checkout@v3", NewUse: "actions/checkout@v3", Reason: "SHA resolution failed"},
		},
	}

	var buf bytes.Buffer
	PrintPinUpdateReport(&buf, report)

	output := buf.String()
	if !strings.Contains(output, "Could not pin 1") {
		t.Errorf("Expected failure count in output, got: %q", output)
	}
}

func TestPrintPinUpdateReport_Empty(t *testing.T) {
	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{},
	}

	var buf bytes.Buffer
	PrintPinUpdateReport(&buf, report)

	if buf.Len() > 0 {
		t.Errorf("Expected no output for empty report, got: %q", buf.String())
	}
}

func TestBuildPinUpdateSummary_UpdatedOnly(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@sha1 # v1"}},
		},
	}
	summary := BuildPinUpdateSummary(report)
	if !strings.Contains(summary, "Pinned 1") {
		t.Errorf("Expected pinned count in summary, got: %q", summary)
	}
}

func TestBuildPinUpdateSummary_UpdatedAndFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@sha1 # v1"}},
		},
		FailedUpdates: []PinUpdateFailure{
			{WorkflowFile: "release.yaml", OldUse: "b@v1", NewUse: "b@sha2 # v1", Reason: "failed"},
		},
	}
	summary := BuildPinUpdateSummary(report)
	if !strings.Contains(summary, "Pinned 1") {
		t.Errorf("Expected pinned count in summary, got: %q", summary)
	}
	if !strings.Contains(summary, "1 could not be pinned") {
		t.Errorf("Expected failure count in summary, got: %q", summary)
	}
}

func TestBuildPinUpdateSummary_FailuresOnly(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		FailedUpdates: []PinUpdateFailure{
			{WorkflowFile: "ci.yaml", OldUse: "a@v1", NewUse: "a@sha1 # v1", Reason: "failed"},
		},
	}
	summary := BuildPinUpdateSummary(report)
	if !strings.Contains(summary, "could not be pinned") {
		t.Errorf("Expected failure message in summary, got: %q", summary)
	}
}

func TestBuildPinUpdateSummary_NoUpdatesNoFailures(t *testing.T) {
	origTTY := IsTTY
	defer func() { IsTTY = origTTY }()
	IsTTY = false

	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{},
	}
	summary := BuildPinUpdateSummary(report)
	if !strings.Contains(summary, "none were pinned") {
		t.Errorf("Expected no-pins message in summary, got: %q", summary)
	}
}

func TestPinActionInfo_Fields(t *testing.T) {
	info := PinActionInfo{
		OwnerRepo:  "actions/checkout",
		ActionPath: "",
		Version:    "v3",
		FullRef:    "actions/checkout@v3",
		Workflow:   ".github/workflows/ci.yaml",
	}
	if info.OwnerRepo != "actions/checkout" {
		t.Errorf("Expected OwnerRepo 'actions/checkout', got %s", info.OwnerRepo)
	}
	if info.Version != "v3" {
		t.Errorf("Expected Version 'v3', got %s", info.Version)
	}
	if info.Workflow != ".github/workflows/ci.yaml" {
		t.Errorf("Expected Workflow 'ci.yaml', got %s", info.Workflow)
	}
}

func TestPinUpdateFailure_Fields(t *testing.T) {
	failure := PinUpdateFailure{
		WorkflowFile: "ci.yaml",
		OldUse:       "actions/checkout@v3",
		NewUse:       "actions/checkout@abc123 # v3",
		Reason:       "SHA resolution failed",
	}
	if failure.WorkflowFile != "ci.yaml" {
		t.Errorf("Expected WorkflowFile 'ci.yaml', got %s", failure.WorkflowFile)
	}
	if failure.Reason != "SHA resolution failed" {
		t.Errorf("Expected Reason 'SHA resolution failed', got %s", failure.Reason)
	}
}

func TestPinUpdateReport_Fields(t *testing.T) {
	report := PinUpdateReport{
		UpdatedByFile: map[string][]FileUpdate{
			"ci.yaml": {{OldUse: "a@v1", NewUse: "a@sha1 # v1"}},
		},
		FailedUpdates: []PinUpdateFailure{
			{WorkflowFile: "release.yaml", OldUse: "b@v1", Reason: "failed"},
		},
	}
	if len(report.UpdatedByFile) != 1 {
		t.Errorf("Expected 1 updated file, got %d", len(report.UpdatedByFile))
	}
	if len(report.FailedUpdates) != 1 {
		t.Errorf("Expected 1 failed update, got %d", len(report.FailedUpdates))
	}
}

func TestMatchesUsesLine_QuotedRefs(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		oldUse string
		want   bool
	}{
		{name: "single-quoted match", line: " - uses: 'actions/checkout@v3'", oldUse: "actions/checkout@v3", want: true},
		{name: "double-quoted match", line: " - uses: \"actions/checkout@v3\"", oldUse: "actions/checkout@v3", want: true},
		{name: "single-quoted with trailing comment", line: " - uses: 'actions/checkout@v3' # nosemgrep", oldUse: "actions/checkout@v3", want: true},
		{name: "double-quoted with trailing comment", line: " - uses: \"actions/checkout@v3\" # nosemgrep", oldUse: "actions/checkout@v3", want: true},
		{name: "unquoted still matches", line: " - uses: actions/checkout@v3", oldUse: "actions/checkout@v3", want: true},
		{name: "single-quoted no match different ref", line: " - uses: 'actions/setup-go@v4'", oldUse: "actions/checkout@v3", want: false},
		{name: "double-quoted no match different ref", line: " - uses: \"actions/setup-go@v4\"", oldUse: "actions/checkout@v3", want: false},
		{name: "single-quoted partial match not a match", line: " - uses: 'actions/checkout@v3-extra'", oldUse: "actions/checkout@v3", want: false},
		{name: "double-quoted partial match not a match", line: " - uses: \"actions/checkout@v3-extra\"", oldUse: "actions/checkout@v3", want: false},
		{name: "single-quoted exact match at end of line", line: " - uses: 'actions/checkout@v3'", oldUse: "actions/checkout@v3", want: true},
		{name: "double-quoted exact match at end of line", line: " - uses: \"actions/checkout@v3\"", oldUse: "actions/checkout@v3", want: true},
		{name: "single-quoted subpath action", line: " - uses: 'github/codeql-action/init@v4.35.2'", oldUse: "github/codeql-action/init@v4.35.2", want: true},
		{name: "double-quoted subpath action", line: " - uses: \"github/codeql-action/init@v4.35.2\"", oldUse: "github/codeql-action/init@v4.35.2", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesUsesLine(tt.line, tt.oldUse)
			if got != tt.want {
				t.Errorf("matchesUsesLine(%q, %q) = %v, want %v", tt.line, tt.oldUse, got, tt.want)
			}
		})
	}
}

func TestBuildReplacementLine_QuotedRefs(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		oldUse   string
		newUse   string
		expected string
	}{
		{
			name:     "single-quoted no trailing comment - SHA mode",
			line:     " - uses: 'actions/checkout@v3'",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3",
			expected: " - uses: 'actions/checkout@sha789' # v3",
		},
		{
			name:     "double-quoted no trailing comment - SHA mode",
			line:     " - uses: \"actions/checkout@v3\"",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3",
			expected: " - uses: \"actions/checkout@sha789\" # v3",
		},
		{
			name:     "single-quoted no trailing comment - semver mode",
			line:     " - uses: 'actions/checkout@v3'",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4",
			expected: " - uses: 'actions/checkout@v4'",
		},
		{
			name:     "double-quoted no trailing comment - semver mode",
			line:     " - uses: \"actions/checkout@v3\"",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4",
			expected: " - uses: \"actions/checkout@v4\"",
		},
		{
			name:     "single-quoted with nosemgrep comment - SHA mode",
			line:     " - uses: 'actions/checkout@v3' # nosemgrep: some-rule",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v4.1.2",
			expected: " - uses: 'actions/checkout@sha789' # v4.1.2 # nosemgrep: some-rule",
		},
		{
			name:     "double-quoted with nosemgrep comment - SHA mode",
			line:     " - uses: \"actions/checkout@v3\" # nosemgrep: some-rule",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v4.1.2",
			expected: " - uses: \"actions/checkout@sha789\" # v4.1.2 # nosemgrep: some-rule",
		},
		{
			name:     "single-quoted with nosemgrep comment - semver mode",
			line:     " - uses: 'actions/checkout@v3' # nosemgrep: some-rule",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4.1.2",
			expected: " - uses: 'actions/checkout@v4.1.2' # nosemgrep: some-rule",
		},
		{
			name:     "double-quoted with nosemgrep comment - semver mode",
			line:     " - uses: \"actions/checkout@v3\" # nosemgrep: some-rule",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4.1.2",
			expected: " - uses: \"actions/checkout@v4.1.2\" # nosemgrep: some-rule",
		},
		{
			name:     "single-quoted re-pin replaces old semver comment",
			line:     " - uses: 'actions/checkout@oldSHA' # v3",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: " - uses: 'actions/checkout@newSHA' # v4",
		},
		{
			name:     "double-quoted re-pin replaces old semver comment",
			line:     " - uses: \"actions/checkout@oldSHA\" # v3",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: " - uses: \"actions/checkout@newSHA\" # v4",
		},
		{
			name:     "single-quoted re-pin with nosemgrep keeps nosemgrep",
			line:     " - uses: 'actions/checkout@oldSHA' # v3 # nosemgrep: rule",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: " - uses: 'actions/checkout@newSHA' # v4 # nosemgrep: rule",
		},
		{
			name:     "double-quoted re-pin with nosemgrep keeps nosemgrep",
			line:     " - uses: \"actions/checkout@oldSHA\" # v3 # nosemgrep: rule",
			oldUse:   "actions/checkout@oldSHA",
			newUse:   "actions/checkout@newSHA # v4",
			expected: " - uses: \"actions/checkout@newSHA\" # v4 # nosemgrep: rule",
		},
		{
			name:     "single-quoted subpath with comment preserved",
			line:     " - uses: 'github/codeql-action/init@v4.35.2' # nosemgrep: some-comment",
			oldUse:   "github/codeql-action/init@v4.35.2",
			newUse:   "github/codeql-action/init@sha456 # v4.35.4",
			expected: " - uses: 'github/codeql-action/init@sha456' # v4.35.4 # nosemgrep: some-comment",
		},
		{
			name:     "double-quoted subpath with comment preserved",
			line:     " - uses: \"github/codeql-action/init@v4.35.2\" # nosemgrep: some-comment",
			oldUse:   "github/codeql-action/init@v4.35.2",
			newUse:   "github/codeql-action/init@sha456 # v4.35.4",
			expected: " - uses: \"github/codeql-action/init@sha456\" # v4.35.4 # nosemgrep: some-comment",
		},
		{
			name:     "single-quoted custom comment gets semver prepended",
			line:     " - uses: 'actions/checkout@v3' # see: https://example.com",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3.1.0",
			expected: " - uses: 'actions/checkout@sha789' # v3.1.0 # see: https://example.com",
		},
		{
			name:     "double-quoted custom comment gets semver prepended",
			line:     " - uses: \"actions/checkout@v3\" # see: https://example.com",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@sha789 # v3.1.0",
			expected: " - uses: \"actions/checkout@sha789\" # v3.1.0 # see: https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildReplacementLine(tt.line, tt.oldUse, tt.newUse)
			if result != tt.expected {
				t.Errorf("buildReplacementLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestReplaceUsesLine_QuotedRefs(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		oldUse   string
		newUse   string
		expected string
	}{
		{
			name:     "single-quoted standard replacement",
			content:  " - uses: 'actions/checkout@v3'\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@abc123 # v3",
			expected: " - uses: 'actions/checkout@abc123' # v3\n",
		},
		{
			name:     "double-quoted standard replacement",
			content:  " - uses: \"actions/checkout@v3\"\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@abc123 # v3",
			expected: " - uses: \"actions/checkout@abc123\" # v3\n",
		},
		{
			name:     "single-quoted semver mode replacement",
			content:  " - uses: 'actions/checkout@v3'\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4",
			expected: " - uses: 'actions/checkout@v4'\n",
		},
		{
			name:     "double-quoted semver mode replacement",
			content:  " - uses: \"actions/checkout@v3\"\n",
			oldUse:   "actions/checkout@v3",
			newUse:   "actions/checkout@v4",
			expected: " - uses: \"actions/checkout@v4\"\n",
		},
		{
			name:     "single-quoted with nosemgrep comment preserved",
			content:  " - uses: 'gitleaks/gitleaks-action@v2' # nosemgrep: some-comment\n",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@abc123def # v2.3.9",
			expected: " - uses: 'gitleaks/gitleaks-action@abc123def' # v2.3.9 # nosemgrep: some-comment\n",
		},
		{
			name:     "double-quoted with nosemgrep comment preserved",
			content:  " - uses: \"gitleaks/gitleaks-action@v2\" # nosemgrep: some-comment\n",
			oldUse:   "gitleaks/gitleaks-action@v2",
			newUse:   "gitleaks/gitleaks-action@abc123def # v2.3.9",
			expected: " - uses: \"gitleaks/gitleaks-action@abc123def\" # v2.3.9 # nosemgrep: some-comment\n",
		},
		{
			name:     "mixed quoted and unquoted lines - only matching line updated",
			content:  " - uses: actions/checkout@v3\n - uses: 'actions/setup-go@v4'\n - uses: \"docker/build-push-action@v5\"\n",
			oldUse:   "actions/setup-go@v4",
			newUse:   "actions/setup-go@def456 # v4",
			expected: " - uses: actions/checkout@v3\n - uses: 'actions/setup-go@def456' # v4\n - uses: \"docker/build-push-action@v5\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReplaceUsesLine([]byte(tt.content), tt.oldUse, tt.newUse)
			if string(result) != tt.expected {
				t.Errorf("ReplaceUsesLine() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestApplyUpdatesToFile_QuotedRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		content             string
		updates             []FileUpdate
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name:    "single-quoted SHA pin",
			content: "name: CI\non: push\njobs:\n test:\n steps:\n - uses: 'actions/checkout@v3'\n - uses: 'actions/setup-go@v4'\n",
			updates: []FileUpdate{
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123def # v3"},
				{OldUse: "actions/setup-go@v4", NewUse: "actions/setup-go@def456abc # v4"},
			},
			expectedContains:    []string{"'actions/checkout@abc123def' # v3", "'actions/setup-go@def456abc' # v4"},
			expectedNotContains: []string{"'actions/checkout@v3'", "'actions/setup-go@v4'"},
		},
		{
			name:    "double-quoted SHA pin",
			content: "name: CI\non: push\njobs:\n test:\n steps:\n - uses: \"actions/checkout@v3\"\n - uses: \"actions/setup-go@v4\"\n",
			updates: []FileUpdate{
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123def # v3"},
				{OldUse: "actions/setup-go@v4", NewUse: "actions/setup-go@def456abc # v4"},
			},
			expectedContains:    []string{"\"actions/checkout@abc123def\" # v3", "\"actions/setup-go@def456abc\" # v4"},
			expectedNotContains: []string{"\"actions/checkout@v3\"", "\"actions/setup-go@v4\""},
		},
		{
			name:    "mixed quotes with comments",
			content: "name: CI\non: push\njobs:\n test:\n steps:\n - uses: 'actions/checkout@v3' # nosemgrep: some-rule\n - uses: \"gitleaks/gitleaks-action@v2\" # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha\n",
			updates: []FileUpdate{
				{OldUse: "actions/checkout@v3", NewUse: "actions/checkout@abc123def # v4.1.2"},
				{OldUse: "gitleaks/gitleaks-action@v2", NewUse: "gitleaks/gitleaks-action@def456abc # v2.3.9"},
			},
			expectedContains: []string{
				"'actions/checkout@abc123def' # v4.1.2 # nosemgrep: some-rule",
				"\"gitleaks/gitleaks-action@def456abc\" # v2.3.9 # nosemgrep: yaml.github-actions.security.third-party-action-not-pinned-to-commit-sha",
			},
			expectedNotContains: []string{"'actions/checkout@v3'", "\"gitleaks/gitleaks-action@v2\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "ci.yaml")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			if err := ApplyUpdatesToFile(filePath, tt.updates); err != nil {
				t.Fatalf("ApplyUpdatesToFile failed: %v", err)
			}

			result, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			resultStr := string(result)
			for _, expected := range tt.expectedContains {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, resultStr)
				}
			}
			for _, notExpected := range tt.expectedNotContains {
				if strings.Contains(resultStr, notExpected) {
					t.Errorf("Expected result NOT to contain %q, got: %s", notExpected, resultStr)
				}
			}
		})
	}
}

func TestWriteOutdatedActions_QuotedRefs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/releases/latest":
			w.WriteHeader(200)
			if err := json.NewEncoder(w).Encode(gh.ReleaseInfo{
				TagName: "v4.1.2",
				HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2",
			}); err != nil {
				t.Errorf("failed to encode release info: %v", err)
			}
		case r.URL.Path == "/repos/owner/repo/git/refs/tags/v4.1.2":
			w.WriteHeader(200)
			if _, err := w.Write([]byte(`{"ref":"refs/tags/v4.1.2","object":{"sha":"abc123def456","type":"commit"}}`)); err != nil {
				t.Errorf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := newTestGithubClient(server)
	ctx := context.Background()

	tests := []struct {
		name                string
		content             string
		useSemver           bool
		expectedContains    string
		expectedNotContains string
	}{
		{
			name:                "single-quoted SHA mode",
			content:             "name: CI\non: push\njobs:\n test:\n steps:\n - uses: 'owner/repo@v3'\n",
			useSemver:           false,
			expectedContains:    "'owner/repo@abc123def456' # v4.1.2",
			expectedNotContains: "'owner/repo@v3'",
		},
		{
			name:                "double-quoted SHA mode",
			content:             "name: CI\non: push\njobs:\n test:\n steps:\n - uses: \"owner/repo@v3\"\n",
			useSemver:           false,
			expectedContains:    "\"owner/repo@abc123def456\" # v4.1.2",
			expectedNotContains: "\"owner/repo@v3\"",
		},
		{
			name:                "single-quoted semver mode",
			content:             "name: CI\non: push\njobs:\n test:\n steps:\n - uses: 'owner/repo@v3'\n",
			useSemver:           true,
			expectedContains:    "'owner/repo@v4.1.2'",
			expectedNotContains: "'owner/repo@v3'",
		},
		{
			name:                "double-quoted semver mode",
			content:             "name: CI\non: push\njobs:\n test:\n steps:\n - uses: \"owner/repo@v3\"\n",
			useSemver:           true,
			expectedContains:    "\"owner/repo@v4.1.2\"",
			expectedNotContains: "\"owner/repo@v3\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "ci.yaml")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			wf := &workflow.WorkflowFile{
				Path: filePath,
				UsesWithVersions: []workflow.ActionRef{
					{OwnerRepo: "owner/repo", Version: "v3", FullRef: "owner/repo@v3"},
				},
			}

			outdated := []OutdatedActionInfo{
				{OwnerRepo: "owner/repo", CurrentRef: "v3", LatestTag: "v4.1.2", Workflow: "ci.yaml", FullRef: "owner/repo@v3"},
			}

			releases := map[string]*gh.ReleaseInfo{
				"owner/repo": {TagName: "v4.1.2", HTMLURL: "https://github.com/owner/repo/releases/tag/v4.1.2"},
			}

			report := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, tt.useSemver, false)
			if got := OutdatedUpdateFailureCount(report); got != 0 {
				t.Fatalf("WriteOutdatedActions() failure count = %d, want 0", got)
			}

			result, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			resultStr := string(result)
			if !strings.Contains(resultStr, tt.expectedContains) {
				t.Errorf("Expected result to contain %q, got: %s", tt.expectedContains, resultStr)
			}
			if strings.Contains(resultStr, tt.expectedNotContains) {
				t.Errorf("Expected result NOT to contain %q, got: %s", tt.expectedNotContains, resultStr)
			}
		})
	}
}
