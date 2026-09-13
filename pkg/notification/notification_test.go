package notification

import (
	"context"
	"errors"
	"testing"
)

func TestDiscordNotifierIsInitializedAndHonorsContext(t *testing.T) {
	notifier, err := NewDiscordNotifier("token", "123456789012345678")
	if err != nil {
		t.Fatal(err)
	}
	if notifier.session.Client == nil || notifier.session.Ratelimiter == nil {
		t.Fatal("Discord session is missing required client or rate limiter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = notifier.Notify(ctx, "subject", "message")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Notify() error = %v, want context cancellation", err)
	}
}
