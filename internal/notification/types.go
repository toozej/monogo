package notification

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nikoksr/notify"
)

type ArchivedActionInfo struct {
	Repo     string `json:"repo"`
	Workflow string `json:"workflow"`
	Uses     string `json:"uses"`
}

type Notifier interface {
	Notify(ctx context.Context, subject, message string) error
}

type GotifyNotifier struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewGotifyNotifier(endpoint, token string) *GotifyNotifier {
	return &GotifyNotifier{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		token:    token,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type NikoksrNotifier struct {
	notifier *notify.Notify
}

func NewNikoksrNotifier() *NikoksrNotifier {
	return &NikoksrNotifier{
		notifier: notify.New(),
	}
}

type NotificationManager struct {
	notifiers []Notifier
	condense  bool
}
