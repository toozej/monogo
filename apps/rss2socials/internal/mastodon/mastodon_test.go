package mastodon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toozej/monogo/apps/rss2socials/internal/config"
	"github.com/toozej/monogo/apps/rss2socials/internal/rss"
)

func TestGetTootContent_WithContent(t *testing.T) {
	post := rss.RSSItem{
		Title:   "Thoughts on Go",
		Content: "Go is a great language",
		Link:    "https://example.com/thoughts",
	}
	expected := "New post: https://example.com/thoughts"
	result := GetTootContent(post)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetTootContent_WithoutContent(t *testing.T) {
	post := rss.RSSItem{
		Title: "New Blog Post",
		Link:  "https://example.com/blog",
	}
	expected := "New post: https://example.com/blog"
	result := GetTootContent(post)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestNewClient(t *testing.T) {
	conf := config.Config{
		MastodonURL:          "https://mastodon.example.com",
		MastodonClientKey:    "test-client-key",
		MastodonClientSecret: "test-client-secret",
		MastodonAccessToken:  "test-access-token",
	}

	client := NewClient(conf)
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.Config == nil {
		t.Fatal("Expected non-nil client config")
	}
	if client.Config.Server != conf.MastodonURL {
		t.Errorf("Expected server %q, got %q", conf.MastodonURL, client.Config.Server)
	}
	if client.Config.ClientID != conf.MastodonClientKey {
		t.Errorf("Expected client ID %q, got %q", conf.MastodonClientKey, client.Config.ClientID)
	}
	if client.Config.ClientSecret != conf.MastodonClientSecret {
		t.Errorf("Expected client secret %q, got %q", conf.MastodonClientSecret, client.Config.ClientSecret)
	}
	if client.Config.AccessToken != conf.MastodonAccessToken {
		t.Errorf("Expected access token %q, got %q", conf.MastodonAccessToken, client.Config.AccessToken)
	}
}

func TestTootPost_MissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		conf    config.Config
		wantErr bool
	}{
		{
			name: "Missing URL",
			conf: config.Config{
				MastodonAccessToken: "token",
			},
			wantErr: true,
		},
		{
			name: "Missing token",
			conf: config.Config{
				MastodonURL: "https://mastodon.example.com",
			},
			wantErr: true,
		},
		{
			name:    "Both missing",
			conf:    config.Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TootPost(tt.conf, "test content")
			if (err != nil) != tt.wantErr {
				t.Errorf("TootPost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTootPost(t *testing.T) {
	type mastodonResponse struct {
		ID string `json:"id"`
	}

	tests := []struct {
		name          string
		statusCode    int
		responseBody  interface{}
		expectedError bool
	}{
		{
			name:       "Success",
			statusCode: http.StatusOK,
			responseBody: mastodonResponse{
				ID: "123456",
			},
			expectedError: false,
		},
		{
			name:          "Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  map[string]string{"error": "internal server error"},
			expectedError: true,
		},
		{
			name:          "Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  map[string]string{"error": "unauthorized"},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/statuses" {
					t.Errorf("Expected path /api/v1/statuses, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got %s", r.Method)
				}
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("Expected Authorization header 'Bearer test-token', got %q", auth)
				}

				w.WriteHeader(tt.statusCode)
				if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
					t.Fatalf("failed to encode response body: %v", err)
				}
			}))
			defer mockServer.Close()

			conf := config.Config{
				MastodonURL:          mockServer.URL,
				MastodonClientKey:    "test-client-key",
				MastodonClientSecret: "test-client-secret",
				MastodonAccessToken:  "test-token",
			}

			err := TootPost(conf, "Test toot content")
			if (err != nil) != tt.expectedError {
				t.Errorf("TestTootPost(%s) failed: expected error: %v, got: %v", tt.name, tt.expectedError, err)
			}
		})
	}
}
