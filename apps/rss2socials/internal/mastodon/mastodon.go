// Package mastodon provides functionality for interacting with the Mastodon API.
// It includes utilities for formatting toot content from RSS items and sending posts to Mastodon instances
// using the github.com/mattn/go-mastodon library.
package mastodon

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mattn/go-mastodon"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
	"github.com/toozej/monogo/apps/rss2socials/internal/rss"
)

// GetTootContent constructs the toot message for the given RSS item.
// Only the link is included to avoid posting large content that may exceed
// social site API limits or render poorly.
func GetTootContent(post rss.RSSItem) string {
	return fmt.Sprintf("New post: %s", post.Link)
}

// NewClient creates a new Mastodon API client from the given configuration.
func NewClient(conf config.Config) *mastodon.Client {
	client := mastodon.NewClient(&mastodon.Config{
		Server:       conf.MastodonURL,
		ClientID:     conf.MastodonClientKey,
		ClientSecret: conf.MastodonClientSecret,
		AccessToken:  conf.MastodonAccessToken,
	})
	client.Timeout = 30 * time.Second
	return client
}

// TootPost sends a post to Mastodon using the go-mastodon library.
func TootPost(conf config.Config, content string) error {
	return TootPostContext(context.Background(), conf, content, "")
}

func TootPostContext(ctx context.Context, conf config.Config, content, idempotencyKey string) error {
	if conf.MastodonURL == "" || conf.MastodonAccessToken == "" {
		return fmt.Errorf("mastodon URL and access token must be set")
	}

	client := NewClient(conf)
	if idempotencyKey != "" {
		client.Transport = idempotencyTransport{base: http.DefaultTransport, key: idempotencyKey}
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := client.PostStatus(opCtx, &mastodon.Toot{
		Status:     content,
		Visibility: mastodon.VisibilityPublic,
	})
	return err
}

type idempotencyTransport struct {
	base http.RoundTripper
	key  string
}

func (t idempotencyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	request := req.Clone(req.Context())
	request.Header.Set("Idempotency-Key", t.key)
	return t.base.RoundTrip(request)
}
