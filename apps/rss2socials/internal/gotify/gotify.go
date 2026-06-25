// Package gotify provides functionality for sending error notifications to Gotify instances.
// It includes utilities for logging failures and sending HTTP requests to the Gotify API.
package gotify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
)

// LogFailure logs the error and sends a notification to the Gotify instance.
func LogFailure(message string, err error, conf *config.Config) {
	log.Printf("%s: %s", message, err)
	if conf.GotifyURL != "" && conf.GotifyToken != "" {
		if err := SendGotifyNotification(conf, message, err.Error()); err != nil {
			log.Printf("Error sending Gotify notification: %s", err)
		}
	}
}

// LogSuccess logs a success message and sends a notification to the Gotify instance
// when GotifyNotifyOnSuccess is enabled.
func LogSuccess(message string, conf *config.Config) {
	log.Info(message)
	if conf.GotifyNotifyOnSuccess && conf.GotifyURL != "" && conf.GotifyToken != "" {
		if err := SendGotifyNotification(conf, "rss2socials success", message); err != nil {
			log.Printf("Error sending Gotify success notification: %s", err)
		}
	}
}

// SendGotifyNotification sends a notification to Gotify.
func SendGotifyNotification(conf *config.Config, title, message string) error {
	// sendGotifyNotification sends a notification to the Gotify instance using the provided configuration.
	// It marshals the notification data as JSON and posts it via HTTP.
	if conf.GotifyURL == "" || conf.GotifyToken == "" {
		return errors.New("gotify URL or token is not configured")
	}

	notification := map[string]interface{}{
		"title":    title,
		"message":  message,
		"priority": 5,
	}

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal Gotify notification: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/message?token=%s", conf.GotifyURL, conf.GotifyToken), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create Gotify request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- GotifyURL is from config, not user input
	if err != nil {
		return fmt.Errorf("failed to send Gotify request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gotify returned non-OK status: %s", resp.Status)
	}

	return nil
}
