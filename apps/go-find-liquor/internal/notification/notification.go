package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/discord"
	"github.com/nikoksr/notify/service/pushbullet"
	"github.com/nikoksr/notify/service/pushover"
	"github.com/nikoksr/notify/service/slack"
	"github.com/nikoksr/notify/service/telegram"
	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-find-liquor/internal/search"
	"github.com/toozej/go-find-liquor/pkg/config"
)

// Notifier is an interface for sending notifications
type Notifier interface {
	Notify(ctx context.Context, subject, message string) error
}

// GotifyNotifier implements direct Gotify API integration
type GotifyNotifier struct {
	endpoint string
	token    string
	client   *http.Client
}

// NewGotifyNotifier creates a new Gotify notifier
func NewGotifyNotifier(endpoint, token string) *GotifyNotifier {
	return &GotifyNotifier{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		token:    token,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends a notification to Gotify
func (g *GotifyNotifier) Notify(ctx context.Context, subject, message string) error {
	url := fmt.Sprintf("%s/message?token=%s", g.endpoint, g.token)

	payload := map[string]interface{}{
		"title":    subject,
		"message":  message,
		"priority": 5,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req) // #nosec G704 -- GotifyURL is from config, not user input
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("gotify returned status code %d", resp.StatusCode)
	}

	return nil
}

// NikoksrNotifier uses the nikoksr/notify library for other notification services
type NikoksrNotifier struct {
	notifier *notify.Notify
}

// NewNikoksrNotifier creates a new notifier using nikoksr/notify
func NewNikoksrNotifier() *NikoksrNotifier {
	return &NikoksrNotifier{
		notifier: notify.New(),
	}
}

// AddSlack adds Slack notification service
func (n *NikoksrNotifier) AddSlack(token string, channelID string) {
	service := slack.New(token)
	service.AddReceivers(channelID)
	n.notifier.UseServices(service)
}

// AddTelegram adds Telegram notification service
func (n *NikoksrNotifier) AddTelegram(token string, chatID int64) {
	service, _ := telegram.New(token)
	service.AddReceivers(chatID)
	n.notifier.UseServices(service)
}

// AddDiscord adds Discord notification service
func (n *NikoksrNotifier) AddDiscord(token string, channelID string) {
	service := discord.New()
	_ = service.AuthenticateWithBotToken(token)
	service.AddReceivers(channelID)
	n.notifier.UseServices(service)
}

// AddPushover adds Pushover notification service
func (n *NikoksrNotifier) AddPushover(token string, recipientID string) {
	service := pushover.New(token)
	service.AddReceivers(recipientID)
	n.notifier.UseServices(service)
}

// AddPushover adds Pushover notification service
func (n *NikoksrNotifier) AddPushbullet(token string, deviceNickname string) {
	service := pushbullet.New(token)
	service.AddReceivers(deviceNickname)
	n.notifier.UseServices(service)
}

// Notify sends a notification using nikoksr/notify
func (n *NikoksrNotifier) Notify(ctx context.Context, subject, message string) error {
	return n.notifier.Send(ctx, subject, message)
}

// NotificationManager manages multiple notification providers
type NotificationManager struct {
	notifiers []Notifier
	condense  bool
}

// NewNotificationManager creates a notification manager from config
func NewNotificationManager(notificationConfigs []config.NotificationConfig) (*NotificationManager, error) {
	manager := &NotificationManager{}

	// Determine condense setting from first notification config (all should have same setting per user)
	if len(notificationConfigs) > 0 {
		manager.condense = notificationConfigs[0].Condense
	}

	// Add nicoksr notify for handling multiple services
	nikoksrNotifier := NewNikoksrNotifier()
	nikoksrAdded := false

	for _, nc := range notificationConfigs {
		switch strings.ToLower(nc.Type) {
		case "gotify":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("gotify requires token in credentials")
			}

			gotify := NewGotifyNotifier(nc.Endpoint, token)
			manager.notifiers = append(manager.notifiers, gotify)

		case "slack":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("slack requires token in credentials")
			}

			channelIDStr, ok := nc.Credential["channel_id"]
			if !ok {
				return nil, fmt.Errorf("slack requires channel_id in credentials")
			}

			var channelID string
			_, err := fmt.Sscanf(channelIDStr, "%s", &channelID)
			if err != nil {
				return nil, fmt.Errorf("invalid Slack channel_id: %w", err)
			}

			nikoksrNotifier.AddSlack(token, channelID)
			nikoksrAdded = true

		case "telegram":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("telegram requires token in credentials")
			}

			chatIDStr, ok := nc.Credential["chat_id"]
			if !ok {
				return nil, fmt.Errorf("telegram requires chat_id in credentials")
			}

			var chatID int64
			_, err := fmt.Sscanf(chatIDStr, "%d", &chatID)
			if err != nil {
				return nil, fmt.Errorf("invalid telegram chat_id: %w", err)
			}

			nikoksrNotifier.AddTelegram(token, chatID)
			nikoksrAdded = true

		case "discord":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("discord requires bot token in credentials")
			}

			channelIDStr, ok := nc.Credential["channel_id"]
			if !ok {
				return nil, fmt.Errorf("discord requires channel_id in credentials")
			}

			var channelID string
			_, err := fmt.Sscanf(channelIDStr, "%s", &channelID)
			if err != nil {
				return nil, fmt.Errorf("invalid Slack channel_id: %w", err)
			}

			nikoksrNotifier.AddDiscord(token, channelID)
			nikoksrAdded = true

		case "pushover":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("pushover requires token in credentials")
			}

			recipientID, ok := nc.Credential["recipient_id"]
			if !ok {
				return nil, fmt.Errorf("pushover requires recipient_id in credentials")
			}

			nikoksrNotifier.AddPushover(token, recipientID)
			nikoksrAdded = true

		case "pushbullet":
			token, ok := nc.Credential["token"]
			if !ok {
				return nil, fmt.Errorf("pushbullet requires token in credentials")
			}

			deviceNickname, ok := nc.Credential["device_nickname"]
			if !ok {
				return nil, fmt.Errorf("pushbullet requires device_nickname in credentials")
			}

			nikoksrNotifier.AddPushbullet(token, deviceNickname)
			nikoksrAdded = true

		default:
			return nil, fmt.Errorf("unsupported notification type: %s", nc.Type)
		}
	}

	// Add nikoksr notifier if any services were added to it
	if nikoksrAdded {
		manager.notifiers = append(manager.notifiers, nikoksrNotifier)
	}

	return manager, nil
}

// NotifyFound sends notifications for found liquor items
func (m *NotificationManager) NotifyFound(ctx context.Context, item search.LiquorItem) error {
	subject := fmt.Sprintf("GFL - Found %s!", item.Name)
	message := fmt.Sprintf("Found %s at %s on %s at %s for %s",
		item.Name,
		item.Store,
		item.Date.Format("2006-01-02"),
		item.Date.Format("15:04:05"),
		item.Price,
	)

	log.Info(message)

	var lastErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(ctx, subject, message); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			lastErr = err
		}
	}

	return lastErr
}

// NotifyFoundItems sends notifications for multiple found liquor items
// If condense is enabled, combines all items into a single notification
// If condense is disabled, sends individual notifications for each item
func (m *NotificationManager) NotifyFoundItems(ctx context.Context, items []search.LiquorItem) error {
	if len(items) == 0 {
		return nil // No items to notify about
	}

	if m.condense {
		return m.sendCondensedNotification(ctx, items)
	}

	// Send individual notifications
	var lastErr error
	for _, item := range items {
		if err := m.NotifyFound(ctx, item); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// sendCondensedNotification creates and sends a single notification for multiple items
func (m *NotificationManager) sendCondensedNotification(ctx context.Context, items []search.LiquorItem) error {
	if len(items) == 0 {
		return nil
	}

	var subject string
	var message strings.Builder

	if len(items) == 1 {
		// Single item - use same format as individual notification
		item := items[0]
		subject = fmt.Sprintf("GFL - Found %s!", item.Name)
		message.WriteString(fmt.Sprintf("Found %s at %s on %s at %s for %s",
			item.Name,
			item.Store,
			item.Date.Format("2006-01-02"),
			item.Date.Format("15:04:05"),
			item.Price,
		))
	} else {
		// Multiple items - create condensed format
		subject = fmt.Sprintf("GFL - Found %d items!", len(items))
		message.WriteString(fmt.Sprintf("Found %d liquor items:\n\n", len(items)))

		for i, item := range items {
			message.WriteString(fmt.Sprintf("%d. %s at %s for %s\n",
				i+1,
				item.Name,
				item.Store,
				item.Price,
			))
		}

		// Add timestamp for the search
		message.WriteString(fmt.Sprintf("\nSearch completed on %s at %s",
			items[0].Date.Format("2006-01-02"),
			items[0].Date.Format("15:04:05"),
		))
	}

	messageStr := message.String()
	log.Info(messageStr)

	var lastErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(ctx, subject, messageStr); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			lastErr = err
		}
	}

	return lastErr
}

// NotifyHeartbeat sends notifications for nothing found but still trying.
// If healthCheckItem is non-empty, it indicates a random common item was searched
// as a health check, and healthCheckFound indicates whether it was found in stock.
func (m *NotificationManager) NotifyHeartbeat(ctx context.Context, healthCheckItem string, healthCheckFound bool) error {
	subject := "GFL - Heartbeat"
	message := "GFL is still running and searching"

	if healthCheckItem != "" {
		if healthCheckFound {
			message = fmt.Sprintf("%s. Health check: searched for '%s' and found it in stock", message, healthCheckItem)
		} else {
			message = fmt.Sprintf("%s. Health check: searched for '%s' but it was not found", message, healthCheckItem)
		}
	}

	log.Info(message)

	var lastErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(ctx, subject, message); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			lastErr = err
		}
	}

	return lastErr
}
