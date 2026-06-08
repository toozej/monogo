package notification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGotifyNotifier_Notify_RequestError(t *testing.T) {
	notifier := NewGotifyNotifier("http://127.0.0.1:0", "token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := notifier.Notify(ctx, "subject", "message")
	if err == nil {
		t.Error("Expected error from Notify due to bad endpoint")
	}
}

func TestGotifyNotifier_Notify_RequestCreationError(t *testing.T) {
	// newline character in url causes NewRequestWithContext to fail
	notifier := NewGotifyNotifier("http://example.com/api\n/message", "token")
	err := notifier.Notify(context.Background(), "subject", "message")
	if err == nil {
		t.Error("Expected error from Notify due to invalid URL")
	}
}

func TestNikoksrNotifier_Notify(t *testing.T) {
	n := NewNikoksrNotifier()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := n.Notify(ctx, "test", "test message")
	// If no receivers/services, nikoksr might return nil or error
	_ = err
}

// ErrorMockNotifier is a notifier that always returns an error
type ErrorMockNotifier struct{}

func (e *ErrorMockNotifier) Notify(ctx context.Context, subject, message string) error {
	return errors.New("mock error")
}

func TestNotificationManager_NotifyArchivedActions_Errors(t *testing.T) {
	manager := &NotificationManager{
		notifiers: []Notifier{&ErrorMockNotifier{}},
		condense:  false,
	}

	actions := []ArchivedActionInfo{
		{Uses: "actions/checkout@v3", Workflow: "ci.yml"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := manager.NotifyArchivedActions(ctx, actions, "test/repo")
	if err == nil {
		t.Error("Expected error from NotifyArchivedActions")
	}

	// Also test condensed error
	manager.condense = true
	err = manager.NotifyArchivedActions(ctx, actions, "test/repo")
	if err == nil {
		t.Error("Expected error from NotifyArchivedActions condensed")
	}

	// Test empty list
	err = manager.NotifyArchivedActions(ctx, []ArchivedActionInfo{}, "test/repo")
	if err != nil {
		t.Errorf("Expected no error for empty actions, got %v", err)
	}
}

func TestAddDiscordPanicRecover(t *testing.T) {
	// To cover the defer recover branch in AddDiscord
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AddDiscord panicked instead of recovering: %v", r)
		}
	}()
	n := NewNikoksrNotifier()
	// An empty token string or nil triggers panic in some contexts in older discordgo versions
	n.AddDiscord("", "123")
}
