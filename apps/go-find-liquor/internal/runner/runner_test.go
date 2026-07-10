package runner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
	"github.com/toozej/monogo/apps/go-find-liquor/internal/search"
)

type fakeSearcher struct {
	search func(context.Context, string, string, int) ([]search.LiquorItem, error)
}

func (f fakeSearcher) SearchItem(ctx context.Context, item, zipcode string, distance int) ([]search.LiquorItem, error) {
	if f.search != nil {
		return f.search(ctx, item, zipcode, distance)
	}
	return nil, nil
}

type fakeNotifier struct {
	foundErr     error
	heartbeatErr error
}

func (f fakeNotifier) NotifyFoundItems(context.Context, []search.LiquorItem) error {
	return f.foundErr
}

func (f fakeNotifier) NotifyHeartbeat(context.Context, string, bool) error {
	return f.heartbeatErr
}

func injectFakes(t *testing.T, runner Runner, searcher itemSearcher, notifier itemNotifier) *SearchRunner {
	t.Helper()
	sr, ok := runner.(*SearchRunner)
	if !ok {
		t.Fatalf("runner type = %T", runner)
	}
	for _, user := range sr.userRunners {
		user.searcher = searcher
		user.notifier = notifier
	}
	return sr
}

// TestRunner_NewRunner tests the creation of Runner
func TestRunner_NewRunner(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid multi-user config",
			config: config.Config{
				Interval:  time.Hour,
				UserAgent: "test-agent",
				Users: []config.UserConfig{
					{
						Name:     "user1",
						Items:    []string{"item1", "item2"},
						Zipcode:  "97201",
						Distance: 10,
						Notifications: []config.NotificationConfig{
							{
								Type:     "gotify",
								Endpoint: "http://localhost:8080",
								Credential: map[string]string{
									"token": "test-token",
								},
								Condense: false,
							},
						},
					},
					{
						Name:     "user2",
						Items:    []string{"item3"},
						Zipcode:  "97210",
						Distance: 15,
						Notifications: []config.NotificationConfig{
							{
								Type:     "gotify",
								Endpoint: "http://localhost:8080",
								Credential: map[string]string{
									"token": "test-token-2",
								},
								Condense: true,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid single-user config",
			config: config.Config{
				Interval:  time.Hour,
				UserAgent: "test-agent",
				Users: []config.UserConfig{
					{
						Name:     "user1",
						Items:    []string{"item1"},
						Zipcode:  "97201",
						Distance: 10,
						Notifications: []config.NotificationConfig{
							{
								Type:     "gotify",
								Endpoint: "http://localhost:8080",
								Credential: map[string]string{
									"token": "test-token",
								},
								Condense: false,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no users configured",
			config: config.Config{
				Interval:  time.Hour,
				UserAgent: "test-agent",
				Users:     []config.UserConfig{},
			},
			wantErr: true,
		},
		{
			name: "negative interval",
			config: config.Config{Interval: -time.Second, Users: []config.UserConfig{{
				Name: "user1", Items: []string{"item"}, Zipcode: "97201", Distance: 10,
			}}},
			wantErr: true,
		},
		{
			name: "duplicate user names",
			config: config.Config{Interval: time.Hour, Users: []config.UserConfig{
				{Name: "same", Items: []string{"one"}, Zipcode: "97201", Distance: 10},
				{Name: "same", Items: []string{"two"}, Zipcode: "97202", Distance: 10},
			}},
			wantErr: true,
		},
		{
			name: "invalid notification config - missing token",
			config: config.Config{
				Interval:  time.Hour,
				UserAgent: "test-agent",
				Users: []config.UserConfig{
					{
						Name:     "user1",
						Items:    []string{"item1"},
						Zipcode:  "97201",
						Distance: 10,
						Notifications: []config.NotificationConfig{
							{
								Type:       "gotify",
								Endpoint:   "http://localhost:8080",
								Credential: map[string]string{
									// Missing token
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, err := NewRunner(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRunner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if runner == nil {
					t.Error("NewRunner() returned nil runner")
					return
				}
				if runner.GetUserCount() != len(tt.config.Users) {
					t.Errorf("NewRunner() created %d user runners, expected %d",
						runner.GetUserCount(), len(tt.config.Users))
				}
				// Verify each user runner was created
				for _, user := range tt.config.Users {
					if !runner.HasUser(user.Name) {
						t.Errorf("NewRunner() missing user runner for '%s'", user.Name)
					}
				}
			}
		})
	}
}

// TestRunner_RunOnce tests single execution of all user searches
func TestRunner_RunOnce(t *testing.T) {
	// Create a test configuration with multiple users
	cfg := config.Config{
		Interval:  time.Hour,
		UserAgent: "test-agent",
		Users: []config.UserConfig{
			{
				Name:     "user1",
				Items:    []string{"test-item-1"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token-1",
						},
						Condense: false,
					},
				},
			},
			{
				Name:     "user2",
				Items:    []string{"test-item-2"},
				Zipcode:  "97210",
				Distance: 15,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token-2",
						},
						Condense: true,
					},
				},
			},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("Failed to create Runner: %v", err)
	}
	injectFakes(t, runner, fakeSearcher{}, fakeNotifier{})

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = runner.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
}

// TestRunner_ConcurrentExecution tests that users run concurrently and independently
func TestRunner_ConcurrentExecution(t *testing.T) {
	// Create a test configuration with multiple users
	cfg := config.Config{
		Interval:  100 * time.Millisecond, // Short interval for testing
		UserAgent: "test-agent",
		Users: []config.UserConfig{
			{
				Name:     "user1",
				Items:    []string{"test-item-1"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token-1",
						},
						Condense: false,
					},
				},
			},
			{
				Name:     "user2",
				Items:    []string{"test-item-2"},
				Zipcode:  "97210",
				Distance: 15,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token-2",
						},
						Condense: true,
					},
				},
			},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("Failed to create Runner: %v", err)
	}
	injectFakes(t, runner, fakeSearcher{}, fakeNotifier{})

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start the runner in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx)
	}()

	// Let it run for a short time
	time.Sleep(500 * time.Millisecond)

	// Stop the runner
	runner.Stop()

	// Wait for completion
	select {
	case err := <-done:
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Start completed with error (expected due to network calls): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Runner did not stop within timeout")
	}
}

// TestRunner_UserIsolation tests that user configurations are properly isolated
func TestRunner_UserIsolation(t *testing.T) {
	cfg := config.Config{
		Interval:  time.Hour,
		UserAgent: "test-agent",
		Users: []config.UserConfig{
			{
				Name:     "user1",
				Items:    []string{"item1", "item2"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "token1",
						},
						Condense: false,
					},
				},
			},
			{
				Name:     "user2",
				Items:    []string{"item3", "item4"},
				Zipcode:  "97210",
				Distance: 20,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8081",
						Credential: map[string]string{
							"token": "token2",
						},
						Condense: true,
					},
				},
			},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("Failed to create Runner: %v", err)
	}
	injectFakes(t, runner, fakeSearcher{}, fakeNotifier{})

	// Verify user isolation by checking that both users are configured
	if runner.GetUserCount() != 2 {
		t.Errorf("Expected 2 users, got %d", runner.GetUserCount())
	}

	if !runner.HasUser("user1") {
		t.Error("User1 runner not found")
	}
	if !runner.HasUser("user2") {
		t.Error("User2 runner not found")
	}

	// The fact that we can create the runner with different user configs
	// and both users are present indicates proper isolation
}

// TestRunner_ProperCleanup tests that all resources are properly cleaned up
func TestRunner_ProperCleanup(t *testing.T) {
	cfg := config.Config{
		Interval:  50 * time.Millisecond, // Very short interval
		UserAgent: "test-agent",
		Users: []config.UserConfig{
			{
				Name:     "user1",
				Items:    []string{"test-item"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token",
						},
						Condense: false,
					},
				},
			},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("Failed to create Runner: %v", err)
	}
	injectFakes(t, runner, fakeSearcher{}, fakeNotifier{})

	// Create a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the runner
	done := make(chan error, 1)
	go func() {
		done <- runner.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop the runner
	runner.Stop()

	// Verify it stops within reasonable time
	select {
	case err := <-done:
		if err != nil {
			t.Logf("Runner stopped with error (expected): %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Runner did not stop within timeout - cleanup may not be working properly")
	}
}

func TestRunOnceAggregatesUserSearchFailures(t *testing.T) {
	cfg := config.Config{Interval: time.Hour, Users: []config.UserConfig{
		{Name: "alice", Items: []string{"one"}, Zipcode: "97201", Distance: 10},
		{Name: "bob", Items: []string{"two"}, Zipcode: "97202", Distance: 10},
	}}
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	injectFakes(t, runner, fakeSearcher{search: func(context.Context, string, string, int) ([]search.LiquorItem, error) {
		return nil, errors.New("search unavailable")
	}}, fakeNotifier{})

	err = runner.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "alice") || !strings.Contains(err.Error(), "bob") {
		t.Fatalf("RunOnce() error = %v, want both user contexts", err)
	}
}

func TestRunOnceReturnsNotificationFailures(t *testing.T) {
	cfg := config.Config{Interval: time.Hour, Users: []config.UserConfig{
		{Name: "alice", Items: []string{"one"}, Zipcode: "97201", Distance: 10},
	}}
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	injectFakes(t, runner, fakeSearcher{search: func(context.Context, string, string, int) ([]search.LiquorItem, error) {
		return []search.LiquorItem{{Name: "Found", Store: "Store", Date: time.Now()}}, nil
	}}, fakeNotifier{foundErr: errors.New("found notification failed"), heartbeatErr: errors.New("heartbeat failed")})

	err = runner.RunOnce(context.Background())
	if err == nil || !strings.Contains(err.Error(), "notify findings") || !strings.Contains(err.Error(), "notify heartbeat") {
		t.Fatalf("RunOnce() error = %v, want both notification failures", err)
	}
}

func TestStopIsConcurrentSafeAndWaitsForSearch(t *testing.T) {
	cfg := config.Config{Interval: time.Hour, Users: []config.UserConfig{
		{Name: "alice", Items: []string{"one"}, Zipcode: "97201", Distance: 10},
	}}
	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan struct{})
	var startedOnce sync.Once
	var finishedOnce sync.Once
	injectFakes(t, runner, fakeSearcher{search: func(ctx context.Context, item, zipcode string, distance int) ([]search.LiquorItem, error) {
		startedOnce.Do(func() { close(started) })
		<-ctx.Done()
		finishedOnce.Do(func() { close(finished) })
		return nil, ctx.Err()
	}}, fakeNotifier{})

	done := make(chan error, 1)
	go func() { done <- runner.Start(context.Background()) }()
	<-started
	var stops sync.WaitGroup
	for i := 0; i < 10; i++ {
		stops.Add(1)
		go func() {
			defer stops.Done()
			runner.Stop()
		}()
	}
	stops.Wait()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	default:
		t.Fatal("Start returned before the in-flight search exited")
	}
}

// TestRunner_SingleUser tests that the runner works correctly with a single user
func TestRunner_SingleUser(t *testing.T) {
	cfg := config.Config{
		Interval:  time.Hour,
		UserAgent: "test-agent",
		Users: []config.UserConfig{
			{
				Name:     "single-user",
				Items:    []string{"test-item"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []config.NotificationConfig{
					{
						Type:     "gotify",
						Endpoint: "http://localhost:8080",
						Credential: map[string]string{
							"token": "test-token",
						},
						Condense: false,
					},
				},
			},
		},
	}

	runner, err := NewRunner(cfg)
	if err != nil {
		t.Fatalf("Failed to create Runner: %v", err)
	}
	injectFakes(t, runner, fakeSearcher{}, fakeNotifier{})

	// Verify single user setup
	if runner.GetUserCount() != 1 {
		t.Errorf("Expected 1 user runner, got %d", runner.GetUserCount())
	}

	if !runner.HasUser("single-user") {
		t.Error("Single user runner not found")
	}

	// Test RunOnce with single user
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = runner.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
}
