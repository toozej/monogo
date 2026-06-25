package threads

import (
	"context"
	"fmt"

	threadsgo "github.com/tirthpatell/threads-go"

	"github.com/toozej/rss2socials/pkg/config"
)

func NewClient(conf config.Config) (*threadsgo.Client, error) {
	if conf.ThreadsClientID == "" || conf.ThreadsClientSecret == "" {
		return nil, fmt.Errorf("threads client ID and client secret are required")
	}

	config := &threadsgo.Config{
		ClientID:     conf.ThreadsClientID,
		ClientSecret: conf.ThreadsClientSecret,
		RedirectURI:  conf.ThreadsRedirectURI,
		Scopes:       []string{"threads_basic", "threads_content_publish"},
	}

	if conf.ThreadsToken != "" {
		client, err := threadsgo.NewClientWithToken(conf.ThreadsToken, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create threads client with token: %w", err)
		}
		return client, nil
	}

	client, err := threadsgo.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create threads client: %w", err)
	}
	return client, nil
}

func Post(ctx context.Context, conf config.Config, content string) error {
	if conf.ThreadsClientID == "" || conf.ThreadsClientSecret == "" {
		return fmt.Errorf("threads client ID and client secret are required")
	}

	client, err := NewClient(conf)
	if err != nil {
		return err
	}

	_, err = client.CreateTextPost(ctx, &threadsgo.TextPostContent{
		Text: content,
	})
	if err != nil {
		return fmt.Errorf("failed to create threads post: %w", err)
	}

	return nil
}
