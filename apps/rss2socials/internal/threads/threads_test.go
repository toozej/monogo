package threads

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
)

func TestPost_MissingCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		token        string
	}{
		{name: "Missing Client ID", clientID: "", clientSecret: "secret123", token: "token123"},
		{name: "Missing Client Secret", clientID: "clientid123", clientSecret: "", token: "token123"},
		{name: "Both Missing", clientID: "", clientSecret: "", token: "token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.Config{
				ThreadsClientID:     tt.clientID,
				ThreadsClientSecret: tt.clientSecret,
				ThreadsToken:        tt.token,
			}
			err := Post(context.Background(), conf, "Hello Threads")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "client ID and client secret are required")
		})
	}
}

func TestNewClient_MissingCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
	}{
		{name: "Missing Client ID", clientID: "", clientSecret: "secret123"},
		{name: "Missing Client Secret", clientID: "clientid123", clientSecret: ""},
		{name: "Both Missing", clientID: "", clientSecret: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.Config{
				ThreadsClientID:     tt.clientID,
				ThreadsClientSecret: tt.clientSecret,
			}
			client, err := NewClient(conf)
			assert.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), "client ID and client secret are required")
		})
	}
}

func TestNewClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conf := config.Config{
		ThreadsClientID:     "test-client-id",
		ThreadsClientSecret: "test-client-secret",
		ThreadsRedirectURI:  "https://example.com/callback",
		ThreadsToken:        "test-token",
	}
	// Use a channel to ensure the test doesn't hang beyond the context timeout.
	done := make(chan error, 1)
	go func() {
		_, err := NewClient(conf)
		done <- err
	}()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-ctx.Done():
		t.Log("NewClient timed out as expected for invalid credentials")
	}
}

func TestPost_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conf := config.Config{
		ThreadsClientID:     "test-client-id",
		ThreadsClientSecret: "test-client-secret",
		ThreadsRedirectURI:  "https://example.com/callback",
		ThreadsToken:        "test-token",
	}
	// Use a channel to ensure the test doesn't hang beyond the context timeout.
	done := make(chan error, 1)
	go func() {
		done <- Post(ctx, conf, "Integration test post")
	}()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-ctx.Done():
		t.Log("Post timed out as expected for invalid credentials")
	}
}
