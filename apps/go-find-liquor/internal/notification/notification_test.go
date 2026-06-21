package notification

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-find-liquor/internal/search"
	"github.com/toozej/go-find-liquor/pkg/config"
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

func TestNotificationManager_NotifyFoundItems_EmptyList(t *testing.T) {
	// Test with both condense settings
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

			err := manager.NotifyFoundItems(context.Background(), []search.LiquorItem{})

			if err != nil {
				t.Errorf("Expected no error for empty list, got: %v", err)
			}

			notifications := mockNotifier.GetNotifications()
			if len(notifications) != 0 {
				t.Errorf("Expected no notifications for empty list, got %d", len(notifications))
			}
		})
	}
}

func TestNotificationManager_NotifyFoundItems_SingleItem(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	item := search.LiquorItem{
		Name:  "Blanton's",
		Code:  "12345",
		Store: "Test Store",
		Date:  testTime,
		Price: "$59.99",
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

			err := manager.NotifyFoundItems(context.Background(), []search.LiquorItem{item})

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			notifications := mockNotifier.GetNotifications()
			if len(notifications) != 1 {
				t.Errorf("Expected 1 notification, got %d", len(notifications))
				return
			}

			notification := notifications[0]
			expectedSubject := "GFL - Found Blanton's!"
			if notification.Subject != expectedSubject {
				t.Errorf("Expected subject '%s', got '%s'", expectedSubject, notification.Subject)
			}

			// Both condense modes should produce the same result for single item
			expectedMessage := "Found Blanton's at Test Store on 2024-01-15 at 14:30:00 for $59.99"
			if notification.Message != expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", expectedMessage, notification.Message)
			}
		})
	}
}

func TestNotificationManager_NotifyFoundItems_MultipleItems_Individual(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	items := []search.LiquorItem{
		{
			Name:  "Blanton's",
			Code:  "12345",
			Store: "Store A",
			Date:  testTime,
			Price: "$59.99",
		},
		{
			Name:  "W.L. Weller Special Reserve",
			Code:  "67890",
			Store: "Store B",
			Date:  testTime,
			Price: "$29.99",
		},
	}

	manager, mockNotifier := createTestNotificationManager(false) // condense disabled

	err := manager.NotifyFoundItems(context.Background(), items)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 2 {
		t.Errorf("Expected 2 individual notifications, got %d", len(notifications))
		return
	}

	// Check first notification
	if notifications[0].Subject != "GFL - Found Blanton's!" {
		t.Errorf("Expected first subject 'GFL - Found Blanton's!', got '%s'", notifications[0].Subject)
	}

	// Check second notification
	if notifications[1].Subject != "GFL - Found W.L. Weller Special Reserve!" {
		t.Errorf("Expected second subject 'GFL - Found W.L. Weller Special Reserve!', got '%s'", notifications[1].Subject)
	}
}

func TestNotificationManager_NotifyFoundItems_MultipleItems_Condensed(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	items := []search.LiquorItem{
		{
			Name:  "Blanton's",
			Code:  "12345",
			Store: "Store A",
			Date:  testTime,
			Price: "$59.99",
		},
		{
			Name:  "W.L. Weller Special Reserve",
			Code:  "67890",
			Store: "Store B",
			Date:  testTime,
			Price: "$29.99",
		},
		{
			Name:  "Eagle Rare",
			Code:  "11111",
			Store: "Store C",
			Date:  testTime,
			Price: "$39.99",
		},
	}

	manager, mockNotifier := createTestNotificationManager(true) // condense enabled

	err := manager.NotifyFoundItems(context.Background(), items)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 condensed notification, got %d", len(notifications))
		return
	}

	notification := notifications[0]
	expectedSubject := "GFL - Found 3 items!"
	if notification.Subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, notification.Subject)
	}

	// Check that message contains all items
	message := notification.Message
	if !strings.Contains(message, "Found 3 liquor items:") {
		t.Errorf("Expected message to contain 'Found 3 liquor items:', got: %s", message)
	}

	if !strings.Contains(message, "1. Blanton's at Store A for $59.99") {
		t.Errorf("Expected message to contain first item details, got: %s", message)
	}

	if !strings.Contains(message, "2. W.L. Weller Special Reserve at Store B for $29.99") {
		t.Errorf("Expected message to contain second item details, got: %s", message)
	}

	if !strings.Contains(message, "3. Eagle Rare at Store C for $39.99") {
		t.Errorf("Expected message to contain third item details, got: %s", message)
	}

	if !strings.Contains(message, "Search completed on 2024-01-15 at 14:30:00") {
		t.Errorf("Expected message to contain timestamp, got: %s", message)
	}
}

func TestNewNotificationManager_CondenseField(t *testing.T) {
	testCases := []struct {
		name             string
		configs          []config.NotificationConfig
		expectedCondense bool
	}{
		{
			name:             "empty config",
			configs:          []config.NotificationConfig{},
			expectedCondense: false,
		},
		{
			name: "condense enabled",
			configs: []config.NotificationConfig{
				{
					Type:     "gotify",
					Endpoint: "http://example.com",
					Condense: true,
					Credential: map[string]string{
						"token": "test-token",
					},
				},
			},
			expectedCondense: true,
		},
		{
			name: "condense disabled",
			configs: []config.NotificationConfig{
				{
					Type:     "gotify",
					Endpoint: "http://example.com",
					Condense: false,
					Credential: map[string]string{
						"token": "test-token",
					},
				},
			},
			expectedCondense: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewNotificationManager(tc.configs)
			if err != nil {
				t.Errorf("Expected no error creating notification manager, got: %v", err)
				return
			}

			if manager.condense != tc.expectedCondense {
				t.Errorf("Expected condense to be %v, got %v", tc.expectedCondense, manager.condense)
			}
		})
	}
}

func TestNotificationManager_NotifyHeartbeat_NoHealthCheck(t *testing.T) {
	manager, mockNotifier := createTestNotificationManager(false)

	err := manager.NotifyHeartbeat(context.Background(), "", false)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	expectedSubject := "GFL - Heartbeat"
	if notifications[0].Subject != expectedSubject {
		t.Errorf("Expected subject '%s', got '%s'", expectedSubject, notifications[0].Subject)
	}

	expectedMessage := "GFL is still running and searching"
	if notifications[0].Message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, notifications[0].Message)
	}
}

func TestNotificationManager_NotifyHeartbeat_HealthCheckFound(t *testing.T) {
	manager, mockNotifier := createTestNotificationManager(false)

	err := manager.NotifyHeartbeat(context.Background(), "TITO'S HANDMADE VODKA", true)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if !strings.Contains(notifications[0].Message, "TITO'S HANDMADE VODKA") {
		t.Errorf("Expected message to contain health check item, got: %s", notifications[0].Message)
	}

	if !strings.Contains(notifications[0].Message, "found it in stock") {
		t.Errorf("Expected message to indicate item found, got: %s", notifications[0].Message)
	}
}

func TestNotificationManager_NotifyHeartbeat_HealthCheckNotFound(t *testing.T) {
	manager, mockNotifier := createTestNotificationManager(false)

	err := manager.NotifyHeartbeat(context.Background(), "JACK DANIEL'S OLD NO 7", false)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	notifications := mockNotifier.GetNotifications()
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifications))
	}

	if !strings.Contains(notifications[0].Message, "JACK DANIEL'S OLD NO 7") {
		t.Errorf("Expected message to contain health check item, got: %s", notifications[0].Message)
	}

	if !strings.Contains(notifications[0].Message, "not found") {
		t.Errorf("Expected message to indicate item not found, got: %s", notifications[0].Message)
	}
}
