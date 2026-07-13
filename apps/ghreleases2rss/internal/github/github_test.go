package github

import (
	"strings"
	"testing"
)

func TestGetReleaseFeedURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "Valid full GitHub URL",
			input:   "https://github.com/username/repo",
			want:    "https://github.com/username/repo/releases.atom",
			wantErr: false,
		},
		{
			name:    "Valid username/repoName",
			input:   "username/repo",
			want:    "https://github.com/username/repo/releases.atom",
			wantErr: false,
		},
		{
			name:    "Valid hyphenated owner",
			input:   "user-name/repo_name",
			want:    "https://github.com/user-name/repo_name/releases.atom",
			wantErr: false,
		},
		{
			name:    "Valid managed-user owner",
			input:   "owner_name/repo",
			want:    "https://github.com/owner_name/repo/releases.atom",
			wantErr: false,
		},
		{
			name:    "Valid GHCR URL",
			input:   "ghcr.io/username/repo",
			want:    "https://github.com/username/repo/releases.atom",
			wantErr: false,
		},
		{
			name:    "Valid GHCR URL with tag",
			input:   "ghcr.io/username/repo:latest",
			want:    "https://github.com/username/repo/releases.atom",
			wantErr: false,
		},
		{
			name:    "Invalid GitHub URL",
			input:   "https://invalid.com/username/repo",
			want:    "",
			wantErr: true,
		},
		{
			name:    "Invalid GHCR URL",
			input:   "ghcr.io/username",
			want:    "",
			wantErr: true,
		},
		{
			name:    "Invalid username/repoName format",
			input:   "username",
			want:    "",
			wantErr: true,
		},
		{name: "GitHub URL with extra path", input: "https://github.com/owner/repo/issues", wantErr: true},
		{name: "Lookalike GHCR host", input: "https://evilghcr.io/owner/repo", wantErr: true},
		{name: "Invalid repository characters", input: "owner/repo name", wantErr: true},
		{name: "Owner dot segment", input: "../repo", wantErr: true},
		{name: "Repository dot segment", input: "owner/..", wantErr: true},
		{name: "Owner period", input: "owner.name/repo", wantErr: true},
		{name: "Owner leading hyphen", input: "-owner/repo", wantErr: true},
		{name: "Owner trailing hyphen", input: "owner-/repo", wantErr: true},
		{name: "Owner consecutive hyphens", input: "owner--name/repo", wantErr: true},
		{name: "Owner too long", input: strings.Repeat("a", 40) + "/repo", wantErr: true},
		{name: "Repository too long", input: "owner/" + strings.Repeat("a", 101), wantErr: true},
		{name: "GHCR empty tag", input: "ghcr.io/owner/repo:", wantErr: true},
		{name: "GHCR invalid tag", input: "ghcr.io/owner/repo:tag:extra", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetReleaseFeedURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReleaseFeedURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetReleaseFeedURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
