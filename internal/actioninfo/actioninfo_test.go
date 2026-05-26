package actioninfo

import (
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

func TestCheckOutdatedActions_SubpathAction(t *testing.T) {
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
			{OwnerRepo: "github/codeql-action", Subpath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
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
	if result[0].Subpath != "init" {
		t.Errorf("expected Subpath init, got %s", result[0].Subpath)
	}
	if result[0].CurrentRef != "v4.35.2" {
		t.Errorf("expected CurrentRef v4.35.2, got %s", result[0].CurrentRef)
	}
	if result[0].LatestTag != "v4.35.4" {
		t.Errorf("expected LatestTag v4.35.4, got %s", result[0].LatestTag)
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

	if err := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false); err != nil {
		t.Fatalf("WriteOutdatedActions failed: %v", err)
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

	if err := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, true, false); err != nil {
		t.Fatalf("WriteOutdatedActions with semver failed: %v", err)
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

func TestWriteOutdatedActions_Subpath(t *testing.T) {
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
			{OwnerRepo: "github/codeql-action", Subpath: "init", Version: "v4.35.2", FullRef: "github/codeql-action/init@v4.35.2"},
		},
	}

	outdated := []OutdatedActionInfo{
		{OwnerRepo: "github/codeql-action", Subpath: "init", CurrentRef: "v4.35.2", LatestTag: "v4.35.4", Workflow: "ci.yaml", FullRef: "github/codeql-action/init@v4.35.2"},
	}

	releases := map[string]*gh.ReleaseInfo{
		"github/codeql-action": {TagName: "v4.35.4", HTMLURL: "https://github.com/github/codeql-action/releases/tag/v4.35.4"},
	}

	// Test SHA mode preserves subpath
	if err := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, false, false); err != nil {
		t.Fatalf("WriteOutdatedActions failed: %v", err)
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
	if err := WriteOutdatedActions(ctx, client, []*workflow.WorkflowFile{wf}, outdated, releases, true, false); err != nil {
		t.Fatalf("WriteOutdatedActions with semver failed: %v", err)
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
