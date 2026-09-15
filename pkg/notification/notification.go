package notification

import (
	"bytes"
	"context"
	"encoding/json"
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

// Config selects a transport. Keep message templates in the calling app.
type Config struct {
	HTTPClient *http.Client      `json:"-"`
	Type       string            `json:"type"`
	Endpoint   string            `json:"endpoint"`
	Credential map[string]string `json:"credential"`
}

func New(nc Config) (Notifier, error) {
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

		gotify := NewGotifyNotifier(nc.Endpoint, token)
		if nc.HTTPClient != nil {
			gotify.client = nc.HTTPClient
		}
		notifier = gotify

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
	return notifier, nil
}

func validateEndpoint(service, value string) error {
	endpoint, err := url.Parse(value)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("%s endpoint must be an absolute HTTP(S) URL", service)
	}
	return nil
}
