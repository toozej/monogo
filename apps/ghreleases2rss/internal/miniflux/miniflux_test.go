package miniflux

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test GetCategoryID for success and failure cases
func TestGetCategoryID(t *testing.T) {
	// Mock server to simulate Miniflux API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/categories" {
			_, _ = fmt.Fprintln(w, `[{"id": 1, "title": "Tech"},{"id": 2, "title": "News"}]`)
		}
	}))
	defer mockServer.Close()

	apiURL := mockServer.URL
	apiKey := "dummy-api-key"

	tests := []struct {
		categoryName string
		wantID       int
		wantErr      bool
	}{
		{"Tech", 1, false},
		{"News", 2, false},
		{"NonExistent", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.categoryName, func(t *testing.T) {
			gotID, err := GetCategoryID(apiURL, apiKey, tt.categoryName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCategoryID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotID != tt.wantID {
				t.Errorf("GetCategoryID() = %v, want %v", gotID, tt.wantID)
			}
		})
	}
}

// Test SubscribeToFeed for success and failure cases
func TestSubscribeToFeed(t *testing.T) {
	called := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/feeds" {
			called = true
			var body struct {
				FeedURL    string `json:"feed_url"`
				CategoryID int    `json:"category_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if body.FeedURL != "https://github.com/username/repo/releases.atom" || body.CategoryID != 7 {
				t.Errorf("request body = %+v", body)
			}
			w.WriteHeader(http.StatusCreated) // Simulate successful feed creation
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	apiURL := mockServer.URL
	apiKey := "dummy-api-key"
	feedURL := "https://github.com/username/repo/releases.atom"

	err := SubscribeToFeed(apiURL, apiKey, 7, feedURL)
	if err != nil {
		t.Errorf("SubscribeToRSS() error = %v", err)
	}
	if !called {
		t.Fatal("expected /v1/feeds request")
	}
}

func TestResponseBodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `[{"id":1,"title":"Tech"}`)
		_, _ = fmt.Fprint(w, strings.Repeat(" ", maxResponseBytes))
		_, _ = fmt.Fprint(w, `]`)
	}))
	defer server.Close()

	if _, err := GetCategoryID(server.URL, "key", "Tech"); err == nil {
		t.Fatal("expected oversized response to fail")
	}
}

func TestRequestsUseClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	original := httpClient
	client := *httpClient
	client.Timeout = 50 * time.Millisecond
	httpClient = &client
	t.Cleanup(func() { httpClient = original })

	started := time.Now()
	if _, err := GetCategoryID(server.URL, "key", "Tech"); err == nil {
		t.Fatal("expected request timeout")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("request timeout took %s", elapsed)
	}
}

// Test DeleteFeed for success (2xx) and failure (>=400) status handling.
func TestDeleteFeed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete || r.URL.Path != "/v1/feeds/10" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		if err := DeleteFeed(server.URL, "key", 10); err != nil {
			t.Fatalf("DeleteFeed() error = %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		if err := DeleteFeed(server.URL, "key", 10); err == nil {
			t.Fatal("expected error on 500 status")
		}
	})
}

func TestRejectsInvalidEndpointAndDoesNotFollowRedirects(t *testing.T) {
	if _, err := GetCategoryID("not-a-url", "key", "Tech"); err == nil {
		t.Fatal("expected invalid endpoint error")
	}

	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected = true
		if got := r.Header.Get("X-Auth-Token"); got != "" {
			t.Errorf("redirect target received Miniflux token %q", got)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	if err := SubscribeToFeed(redirector.URL, "key", 0, "https://github.com/a/b/releases.atom"); err == nil {
		t.Fatal("expected redirect status to fail")
	}
	if redirected {
		t.Fatal("Miniflux client followed a redirect")
	}
}
