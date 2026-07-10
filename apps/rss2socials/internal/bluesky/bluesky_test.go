package bluesky

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
)

func TestPost_MissingCredentials(t *testing.T) {
	tests := []struct {
		name   string
		handle string
		appkey string
	}{
		{name: "Missing Handle", handle: "", appkey: "appkey123"},
		{name: "Missing AppKey", handle: "user.bsky.social", appkey: ""},
		{name: "Both Missing", handle: "", appkey: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.Config{
				BlueskyHandle: tt.handle,
				BlueskyAppKey: tt.appkey,
			}
			err := Post(context.Background(), conf, "Hello Bluesky")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "handle and appkey are required")
		})
	}
}

func TestNewClient_MissingCredentials(t *testing.T) {
	tests := []struct {
		name   string
		handle string
		appkey string
	}{
		{name: "Missing Handle", handle: "", appkey: "appkey123"},
		{name: "Missing AppKey", handle: "user.bsky.social", appkey: ""},
		{name: "Both Missing", handle: "", appkey: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.Config{
				BlueskyHandle: tt.handle,
				BlueskyAppKey: tt.appkey,
			}
			client, err := NewClient(context.Background(), conf)
			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), "handle and appkey are required")
		})
	}
}

func TestPost_Integration(t *testing.T) {
	if os.Getenv("RSS2SOCIALS_LIVE_TESTS") == "" {
		t.Skip("set RSS2SOCIALS_LIVE_TESTS=1 to run live integration tests")
	}

	conf := config.Config{
		BlueskyHandle: "test.bsky.social",
		BlueskyAppKey: "test-appkey",
	}
	err := Post(context.Background(), conf, "Integration test post")
	assert.Error(t, err)
}

func TestNewClient_Integration(t *testing.T) {
	if os.Getenv("RSS2SOCIALS_LIVE_TESTS") == "" {
		t.Skip("set RSS2SOCIALS_LIVE_TESTS=1 to run live integration tests")
	}

	conf := config.Config{
		BlueskyHandle: "test.bsky.social",
		BlueskyAppKey: "test-appkey",
	}
	client, err := NewClient(context.Background(), conf)
	assert.Error(t, err)
	assert.Nil(t, client)
}
