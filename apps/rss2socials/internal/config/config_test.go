package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name                         string
		setupEnvVars                 map[string]string
		setupEnvFileContent          string
		expectError                  bool
		expectedMastodonURL          string
		expectedMastodonClientKey    string
		expectedMastodonClientSecret string
		expectedMastodonToken        string
		expectedGotifyURL            string
		expectedGotifyToken          string
		expectedDebug                bool
		expectedFeedURL              string
		expectedInterval             int
		expectedBlueskyHandle        string
		expectedBlueskyAppKey        string
		expectedThreadsClientID      string
		expectedThreadsClientSecret  string
		expectedThreadsRedirectURI   string
		expectedThreadsToken         string
	}{
		{
			name: "Valid environment variables",
			setupEnvVars: map[string]string{
				"MASTODON_URL":           "https://mastodon.example.com",
				"MASTODON_CLIENT_KEY":    "clientkey123",
				"MASTODON_CLIENT_SECRET": "clientsecret456",
				"MASTODON_ACCESS_TOKEN":  "token123",
				"GOTIFY_URL":             "https://gotify.example.com",
				"GOTIFY_TOKEN":           "gotifytoken456",
				"DEBUG":                  "true",
				"FEED_URL":               "https://example.com/rss",
				"INTERVAL":               "30",
				"BLUESKY_HANDLE":         "user.bsky.social",
				"BLUESKY_APPKEY":         "appkey123",
				"THREADS_CLIENT_ID":      "threadsclientid123",
				"THREADS_CLIENT_SECRET":  "threadsclientsecret456",
				"THREADS_REDIRECT_URI":   "https://example.com/callback",
				"THREADS_ACCESS_TOKEN":   "threadstoken789",
			},
			expectError:                  false,
			expectedMastodonURL:          "https://mastodon.example.com",
			expectedMastodonClientKey:    "clientkey123",
			expectedMastodonClientSecret: "clientsecret456",
			expectedMastodonToken:        "token123",
			expectedGotifyURL:            "https://gotify.example.com",
			expectedGotifyToken:          "gotifytoken456",
			expectedDebug:                true,
			expectedFeedURL:              "https://example.com/rss",
			expectedInterval:             30,
			expectedBlueskyHandle:        "user.bsky.social",
			expectedBlueskyAppKey:        "appkey123",
			expectedThreadsClientID:      "threadsclientid123",
			expectedThreadsClientSecret:  "threadsclientsecret456",
			expectedThreadsRedirectURI:   "https://example.com/callback",
			expectedThreadsToken:         "threadstoken789",
		},
		{
			name:                         "Valid .env file",
			setupEnvFileContent:          "MASTODON_URL=https://mastodon.env.com\nMASTODON_CLIENT_KEY=envclientkey\nMASTODON_CLIENT_SECRET=envclientsecret\nMASTODON_ACCESS_TOKEN=envtoken\nGOTIFY_URL=https://gotify.env.com\nGOTIFY_TOKEN=envgotifytoken\nDEBUG=false\nFEED_URL=https://env.com/rss\nINTERVAL=45\nBLUESKY_HANDLE=envuser.bsky.social\nBLUESKY_APPKEY=envappkey\nTHREADS_CLIENT_ID=envthreadsclientid\nTHREADS_CLIENT_SECRET=envthreadsclientsecret\nTHREADS_REDIRECT_URI=https://env.com/callback\nTHREADS_ACCESS_TOKEN=envthreadstoken\n",
			expectError:                  false,
			expectedMastodonURL:          "https://mastodon.env.com",
			expectedMastodonClientKey:    "envclientkey",
			expectedMastodonClientSecret: "envclientsecret",
			expectedMastodonToken:        "envtoken",
			expectedGotifyURL:            "https://gotify.env.com",
			expectedGotifyToken:          "envgotifytoken",
			expectedDebug:                false,
			expectedFeedURL:              "https://env.com/rss",
			expectedInterval:             45,
			expectedBlueskyHandle:        "envuser.bsky.social",
			expectedBlueskyAppKey:        "envappkey",
			expectedThreadsClientID:      "envthreadsclientid",
			expectedThreadsClientSecret:  "envthreadsclientsecret",
			expectedThreadsRedirectURI:   "https://env.com/callback",
			expectedThreadsToken:         "envthreadstoken",
		},
		{
			name: "Environment variable overrides .env file",
			setupEnvVars: map[string]string{
				"MASTODON_URL":           "https://override.com",
				"MASTODON_CLIENT_KEY":    "overrideclientkey",
				"MASTODON_CLIENT_SECRET": "overrideclientsecret",
				"MASTODON_ACCESS_TOKEN":  "token123",
				"GOTIFY_URL":             "https://gotify.example.com",
				"GOTIFY_TOKEN":           "gotifytoken456",
				"BLUESKY_HANDLE":         "override.bsky.social",
				"BLUESKY_APPKEY":         "overrideappkey",
				"THREADS_CLIENT_ID":      "overridethreadsclientid",
				"THREADS_CLIENT_SECRET":  "overridethreadsclientsecret",
				"THREADS_REDIRECT_URI":   "https://override.com/callback",
				"THREADS_ACCESS_TOKEN":   "overridethreadstoken",
			},
			setupEnvFileContent:          "MASTODON_URL=https://file.com\nMASTODON_CLIENT_KEY=fileclientkey\nMASTODON_CLIENT_SECRET=fileclientsecret\nMASTODON_ACCESS_TOKEN=filetoken\nGOTIFY_URL=https://gotify.file.com\nGOTIFY_TOKEN=filetoken\nBLUESKY_HANDLE=file.bsky.social\nBLUESKY_APPKEY=fileappkey\nTHREADS_CLIENT_ID=filethreadsclientid\nTHREADS_CLIENT_SECRET=filethreadsclientsecret\nTHREADS_REDIRECT_URI=https://file.com/callback\nTHREADS_ACCESS_TOKEN=filethreadstoken\n",
			expectError:                  false,
			expectedMastodonURL:          "https://override.com",
			expectedMastodonClientKey:    "overrideclientkey",
			expectedMastodonClientSecret: "overrideclientsecret",
			expectedMastodonToken:        "token123",
			expectedGotifyURL:            "https://gotify.example.com",
			expectedGotifyToken:          "gotifytoken456",
			expectedDebug:                false,
			expectedFeedURL:              "",
			expectedInterval:             60,
			expectedBlueskyHandle:        "override.bsky.social",
			expectedBlueskyAppKey:        "overrideappkey",
			expectedThreadsClientID:      "overridethreadsclientid",
			expectedThreadsClientSecret:  "overridethreadsclientsecret",
			expectedThreadsRedirectURI:   "https://override.com/callback",
			expectedThreadsToken:         "overridethreadstoken",
		},
		{
			name: "Default interval",
			setupEnvVars: map[string]string{
				"MASTODON_URL":           "https://mastodon.example.com",
				"MASTODON_CLIENT_KEY":    "clientkey123",
				"MASTODON_CLIENT_SECRET": "clientsecret456",
				"MASTODON_ACCESS_TOKEN":  "token123",
				"GOTIFY_URL":             "https://gotify.example.com",
				"GOTIFY_TOKEN":           "gotifytoken456",
			},
			expectError:                  false,
			expectedMastodonURL:          "https://mastodon.example.com",
			expectedMastodonClientKey:    "clientkey123",
			expectedMastodonClientSecret: "clientsecret456",
			expectedMastodonToken:        "token123",
			expectedGotifyURL:            "https://gotify.example.com",
			expectedGotifyToken:          "gotifytoken456",
			expectedDebug:                false,
			expectedFeedURL:              "",
			expectedInterval:             60,
			expectedBlueskyHandle:        "",
			expectedBlueskyAppKey:        "",
			expectedThreadsClientID:      "",
			expectedThreadsClientSecret:  "",
			expectedThreadsRedirectURI:   "",
			expectedThreadsToken:         "",
		},
		{
			name: "Missing required env vars returns error",
			setupEnvVars: map[string]string{
				"MASTODON_URL": "https://mastodon.example.com",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			clearEnvVars := []string{"MASTODON_URL", "MASTODON_CLIENT_KEY", "MASTODON_CLIENT_SECRET", "MASTODON_ACCESS_TOKEN", "GOTIFY_URL", "GOTIFY_TOKEN", "DEBUG", "FEED_URL", "INTERVAL", "BLUESKY_HANDLE", "BLUESKY_APPKEY", "THREADS_CLIENT_ID", "THREADS_CLIENT_SECRET", "THREADS_REDIRECT_URI", "THREADS_ACCESS_TOKEN", "THREADS_USER_ID", "POST_NEW_ENTRIES_ONLY", "SHORT_RUN", "DB_PATH"}
			for _, key := range clearEnvVars {
				_ = os.Unsetenv(key)
			}

			if tt.setupEnvFileContent != "" {
				envPath := filepath.Join(tmpDir, ".env")
				if err := os.WriteFile(envPath, []byte(tt.setupEnvFileContent), 0644); err != nil {
					t.Fatalf("Failed to write mock .env file: %v", err)
				}
			}

			for key, value := range tt.setupEnvVars {
				_ = os.Setenv(key, value)
			}

			conf, err := GetEnvVars()
			if err != nil {
				t.Fatalf("unexpected error from GetEnvVars(): %v", err)
			}

			if tt.expectError {
				if verr := ValidateRequired(conf); verr == nil {
					t.Fatal("expected validation error, got nil")
				}
				return
			}
			if verr := ValidateRequired(conf); verr != nil {
				t.Fatalf("unexpected validation error: %v", verr)
			}

			if conf.MastodonURL != tt.expectedMastodonURL {
				t.Errorf("expected MastodonURL %q, got %q", tt.expectedMastodonURL, conf.MastodonURL)
			}
			if conf.MastodonClientKey != tt.expectedMastodonClientKey {
				t.Errorf("expected MastodonClientKey %q, got %q", tt.expectedMastodonClientKey, conf.MastodonClientKey)
			}
			if conf.MastodonClientSecret != tt.expectedMastodonClientSecret {
				t.Errorf("expected MastodonClientSecret %q, got %q", tt.expectedMastodonClientSecret, conf.MastodonClientSecret)
			}
			if conf.MastodonAccessToken != tt.expectedMastodonToken {
				t.Errorf("expected MastodonAccessToken %q, got %q", tt.expectedMastodonToken, conf.MastodonAccessToken)
			}
			if conf.GotifyURL != tt.expectedGotifyURL {
				t.Errorf("expected GotifyURL %q, got %q", tt.expectedGotifyURL, conf.GotifyURL)
			}
			if conf.GotifyToken != tt.expectedGotifyToken {
				t.Errorf("expected GotifyToken %q, got %q", tt.expectedGotifyToken, conf.GotifyToken)
			}
			if conf.Debug != tt.expectedDebug {
				t.Errorf("expected Debug %v, got %v", tt.expectedDebug, conf.Debug)
			}
			if conf.FeedURL != tt.expectedFeedURL {
				t.Errorf("expected FeedURL %q, got %q", tt.expectedFeedURL, conf.FeedURL)
			}
			if conf.Interval != tt.expectedInterval {
				t.Errorf("expected Interval %d, got %d", tt.expectedInterval, conf.Interval)
			}
			if conf.BlueskyHandle != tt.expectedBlueskyHandle {
				t.Errorf("expected BlueskyHandle %q, got %q", tt.expectedBlueskyHandle, conf.BlueskyHandle)
			}
			if conf.BlueskyAppKey != tt.expectedBlueskyAppKey {
				t.Errorf("expected BlueskyAppKey %q, got %q", tt.expectedBlueskyAppKey, conf.BlueskyAppKey)
			}
			if conf.ThreadsClientID != tt.expectedThreadsClientID {
				t.Errorf("expected ThreadsClientID %q, got %q", tt.expectedThreadsClientID, conf.ThreadsClientID)
			}
			if conf.ThreadsClientSecret != tt.expectedThreadsClientSecret {
				t.Errorf("expected ThreadsClientSecret %q, got %q", tt.expectedThreadsClientSecret, conf.ThreadsClientSecret)
			}
			if conf.ThreadsRedirectURI != tt.expectedThreadsRedirectURI {
				t.Errorf("expected ThreadsRedirectURI %q, got %q", tt.expectedThreadsRedirectURI, conf.ThreadsRedirectURI)
			}
			if conf.ThreadsToken != tt.expectedThreadsToken {
				t.Errorf("expected ThreadsToken %q, got %q", tt.expectedThreadsToken, conf.ThreadsToken)
			}
		})
	}
}

func TestEnabledSites(t *testing.T) {
	tests := []struct {
		name          string
		conf          Config
		expectedSites []string
	}{
		{
			name: "All sites configured",
			conf: Config{
				MastodonURL:         "https://mastodon.example.com",
				MastodonAccessToken: "token",
				BlueskyHandle:       "user.bsky.social",
				BlueskyAppKey:       "appkey",
				ThreadsToken:        "threads-token",
				ThreadsClientID:     "threads-client-id",
				ThreadsClientSecret: "threads-client-secret",
			},
			expectedSites: []string{"mastodon", "bluesky", "threads"},
		},
		{
			name: "Only Mastodon configured",
			conf: Config{
				MastodonURL:         "https://mastodon.example.com",
				MastodonAccessToken: "token",
			},
			expectedSites: []string{"mastodon"},
		},
		{
			name: "Only Bluesky configured",
			conf: Config{
				BlueskyHandle: "user.bsky.social",
				BlueskyAppKey: "appkey",
			},
			expectedSites: []string{"bluesky"},
		},
		{
			name: "Only Threads configured",
			conf: Config{
				ThreadsToken:        "threads-token",
				ThreadsClientID:     "threads-client-id",
				ThreadsClientSecret: "threads-client-secret",
			},
			expectedSites: []string{"threads"},
		},
		{
			name:          "No sites configured",
			conf:          Config{},
			expectedSites: nil,
		},
		{
			name: "SocialSites explicitly set overrides auto-detection",
			conf: Config{
				MastodonURL:         "https://mastodon.example.com",
				MastodonAccessToken: "token",
				BlueskyHandle:       "user.bsky.social",
				BlueskyAppKey:       "appkey",
				SocialSites:         []string{"mastodon"},
			},
			expectedSites: []string{"mastodon"},
		},
		{
			name: "SocialSites explicitly set to subset",
			conf: Config{
				MastodonURL:         "https://mastodon.example.com",
				MastodonAccessToken: "token",
				BlueskyHandle:       "user.bsky.social",
				BlueskyAppKey:       "appkey",
				ThreadsToken:        "threads-token",
				ThreadsClientID:     "threads-client-id",
				ThreadsClientSecret: "threads-client-secret",
				SocialSites:         []string{"bluesky", "threads"},
			},
			expectedSites: []string{"bluesky", "threads"},
		},
		{
			name: "Mastodon missing token not auto-enabled",
			conf: Config{
				MastodonURL:   "https://mastodon.example.com",
				BlueskyHandle: "user.bsky.social",
				BlueskyAppKey: "appkey",
			},
			expectedSites: []string{"bluesky"},
		},
		{
			name: "Threads missing client ID not auto-enabled",
			conf: Config{
				ThreadsToken:        "threads-token",
				ThreadsClientSecret: "threads-client-secret",
			},
			expectedSites: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sites := tt.conf.EnabledSites()
			if len(sites) != len(tt.expectedSites) {
				t.Errorf("expected %v sites, got %v", tt.expectedSites, sites)
				return
			}
			for i, s := range sites {
				if s != tt.expectedSites[i] {
					t.Errorf("expected site %q at index %d, got %q", tt.expectedSites[i], i, s)
				}
			}
		})
	}
}
