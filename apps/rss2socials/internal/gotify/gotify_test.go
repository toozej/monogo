package gotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/rss2socials/internal/config"
)

// Test LogFailure function with Gotify notifications
func TestLogFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conf := &config.Config{
		GotifyURL:   server.URL,
		GotifyToken: "test-token",
	}
	LogFailure("Test Error", errors.New("this is a test error"), conf)
}

func TestSendGotifyNotificationHonorsContext(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := SendGotifyNotificationContext(ctx, &config.Config{
		GotifyURL: server.URL, GotifyToken: "token",
	}, "title", "message")
	close(release)
	if err == nil {
		t.Fatal("SendGotifyNotificationContext() error = nil, want deadline error")
	}
}

// Test LogSuccess function with Gotify notifications enabled
func TestLogSuccess_Enabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conf := &config.Config{
		GotifyURL:             server.URL,
		GotifyToken:           "test-token",
		GotifyNotifyOnSuccess: true,
	}
	LogSuccess("Test success message", conf)
}

// Test LogSuccess function with Gotify notifications disabled
func TestLogSuccess_Disabled(t *testing.T) {
	conf := &config.Config{
		GotifyURL:             "https://gotify.example.com",
		GotifyToken:           "test-token",
		GotifyNotifyOnSuccess: false,
	}
	LogSuccess("Test success message", conf)
}

// Test LogSuccess with missing Gotify config
func TestLogSuccess_MissingConfig(t *testing.T) {
	conf := &config.Config{
		GotifyNotifyOnSuccess: true,
	}
	LogSuccess("Test success message", conf)
}

// Test SendGotifyNotification function for success
func TestSendGotifyNotification_Success(t *testing.T) {
	// Setup a test server to mock Gotify responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.URL.Query().Get("token") != "test-token" {
			t.Errorf("Expected token 'test-token', got %s", r.URL.Query().Get("token"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	conf := &config.Config{
		GotifyURL:   server.URL,
		GotifyToken: "test-token",
	}

	err := SendGotifyNotification(conf, "Test Title", "Test Message")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// Test 	SendGotifyNotification function for failure
func TestSendGotifyNotification_Failure(t *testing.T) {
	// Setup a test server to return an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	conf := &config.Config{
		GotifyURL:   server.URL,
		GotifyToken: "test-token",
	}

	err := SendGotifyNotification(conf, "Test Title", "Test Message")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// Test 	SendGotifyNotification with missing URL
func TestSendGotifyNotification_MissingURL(t *testing.T) {
	conf := &config.Config{
		GotifyURL:   "",
		GotifyToken: "test-token",
	}

	err := SendGotifyNotification(conf, "Test Title", "Test Message")
	if err == nil || err.Error() != "gotify URL or token is not configured" {
		t.Errorf("Expected 'gotify URL or token is not configured', got %v", err)
	}
}

// Test 	SendGotifyNotification with missing token
func TestSendGotifyNotification_MissingToken(t *testing.T) {
	conf := &config.Config{
		GotifyURL:   "https://example.com",
		GotifyToken: "",
	}

	err := SendGotifyNotification(conf, "Test Title", "Test Message")
	if err == nil || err.Error() != "gotify URL or token is not configured" {
		t.Errorf("Expected 'gotify URL or token is not configured', got %v", err)
	}
}
