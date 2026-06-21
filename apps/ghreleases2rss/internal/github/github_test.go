package github

import (
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
