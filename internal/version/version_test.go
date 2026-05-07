package version

import (
	"testing"
)

func TestIsVersionOutdated(t *testing.T) {
	tests := []struct {
		name         string
		current      string
		latest       string
		wantOutdated bool
		wantError    bool
	}{
		{
			name:         "same version",
			current:      "v1.2.3",
			latest:       "v1.2.3",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "outdated patch",
			current:      "v1.2.2",
			latest:       "v1.2.3",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "outdated minor",
			current:      "v1.1.0",
			latest:       "v1.2.0",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "outdated major",
			current:      "v1.0.0",
			latest:       "v2.0.0",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "current is newer (should not happen but handle)",
			current:      "v2.0.0",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "major version only - outdated",
			current:      "v1",
			latest:       "v2",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "major version only - same",
			current:      "v2",
			latest:       "v2.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "major.minor version",
			current:      "v1.2",
			latest:       "v1.3.0",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "commit SHA - not comparable",
			current:      "abc123def456",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "short commit SHA",
			current:      "abc1234",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "branch name main",
			current:      "main",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "branch name master",
			current:      "master",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    false,
		},
		{
			name:         "without v prefix",
			current:      "1.2.3",
			latest:       "1.3.0",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "mixed v prefix",
			current:      "v1.2.3",
			latest:       "1.3.0",
			wantOutdated: true,
			wantError:    false,
		},
		{
			name:         "invalid version string",
			current:      "invalid-version",
			latest:       "v1.0.0",
			wantOutdated: false,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdated, err := IsVersionOutdated(tt.current, tt.latest)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if outdated != tt.wantOutdated {
				t.Errorf("IsVersionOutdated(%s, %s) = %v, want %v", tt.current, tt.latest, outdated, tt.wantOutdated)
			}
		})
	}
}

func TestIsCommitSHA(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123def456", true},
		{"abc1234", true},
		{"ABCDEF1234567890", true},
		{"abc123", false}, // too short
		{"v1.0.0", false},
		{"main", false},
		{"ghijkl", false}, // not hex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isCommitSHA(tt.input); got != tt.want {
				t.Errorf("isCommitSHA(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"main", true},
		{"master", true},
		{"MAIN", true}, // case insensitive
		{"develop", true},
		{"dev", true},
		{"staging", true},
		{"production", true},
		{"prod", true},
		{"feature-branch", false},
		{"v1.0.0", false},
		{"abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isBranchName(tt.input); got != tt.want {
				t.Errorf("isBranchName(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"v1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
		{"v1", "1.0.0", false},
		{"v1.2", "1.2.0", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := parseSemver(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if v.String() != tt.expected {
				t.Errorf("parseSemver(%s) = %s, want %s", tt.input, v.String(), tt.expected)
			}
		})
	}
}

func TestIsMajorVersionTag(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"v1", true},
		{"v2", true},
		{"v10", true},
		{"v0", true},
		{"v1.0", false},
		{"v1.2.3", false},
		{"main", false},
		{"master", false},
		{"abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsMajorVersionTag(tt.input); got != tt.want {
				t.Errorf("IsMajorVersionTag(%s) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetMajorVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"v1.2.3", 1},
		{"v2.0.0", 2},
		{"v10.5.3", 10},
		{"1.0.0", 1},
		{"v1", 1},
		{"invalid", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := GetMajorVersion(tt.input); got != tt.expected {
				t.Errorf("GetMajorVersion(%s) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSameMajorVersion(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want bool
	}{
		{"v1.0.0", "v1.2.3", true},
		{"v1", "v1.5.0", true},
		{"v2.0.0", "v2.1.0", true},
		{"v1.0.0", "v2.0.0", false},
		{"v1", "v2", false},
		{"invalid", "v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_"+tt.v2, func(t *testing.T) {
			if got := SameMajorVersion(tt.v1, tt.v2); got != tt.want {
				t.Errorf("SameMajorVersion(%s, %s) = %v, want %v", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}
