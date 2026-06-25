package bluesky

import (
	"context"
	"fmt"

	"github.com/davhofer/botsky/pkg/botsky"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
)

func NewClient(ctx context.Context, conf config.Config) (*botsky.Client, error) {
	if conf.BlueskyHandle == "" || conf.BlueskyAppKey == "" {
		return nil, fmt.Errorf("bluesky handle and appkey are required")
	}

	// TODO: The botsky library does not expose its internal xrpc.Client field,
	// so we cannot set a custom PDS host (e.g. for testing with a mock server).
	// When botsky adds a WithPDS/PDSHost option or exposes SetHost on the Client,
	// update NewClient to pass conf.BlueskyPDS through so that self-hosted
	// PDS instances and test mocks work correctly. See TODO.md.
	client, err := botsky.NewClient(ctx, conf.BlueskyHandle, conf.BlueskyAppKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create bluesky client: %w", err)
	}

	if err := client.Authenticate(ctx); err != nil {
		return nil, fmt.Errorf("failed to authenticate with bluesky: %w", err)
	}

	return client, nil
}

func Post(ctx context.Context, conf config.Config, content string) error {
	if conf.BlueskyHandle == "" || conf.BlueskyAppKey == "" {
		return fmt.Errorf("bluesky handle and appkey are required")
	}

	client, err := NewClient(ctx, conf)
	if err != nil {
		return err
	}

	pb := botsky.NewPostBuilder(content)
	_, _, err = client.Post(ctx, pb)
	if err != nil {
		return fmt.Errorf("failed to create bluesky post: %w", err)
	}

	return nil
}
