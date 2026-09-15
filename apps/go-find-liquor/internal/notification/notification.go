// Package notification formats liquor alerts with shared notification transports.
package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/search"
	shared "github.com/toozej/monogo/pkg/notification"
)

type Notifier = shared.Notifier
type GotifyNotifier = shared.GotifyNotifier
type NikoksrNotifier = shared.NikoksrNotifier
type TelegramNotifier = shared.TelegramNotifier
type DiscordNotifier = shared.DiscordNotifier

var NewGotifyNotifier = shared.NewGotifyNotifier
var NewNikoksrNotifier = shared.NewNikoksrNotifier
var NewTelegramNotifier = shared.NewTelegramNotifier
var NewDiscordNotifier = shared.NewDiscordNotifier

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
		notifier, err := shared.New(shared.Config{Type: nc.Type, Endpoint: nc.Endpoint, Credential: nc.Credential})
		if err != nil {
			return nil, err
		}
		manager.notifiers = append(manager.notifiers, notifier)
		manager.targets = append(manager.targets, notificationTarget{notifier: notifier, condense: nc.Condense})
	}

	return manager, nil
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
