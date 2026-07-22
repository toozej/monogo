package notification

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
)

// GotifyErrorHook forwards error-level application logs to every distinct
// configured Gotify target. It deliberately writes delivery failures to stderr
// rather than Logrus, preventing an unavailable Gotify server from recursively
// generating more error notifications.
type GotifyErrorHook struct {
	notifiers []*GotifyNotifier
}

// NewGotifyErrorHook builds an error hook from the users' existing notification
// configuration. Non-Gotify notification types are intentionally ignored.
func NewGotifyErrorHook(users []config.UserConfig) *GotifyErrorHook {
	hook := &GotifyErrorHook{}
	seen := make(map[string]struct{})
	for _, user := range users {
		for _, target := range user.Notifications {
			if !strings.EqualFold(target.Type, "gotify") {
				continue
			}
			token := strings.TrimSpace(target.Credential["token"])
			endpoint := strings.TrimSpace(target.Endpoint)
			if endpoint == "" || token == "" {
				continue
			}
			key := endpoint + "\x00" + token
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			hook.notifiers = append(hook.notifiers, NewGotifyNotifier(endpoint, token))
		}
	}
	return hook
}

func (h *GotifyErrorHook) Levels() []log.Level {
	return []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel}
}

func (h *GotifyErrorHook) Fire(entry *log.Entry) error {
	if len(h.notifiers) == 0 {
		return nil
	}
	message := entry.Message
	if err, ok := entry.Data[log.ErrorKey].(error); ok && err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, notifier := range h.notifiers {
		if err := notifier.Notify(ctx, "GFL - Error", message); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "send Gotify error notification: %v\n", err)
		}
	}
	return nil
}
