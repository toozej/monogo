package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
