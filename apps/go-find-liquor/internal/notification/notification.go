package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/pushbullet"
	"github.com/nikoksr/notify/service/pushover"
	"github.com/nikoksr/notify/service/slack"
	log "github.com/sirupsen/logrus"

	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/search"
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
	endpoint, err := url.Parse(g.endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("gotify endpoint must be an absolute HTTP(S) URL")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/message"
	query := endpoint.Query()
	query.Set("token", g.token)
	endpoint.RawQuery = query.Encode()

	payload := map[string]interface{}{
		"title":    subject,
		"message":  message,
		"priority": 5,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req) // #nosec G704 -- GotifyURL is from config, not user input
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

type DiscordNotifier struct {
	session   *discordgo.Session
	channelID string
}

func NewDiscordNotifier(token, channelID string) (*DiscordNotifier, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("discord requires bot token in credentials")
	}
	channelID = strings.TrimSpace(channelID)
	if _, err := strconv.ParseUint(channelID, 10, 64); err != nil {
		return nil, fmt.Errorf("invalid Discord channel_id: %w", err)
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("authenticate Discord bot: %w", err)
	}
	session.Client.Timeout = 10 * time.Second
	return &DiscordNotifier{session: session, channelID: channelID}, nil
}

func (d *DiscordNotifier) Notify(ctx context.Context, subject, message string) error {
	_, err := d.session.ChannelMessageSend(
		d.channelID,
		subject+"\n"+message,
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("send Discord notification: %w", err)
	}
	return nil
}

func NewTelegramNotifier(token string, chatID int64) (*TelegramNotifier, error) {
	botID, secret, ok := strings.Cut(token, ":")
	if !ok || secret == "" {
		return nil, fmt.Errorf("telegram token has invalid format")
	}
	if _, err := strconv.ParseInt(botID, 10, 64); err != nil {
		return nil, fmt.Errorf("telegram token has invalid bot ID: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	bot, err := tgbotapi.NewBotAPIWithClient(token, client)
	if err != nil {
		return nil, fmt.Errorf("authenticate Telegram bot: %w", err)
	}
	return &TelegramNotifier{bot: bot, chatID: chatID}, nil
}

func (t *TelegramNotifier) Notify(ctx context.Context, subject, message string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := t.bot.Send(tgbotapi.NewMessage(t.chatID, subject+"\n"+message))
	return err
}

// NotificationManager manages multiple notification providers
type NotificationManager struct {
	notifiers []Notifier
	condense  bool
	targets   []notificationTarget
}

type notificationTarget struct {
	notifier Notifier
	condense bool
}

// NewNotificationManager creates a notification manager from config
func NewNotificationManager(notificationConfigs []config.NotificationConfig) (*NotificationManager, error) {
	manager := &NotificationManager{}

	// Determine condense setting from first notification config (all should have same setting per user)
	if len(notificationConfigs) > 0 {
		manager.condense = notificationConfigs[0].Condense
	}

	for _, nc := range notificationConfigs {
		var notifier Notifier
		switch strings.ToLower(nc.Type) {
		case "gotify":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("gotify requires token in credentials")
			}
			if err := validateEndpoint("gotify", nc.Endpoint); err != nil {
				return nil, err
			}

			notifier = NewGotifyNotifier(nc.Endpoint, token)

		case "slack":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("slack requires token in credentials")
			}

			channelID, ok := nc.Credential["channel_id"]
			if !ok || strings.TrimSpace(channelID) == "" {
				return nil, fmt.Errorf("slack requires channel_id in credentials")
			}
			channelID = strings.TrimSpace(channelID)

			service := NewNikoksrNotifier()
			service.AddSlack(token, channelID)
			notifier = service

		case "telegram":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("telegram requires token in credentials")
			}

			chatIDStr, ok := nc.Credential["chat_id"]
			if !ok || strings.TrimSpace(chatIDStr) == "" {
				return nil, fmt.Errorf("telegram requires chat_id in credentials")
			}

			chatID, err := strconv.ParseInt(strings.TrimSpace(chatIDStr), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid telegram chat_id: %w", err)
			}

			service, err := NewTelegramNotifier(token, chatID)
			if err != nil {
				return nil, err
			}
			notifier = service

		case "discord":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("discord requires bot token in credentials")
			}

			channelID, ok := nc.Credential["channel_id"]
			if !ok || strings.TrimSpace(channelID) == "" {
				return nil, fmt.Errorf("discord requires channel_id in credentials")
			}

			service, err := NewDiscordNotifier(token, channelID)
			if err != nil {
				return nil, err
			}
			notifier = service

		case "pushover":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("pushover requires token in credentials")
			}

			recipientID, ok := nc.Credential["recipient_id"]
			if !ok || strings.TrimSpace(recipientID) == "" {
				return nil, fmt.Errorf("pushover requires recipient_id in credentials")
			}
			recipientID = strings.TrimSpace(recipientID)

			service := NewNikoksrNotifier()
			service.AddPushover(token, recipientID)
			notifier = service

		case "pushbullet":
			token, ok := nc.Credential["token"]
			if !ok || strings.TrimSpace(token) == "" {
				return nil, fmt.Errorf("pushbullet requires token in credentials")
			}

			deviceNickname, ok := nc.Credential["device_nickname"]
			if !ok || strings.TrimSpace(deviceNickname) == "" {
				return nil, fmt.Errorf("pushbullet requires device_nickname in credentials")
			}
			deviceNickname = strings.TrimSpace(deviceNickname)

			service := NewNikoksrNotifier()
			service.AddPushbullet(token, deviceNickname)
			notifier = service

		default:
			return nil, fmt.Errorf("unsupported notification type: %s", nc.Type)
		}
		manager.notifiers = append(manager.notifiers, notifier)
		manager.targets = append(manager.targets, notificationTarget{notifier: notifier, condense: nc.Condense})
	}

	return manager, nil
}

func validateEndpoint(service, value string) error {
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("%s endpoint must be an absolute HTTP(S) URL", service)
	}
	return nil
}

// NotifyFound sends notifications for found liquor items
func (m *NotificationManager) NotifyFound(ctx context.Context, item search.LiquorItem) error {
	return notifyFound(ctx, item, m.notifiers)
}

func notifyFound(ctx context.Context, item search.LiquorItem, notifiers []Notifier) error {
	subject := fmt.Sprintf("GFL - Found %s!", item.Name)
	message := fmt.Sprintf("Found %s at %s on %s at %s for %s",
		item.Name,
		item.Store,
		item.Date.Format("2006-01-02"),
		item.Date.Format("15:04:05"),
		item.Price,
	)

	log.Info(message)

	var notifyErr error
	for _, notifier := range notifiers {
		if err := notifier.Notify(ctx, subject, message); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			notifyErr = errors.Join(notifyErr, err)
		}
	}

	return notifyErr
}

// NotifyFoundItems sends notifications for multiple found liquor items
// If condense is enabled, combines all items into a single notification
// If condense is disabled, sends individual notifications for each item
func (m *NotificationManager) NotifyFoundItems(ctx context.Context, items []search.LiquorItem) error {
	if len(items) == 0 {
		return nil // No items to notify about
	}

	targets := m.targets
	if len(targets) == 0 {
		for _, notifier := range m.notifiers {
			targets = append(targets, notificationTarget{notifier: notifier, condense: m.condense})
		}
	}

	var notifyErr error
	for _, target := range targets {
		if target.condense {
			notifyErr = errors.Join(notifyErr, sendCondensedNotification(ctx, items, []Notifier{target.notifier}))
			continue
		}
		for _, item := range items {
			notifyErr = errors.Join(notifyErr, notifyFound(ctx, item, []Notifier{target.notifier}))
		}
	}
	return notifyErr
}

// sendCondensedNotification creates and sends a single notification for multiple items.
func sendCondensedNotification(ctx context.Context, items []search.LiquorItem, notifiers []Notifier) error {
	if len(items) == 0 {
		return nil
	}

	var subject string
	var message strings.Builder

	if len(items) == 1 {
		// Single item - use same format as individual notification
		item := items[0]
		subject = fmt.Sprintf("GFL - Found %s!", item.Name)
		fmt.Fprintf(&message, "Found %s at %s on %s at %s for %s",
			item.Name,
			item.Store,
			item.Date.Format("2006-01-02"),
			item.Date.Format("15:04:05"),
			item.Price,
		)
	} else {
		// Multiple items - create condensed format
		subject = fmt.Sprintf("GFL - Found %d items!", len(items))
		fmt.Fprintf(&message, "Found %d liquor items:\n\n", len(items))

		for i, item := range items {
			fmt.Fprintf(&message, "%d. %s at %s for %s\n",
				i+1,
				item.Name,
				item.Store,
				item.Price,
			)
		}

		// Add timestamp for the search
		fmt.Fprintf(&message, "\nSearch completed on %s at %s",
			items[0].Date.Format("2006-01-02"),
			items[0].Date.Format("15:04:05"),
		)
	}

	messageStr := message.String()
	log.Info(messageStr)

	var notifyErr error
	for _, notifier := range notifiers {
		if err := notifier.Notify(ctx, subject, messageStr); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			notifyErr = errors.Join(notifyErr, err)
		}
	}

	return notifyErr
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

	var notifyErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Notify(ctx, subject, message); err != nil {
			log.Errorf("Failed to send notification: %v", err)
			notifyErr = errors.Join(notifyErr, err)
		}
	}

	return notifyErr
}
