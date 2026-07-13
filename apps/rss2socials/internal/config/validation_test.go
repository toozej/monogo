package config

import (
	"strings"
	"testing"
)

func validMastodonConfig() Config {
	return Config{
		FeedURL:              "https://example.com/feed.xml",
		Interval:             1,
		SocialSites:          []string{"mastodon"},
		MastodonURL:          "https://social.example.com",
		MastodonClientKey:    "key",
		MastodonClientSecret: "secret",
		MastodonAccessToken:  "token",
	}
}

func TestValidateRequiredSiteSpecificConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		conf    Config
		wantErr string
	}{
		{
			name: "bluesky only",
			conf: Config{FeedURL: "https://example.com/feed.xml", Interval: 1,
				SocialSites: []string{"bluesky"}, BlueskyHandle: "user.example", BlueskyAppKey: "key"},
		},
		{
			name: "selected site missing credential",
			conf: Config{FeedURL: "https://example.com/feed.xml", Interval: 1,
				SocialSites: []string{"bluesky"}, BlueskyHandle: "user.example"},
			wantErr: "BLUESKY_APPKEY",
		},
		{
			name: "unknown site",
			conf: Config{FeedURL: "https://example.com/feed.xml", Interval: 1,
				SocialSites: []string{"myspace"}},
			wantErr: "unknown social site",
		},
		{
			name: "threads redirect required",
			conf: Config{FeedURL: "https://example.com/feed.xml", Interval: 1,
				SocialSites: []string{"threads"}, ThreadsToken: "token",
				ThreadsClientID: "id", ThreadsClientSecret: "secret"},
			wantErr: "THREADS_REDIRECT_URI",
		},
		{
			name:    "invalid feed URL",
			conf:    func() Config { c := validMastodonConfig(); c.FeedURL = "file:///tmp/feed"; return c }(),
			wantErr: "FEED_URL",
		},
		{
			name: "gotify pair required",
			conf: func() Config {
				c := validMastodonConfig()
				c.GotifyURL = "https://gotify.example.com"
				return c
			}(),
			wantErr: "configured together",
		},
		{
			name:    "non-positive interval",
			conf:    func() Config { c := validMastodonConfig(); c.Interval = 0; return c }(),
			wantErr: "INTERVAL",
		},
		{
			name:    "no social sites configured",
			conf:    Config{FeedURL: "https://example.com/feed.xml", Interval: 1},
			wantErr: "at least one social site",
		},
		{
			name:    "invalid mastodon URL scheme",
			conf:    func() Config { c := validMastodonConfig(); c.MastodonURL = "ftp://social.example.com"; return c }(),
			wantErr: "MASTODON_URL",
		},
		{
			name: "invalid gotify URL scheme",
			conf: func() Config {
				c := validMastodonConfig()
				c.GotifyURL = "ftp://gotify.example.com"
				c.GotifyToken = "token"
				return c
			}(),
			wantErr: "GOTIFY_URL",
		},
		{
			name: "valid config with gotify pair",
			conf: func() Config {
				c := validMastodonConfig()
				c.GotifyURL = "https://gotify.example.com"
				c.GotifyToken = "token"
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequired(tt.conf)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateRequired() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateRequired() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
