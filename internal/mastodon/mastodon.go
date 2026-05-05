// Package mastodon provides functionality for interacting with the Mastodon API.
// It includes utilities for formatting toot content from RSS items and sending posts to Mastodon instances
// using the github.com/mattn/go-mastodon library.
package mastodon

import (
	"context"
	"fmt"

	"github.com/mattn/go-mastodon"
	"github.com/toozej/rss2socials/internal/rss"
	"github.com/toozej/rss2socials/pkg/config"
)

// GetTootContent constructs the toot message for the given RSS item.
// Posts with content use the "Content - Link" format; others use "New blog post: Link".
func GetTootContent(post rss.RSSItem) string {
	if post.Content != "" {
		return fmt.Sprintf("%s - %s", post.Content, post.Link)
	}
	return fmt.Sprintf("New blog post: %s", post.Link)
}

// NewClient creates a new Mastodon API client from the given configuration.
func NewClient(conf config.Config) *mastodon.Client {
	return mastodon.NewClient(&mastodon.Config{
		Server:       conf.MastodonURL,
		ClientID:     conf.MastodonClientKey,
		ClientSecret: conf.MastodonClientSecret,
		AccessToken:  conf.MastodonAccessToken,
	})
}

// TootPost sends a post to Mastodon using the go-mastodon library.
func TootPost(conf config.Config, content string) error {
	if conf.MastodonURL == "" || conf.MastodonAccessToken == "" {
		return fmt.Errorf("mastodon URL and access token must be set")
	}

	client := NewClient(conf)
	_, err := client.PostStatus(context.Background(), &mastodon.Toot{
		Status:     content,
		Visibility: mastodon.VisibilityPublic,
	})
	return err
}
