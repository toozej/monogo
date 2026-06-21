package miniflux

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test GetCategoryID for success and failure cases
func TestGetCategoryID(t *testing.T) {
	// Mock server to simulate Miniflux API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/categories" {
			fmt.Fprintln(w, `[{"id": 1, "title": "Tech"},{"id": 2, "title": "News"}]`)
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
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/feeds" {
			w.WriteHeader(http.StatusCreated) // Simulate successful feed creation
		}
	}))
	defer mockServer.Close()

	apiURL := mockServer.URL
	apiKey := "dummy-api-key"
	feedURL := "https://github.com/username/repo/releases.atom"

	err := SubscribeToFeed(apiURL, apiKey, 0, feedURL)
	if err != nil {
		t.Errorf("SubscribeToRSS() error = %v", err)
	}
}
