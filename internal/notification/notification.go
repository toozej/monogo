package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nikoksr/notify/service/discord"
	"github.com/nikoksr/notify/service/pushbullet"
	"github.com/nikoksr/notify/service/pushover"
	"github.com/nikoksr/notify/service/slack"
	"github.com/nikoksr/notify/service/telegram"
	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-find-archived-gh-actions/pkg/config"
)

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

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("gotify returned status code %d", resp.StatusCode)
	}

	return nil
}

func (n *NikoksrNotifier) AddSlack(token string, channelID string) {
	service := slack.New(token)
	service.AddReceivers(channelID)
	n.notifier.UseServices(service)
}

func (n *NikoksrNotifier) AddTelegram(token string, chatID int64) error {
	service, err := telegram.New(token)
	if err != nil {
		return err
	}
	service.AddReceivers(chatID)
	n.notifier.UseServices(service)
	return nil
}

func (n *NikoksrNotifier) AddDiscord(token string, channelID string) {
	service := discord.New()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic in discord authentication: %v", r)
		}
	}()
	_ = service.AuthenticateWithBotToken(token)
	service.AddReceivers(channelID)
	n.notifier.UseServices(service)
}

func (n *NikoksrNotifier) AddPushover(token string, recipientID string) {
	service := pushover.New(token)
	service.AddReceivers(recipientID)
	n.notifier.UseServices(service)
}

func (n *NikoksrNotifier) AddPushbullet(token string, deviceNickname string) {
	service := pushbullet.New(token)
	service.AddReceivers(deviceNickname)
	n.notifier.UseServices(service)
}

func (n *NikoksrNotifier) Notify(ctx context.Context, subject, message string) error {
	return n.notifier.Send(ctx, subject, message)
}

func NewNotificationManager(nc config.NotificationConfig) (*NotificationManager, error) {
	manager := &NotificationManager{
		condense: nc.Condense,
	}

	nikoksrNotifier := NewNikoksrNotifier()
	nikoksrAdded := false

	if nc.GotifyEndpoint != "" || nc.GotifyToken != "" {
		if nc.GotifyEndpoint == "" {
			return nil, fmt.Errorf("gotify requires GOTIFY_ENDPOINT to be set")
		}
		if nc.GotifyToken == "" {
			return nil, fmt.Errorf("gotify requires GOTIFY_TOKEN to be set")
		}
		manager.notifiers = append(manager.notifiers, NewGotifyNotifier(nc.GotifyEndpoint, nc.GotifyToken))
	}

	if nc.SlackToken != "" || nc.SlackChannelID != "" {
		if nc.SlackToken == "" {
			return nil, fmt.Errorf("slack requires SLACK_TOKEN to be set")
		}
		if nc.SlackChannelID == "" {
			return nil, fmt.Errorf("slack requires SLACK_CHANNEL_ID to be set")
		}
		nikoksrNotifier.AddSlack(nc.SlackToken, nc.SlackChannelID)
		nikoksrAdded = true
	}

	if nc.TelegramToken != "" || nc.TelegramChatID != 0 {
		if nc.TelegramToken == "" {
			return nil, fmt.Errorf("telegram requires TELEGRAM_TOKEN to be set")
		}
		if nc.TelegramChatID == 0 {
			return nil, fmt.Errorf("telegram requires TELEGRAM_CHAT_ID to be set")
		}
		if err := nikoksrNotifier.AddTelegram(nc.TelegramToken, nc.TelegramChatID); err != nil {
			return nil, fmt.Errorf("failed to add telegram: %w", err)
		}
		nikoksrAdded = true
	}

	if nc.DiscordToken != "" || nc.DiscordChannelID != "" {
		if nc.DiscordToken == "" {
			return nil, fmt.Errorf("discord requires DISCORD_TOKEN to be set")
		}
		if nc.DiscordChannelID == "" {
			return nil, fmt.Errorf("discord requires DISCORD_CHANNEL_ID to be set")
		}
		nikoksrNotifier.AddDiscord(nc.DiscordToken, nc.DiscordChannelID)
		nikoksrAdded = true
	}

	if nc.PushoverToken != "" || nc.PushoverRecipientID != "" {
		if nc.PushoverToken == "" {
			return nil, fmt.Errorf("pushover requires PUSHOVER_TOKEN to be set")
		}
		if nc.PushoverRecipientID == "" {
			return nil, fmt.Errorf("pushover requires PUSHOVER_RECIPIENT_ID to be set")
		}
		nikoksrNotifier.AddPushover(nc.PushoverToken, nc.PushoverRecipientID)
		nikoksrAdded = true
	}

	if nc.PushbulletToken != "" || nc.PushbulletDeviceNickname != "" {
		if nc.PushbulletToken == "" {
			return nil, fmt.Errorf("pushbullet requires PUSHBULLET_TOKEN to be set")
		}
		if nc.PushbulletDeviceNickname == "" {
			return nil, fmt.Errorf("pushbullet requires PUSHBULLET_DEVICE_NICKNAME to be set")
		}
		nikoksrNotifier.AddPushbullet(nc.PushbulletToken, nc.PushbulletDeviceNickname)
		nikoksrAdded = true
	}

	if nikoksrAdded {
		manager.notifiers = append(manager.notifiers, nikoksrNotifier)
	}

	return manager, nil
}

func (m *NotificationManager) NotifyArchivedActions(ctx context.Context, actions []ArchivedActionInfo, repoName string) error {
	if len(actions) == 0 {
		return nil
	}

	if m.condense {
		return m.sendCondensedNotification(ctx, actions, repoName)
	}

	var lastErr error
	for _, action := range actions {
		if err := m.notifySingleAction(ctx, action, repoName); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *NotificationManager) notifySingleAction(ctx context.Context, action ArchivedActionInfo, repoName string) error {
	subject := fmt.Sprintf("Archived GitHub Action found in %s", repoName)
	message := fmt.Sprintf("Found archived GitHub Action in repository %s:\n\n%s (used in %s)\n\nThis action should be replaced with an actively maintained alternative.",
		repoName,
		action.Uses,
		action.Workflow,
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

func (m *NotificationManager) sendCondensedNotification(ctx context.Context, actions []ArchivedActionInfo, repoName string) error {
	if len(actions) == 0 {
		return nil
	}

	var subject string
	var message strings.Builder

	if len(actions) == 1 {
		return m.notifySingleAction(ctx, actions[0], repoName)
	}

	subject = fmt.Sprintf("Archived GitHub Actions found in %s", repoName)
	message.WriteString(fmt.Sprintf("Found %d archived GitHub Actions in repository %s:\n\n", len(actions), repoName))

	for i, action := range actions {
		message.WriteString(fmt.Sprintf("%d. %s (used in %s)\n", i+1, action.Uses, action.Workflow))
	}

	message.WriteString("\nThese actions should be replaced with actively maintained alternatives.")

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
