package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/pkg/config"
)

// MockNotifier implements the Notifier interface for testing
type MockNotifier struct {
	notifications []NotificationCall
}

type NotificationCall struct {
	Subject string
	Message string
}

func (m *MockNotifier) Notify(ctx context.Context, subject, message string) error {
	m.notifications = append(m.notifications, NotificationCall{
		Subject: subject,
		Message: message,
	})
	return nil
}

func (m *MockNotifier) GetNotifications() []NotificationCall {
	return m.notifications
}

func (m *MockNotifier) Reset() {
	m.notifications = nil
}

// createTestNotificationManager creates a notification manager with mock notifiers for testing
func createTestNotificationManager(condense bool) (*NotificationManager, *MockNotifier) {
	mockNotifier := &MockNotifier{}
	manager := &NotificationManager{
		notifiers: []Notifier{mockNotifier},
		condense:  condense,
	}
	return manager, mockNotifier
}

func TestNotificationManager_NotifyArchivedActions_EmptyList(t *testing.T) {
	manager, mockNotifier := createTestNotificationManager(true)

	err := manager.NotifyArchivedActions(context.Background(), []ArchivedActionInfo{}, "test/repo")

	if err != nil {
		t.Errorf("Expected no error for empty list, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 0 {
		t.Errorf("Expected no notifications for empty list, got %d", len(notifications))
	}
}

func TestNotificationManager_NotifyArchivedActions_SingleItem(t *testing.T) {
	action := ArchivedActionInfo{
		Repo:     "actions/checkout",
		Workflow: "ci.yml",
		Uses:     "actions/checkout@v3",
	}

	testCases := []struct {
		name     string
		condense bool
	}{
		{"condense enabled", true},
		{"condense disabled", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, mockNotifier := createTestNotificationManager(tc.condense)

			err := manager.NotifyArchivedActions(context.Background(), []ArchivedActionInfo{action}, "test/repo")

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			notifications := mockNotifier.GetNotifications()
			if len(notifications) != 1 {
				t.Errorf("Expected 1 notification, got %d", len(notifications))
				return
			}

			notification := notifications[0]
			expectedSubject := "Archived GitHub Action found in test/repo"
			if notification.Subject != expectedSubject {
				t.Errorf("Expected subject '%s', got '%s'", expectedSubject, notification.Subject)
			}

			expectedMessage := "Found archived GitHub Action in repository test/repo:\n\nactions/checkout@v3 (used in ci.yml)\n\nThis action should be replaced with an actively maintained alternative."
			if notification.Message != expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", expectedMessage, notification.Message)
			}
		})
	}
}

func TestNotificationManager_NotifyArchivedActions_MultipleItems_Individual(t *testing.T) {
	actions := []ArchivedActionInfo{
		{
			Repo:     "actions/checkout",
			Workflow: "ci.yml",
			Uses:     "actions/checkout@v3",
		},
		{
			Repo:     "actions/setup-go",
			Workflow: "ci.yml",
			Uses:     "actions/setup-go@v4",
		},
	}

	manager, mockNotifier := createTestNotificationManager(false) // condense disabled

	err := manager.NotifyArchivedActions(context.Background(), actions, "test/repo")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 2 {
		t.Errorf("Expected 2 individual notifications, got %d", len(notifications))
		return
	}

	if notifications[0].Subject != "Archived GitHub Action found in test/repo" {
		t.Errorf("Expected subject 'Archived GitHub Action found in test/repo', got '%s'", notifications[0].Subject)
	}
	if !strings.Contains(notifications[0].Message, "actions/checkout@v3") {
		t.Errorf("Expected message to contain actions/checkout@v3")
	}

	if notifications[1].Subject != "Archived GitHub Action found in test/repo" {
		t.Errorf("Expected subject 'Archived GitHub Action found in test/repo', got '%s'", notifications[1].Subject)
	}
	if !strings.Contains(notifications[1].Message, "actions/setup-go@v4") {
		t.Errorf("Expected message to contain actions/setup-go@v4")
	}
}

func TestNotificationManager_NotifyArchivedActions_MultipleItems_Condensed(t *testing.T) {
	actions := []ArchivedActionInfo{
		{
			Repo:     "actions/checkout",
			Workflow: "ci.yml",
			Uses:     "actions/checkout@v3",
		},
		{
			Repo:     "actions/setup-go",
			Workflow: "test.yml",
			Uses:     "actions/setup-go@v4",
		},
	}

	manager, mockNotifier := createTestNotificationManager(true) // condense enabled

	err := manager.NotifyArchivedActions(context.Background(), actions, "test/repo")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 condensed notification, got %d", len(notifications))
		return
	}

	notification := notifications[0]
	expectedSubject := "Archived GitHub Actions found in test/repo"
	if notification.Subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, notification.Subject)
	}

	message := notification.Message
	if !strings.Contains(message, "Found 2 archived GitHub Actions in repository test/repo:") {
		t.Errorf("Expected message to contain intro, got: %s", message)
	}

	if !strings.Contains(message, "1. actions/checkout@v3 (used in ci.yml)") {
		t.Errorf("Expected message to contain first item details, got: %s", message)
	}

	if !strings.Contains(message, "2. actions/setup-go@v4 (used in test.yml)") {
		t.Errorf("Expected message to contain second item details, got: %s", message)
	}

	if !strings.Contains(message, "These actions should be replaced") {
		t.Errorf("Expected message to contain conclusion, got: %s", message)
	}
}

func TestNewNotificationManager(t *testing.T) {
	testCases := []struct {
		name          string
		config        config.NotificationConfig
		expectError   bool
		errorContains string
	}{
		{
			name:        "empty config — no providers enabled",
			config:      config.NotificationConfig{},
			expectError: false,
		},
		{
			name:        "gotify success",
			config:      config.NotificationConfig{GotifyEndpoint: "http://example.com", GotifyToken: "xyz"},
			expectError: false,
		},
		{
			name:          "gotify missing token",
			config:        config.NotificationConfig{GotifyEndpoint: "http://example.com"},
			expectError:   true,
			errorContains: "gotify requires GOTIFY_TOKEN to be set",
		},
		{
			name:          "gotify missing endpoint",
			config:        config.NotificationConfig{GotifyToken: "xyz"},
			expectError:   true,
			errorContains: "gotify requires GOTIFY_ENDPOINT to be set",
		},
		{
			name:        "slack success",
			config:      config.NotificationConfig{SlackToken: "xyz", SlackChannelID: "C123"},
			expectError: false,
		},
		{
			name:          "slack missing token",
			config:        config.NotificationConfig{SlackChannelID: "C123"},
			expectError:   true,
			errorContains: "slack requires SLACK_TOKEN to be set",
		},
		{
			name:          "slack missing channel_id",
			config:        config.NotificationConfig{SlackToken: "xyz"},
			expectError:   true,
			errorContains: "slack requires SLACK_CHANNEL_ID to be set",
		},
		{
			name:        "telegram success",
			config:      config.NotificationConfig{TelegramToken: "xyz", TelegramChatID: 123},
			expectError: false,
		},
		{
			name:          "telegram api error",
			config:        config.NotificationConfig{TelegramToken: "mock_error", TelegramChatID: 123},
			expectError:   true,
			errorContains: "failed to add telegram",
		},
		{
			name:          "telegram missing token",
			config:        config.NotificationConfig{TelegramChatID: 123},
			expectError:   true,
			errorContains: "telegram requires TELEGRAM_TOKEN to be set",
		},
		{
			name:          "telegram missing chat_id",
			config:        config.NotificationConfig{TelegramToken: "xyz"},
			expectError:   true,
			errorContains: "telegram requires TELEGRAM_CHAT_ID to be set",
		},
		{
			name:        "discord success",
			config:      config.NotificationConfig{DiscordToken: "xyz", DiscordChannelID: "123"},
			expectError: false,
		},
		{
			name:          "discord missing token",
			config:        config.NotificationConfig{DiscordChannelID: "123"},
			expectError:   true,
			errorContains: "discord requires DISCORD_TOKEN to be set",
		},
		{
			name:          "discord missing channel_id",
			config:        config.NotificationConfig{DiscordToken: "xyz"},
			expectError:   true,
			errorContains: "discord requires DISCORD_CHANNEL_ID to be set",
		},
		{
			name:        "pushover success",
			config:      config.NotificationConfig{PushoverToken: "xyz", PushoverRecipientID: "abc"},
			expectError: false,
		},
		{
			name:          "pushover missing token",
			config:        config.NotificationConfig{PushoverRecipientID: "abc"},
			expectError:   true,
			errorContains: "pushover requires PUSHOVER_TOKEN to be set",
		},
		{
			name:          "pushover missing recipient_id",
			config:        config.NotificationConfig{PushoverToken: "xyz"},
			expectError:   true,
			errorContains: "pushover requires PUSHOVER_RECIPIENT_ID to be set",
		},
		{
			name:        "pushbullet success",
			config:      config.NotificationConfig{PushbulletToken: "xyz", PushbulletDeviceNickname: "myphone"},
			expectError: false,
		},
		{
			name:          "pushbullet missing token",
			config:        config.NotificationConfig{PushbulletDeviceNickname: "myphone"},
			expectError:   true,
			errorContains: "pushbullet requires PUSHBULLET_TOKEN to be set",
		},
		{
			name:          "pushbullet missing device_nickname",
			config:        config.NotificationConfig{PushbulletToken: "xyz"},
			expectError:   true,
			errorContains: "pushbullet requires PUSHBULLET_DEVICE_NICKNAME to be set",
		},
		{
			name: "condense flag propagated",
			config: config.NotificationConfig{
				GotifyEndpoint: "http://example.com",
				GotifyToken:    "xyz",
				Condense:       true,
			},
			expectError: false,
		},
	}

	// Save original transport and restore later
	originalTransport := http.DefaultTransport
	defer func() {
		http.DefaultTransport = originalTransport
	}()
	mockTelegramTransport()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewNotificationManager(tc.config)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if manager != nil && tc.config.Condense != manager.condense {
					t.Errorf("Expected condense=%v, got %v", tc.config.Condense, manager.condense)
				}
			}
		})
	}
}

func TestGotifyNotifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
			return
		}

		if payload["title"] != "Test Subject" {
			t.Errorf("Expected subject 'Test Subject', got '%v'", payload["title"])
		}
		if payload["message"] != "Test Message" {
			t.Errorf("Expected message 'Test Message', got '%v'", payload["message"])
		}

		w.WriteHeader(200)
	}))
	defer server.Close()

	notifier := NewGotifyNotifier(server.URL, "test-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := notifier.Notify(ctx, "Test Subject", "Test Message")
	if err != nil {
		t.Errorf("Notify failed: %v", err)
	}
}

func TestGotifyNotifier_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	notifier := NewGotifyNotifier(server.URL, "test-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := notifier.Notify(ctx, "Test Subject", "Test Message")
	if err == nil {
		t.Errorf("Expected error due to 500 status code, got nil")
	} else if !strings.Contains(err.Error(), "status code 500") {
		t.Errorf("Expected status code error, got: %v", err)
	}
}

func TestGotifyNotifier_BadURL(t *testing.T) {
	notifier := NewGotifyNotifier("http://invalid-url-\x00.com", "test-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := notifier.Notify(ctx, "Test Subject", "Test Message")
	if err == nil {
		t.Errorf("Expected error due to bad URL, got nil")
	}
}
