package runner

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-find-liquor/internal/notification"
	"github.com/toozej/go-find-liquor/internal/search"
	"github.com/toozej/go-find-liquor/pkg/config"
)

// Runner interface defines the contract for all runner implementations
type Runner interface {
	Start(ctx context.Context) error
	Stop()
	RunOnce(ctx context.Context) error
	// GetUserCount returns the number of configured users (for testing)
	GetUserCount() int
	// HasUser returns true if a user with the given name is configured (for testing)
	HasUser(name string) bool
}

// userRunner executes periodic searches for a single user (internal implementation)
type userRunner struct {
	userConfig  config.UserConfig
	searcher    *search.Searcher
	notifier    *notification.NotificationManager
	stopChan    chan struct{}
	runningCh   chan struct{}
	interval    time.Duration
	commonItems []string
}

// newUserRunner creates a new user runner with the given user configuration (internal function)
func newUserRunner(userConfig config.UserConfig, interval time.Duration, userAgent string, commonItems []string) (*userRunner, error) {
	// Initialize the searcher
	searcher := search.NewSearcher(userAgent)

	// Initialize notification manager for this user
	notifier, err := notification.NewNotificationManager(userConfig.Notifications)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification manager for user '%s': %w", userConfig.Name, err)
	}

	return &userRunner{
		userConfig:  userConfig,
		searcher:    searcher,
		notifier:    notifier,
		stopChan:    make(chan struct{}),
		runningCh:   make(chan struct{}, 1),
		interval:    interval,
		commonItems: commonItems,
	}, nil
}

// start begins periodic searches for this user (internal method)
func (ur *userRunner) start(ctx context.Context) error {
	log.Infof("Starting search runner for user '%s'", ur.userConfig.Name)

	// Initial search
	go func() {
		ur.runningCh <- struct{}{}
		defer func() {
			<-ur.runningCh
		}()

		if err := ur.runSearch(ctx, true); err != nil {
			log.Errorf("Search failed for user '%s': %v", ur.userConfig.Name, err)
		}
	}()

	// Setup ticker for recurring searches
	ticker := time.NewTicker(ur.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if we're already running
			select {
			case ur.runningCh <- struct{}{}:
				// We got the semaphore, run the search
				go func() {
					defer func() {
						<-ur.runningCh
					}()

					if err := ur.runSearch(ctx, true); err != nil {
						log.Errorf("Search failed for user '%s': %v", ur.userConfig.Name, err)
					}
				}()
			default:
				// A search is already running, skip this tick
				log.Warnf("Previous search still running for user '%s', skipping", ur.userConfig.Name)
			}
		case <-ur.stopChan:
			log.Infof("Stopping search runner for user '%s'", ur.userConfig.Name)
			return nil
		case <-ctx.Done():
			log.Infof("Context cancelled for user '%s'", ur.userConfig.Name)
			return ctx.Err()
		}
	}
}

// runSearch performs a single search for all items for this user
// Collects all found items before sending notifications
// If withHealthCheck is true, a random common item is also searched as a health check
func (ur *userRunner) runSearch(ctx context.Context, withHealthCheck bool) error {
	if len(ur.userConfig.Items) == 0 {
		return fmt.Errorf("user '%s' has no items to search for", ur.userConfig.Name)
	}

	if ur.userConfig.Zipcode == "" {
		return fmt.Errorf("user '%s' has no zipcode configured", ur.userConfig.Name)
	}

	log.Infof("Starting search for user '%s': %d items within %d miles of %s",
		ur.userConfig.Name, len(ur.userConfig.Items), ur.userConfig.Distance, ur.userConfig.Zipcode)

	var allFoundItems []search.LiquorItem

	for _, item := range ur.userConfig.Items {
		// Create a context with timeout for this item
		itemCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		log.Infof("User '%s' searching for item: %s", ur.userConfig.Name, item)

		// Search for the item
		results, err := ur.searcher.SearchItem(itemCtx, item, ur.userConfig.Zipcode, ur.userConfig.Distance)
		if err != nil {
			log.Errorf("Failed to search for %s for user '%s': %v", item, ur.userConfig.Name, err)
			continue
		}

		log.Infof("User '%s' found %d results for %s", ur.userConfig.Name, len(results), item)

		// Collect all found items
		allFoundItems = append(allFoundItems, results...)

		// Random wait between searches to avoid overwhelming the service
		if len(ur.userConfig.Items) > 1 && item != ur.userConfig.Items[len(ur.userConfig.Items)-1] {
			randTimeBig := new(big.Int)
			randTimeBig.SetInt64(int64(30))
			randTime, _ := rand.Int(rand.Reader, randTimeBig)
			waitTime := time.Duration(randTime.Int64()) * time.Second
			log.Debugf("User '%s' waiting %s before next search", ur.userConfig.Name, waitTime)

			select {
			case <-time.After(waitTime):
				// Continue to next item
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Send notifications for all found items (condensed or individual based on user config)
	if len(allFoundItems) > 0 {
		if err := ur.notifier.NotifyFoundItems(ctx, allFoundItems); err != nil {
			log.Warnf("Failed to send notifications for user '%s': %v", ur.userConfig.Name, err)
		}
	}

	// Send heartbeat notification with optional health check search result
	var healthCheckItem string
	var healthCheckFound bool
	if withHealthCheck {
		healthCheckItem = search.RandomCommonItem(ur.commonItems)
		healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Minute)
		defer healthCancel()

		log.Infof("User '%s' running health check search for common item: %s", ur.userConfig.Name, healthCheckItem)
		healthResults, err := ur.searcher.SearchItem(healthCtx, healthCheckItem, ur.userConfig.Zipcode, ur.userConfig.Distance)
		if err != nil {
			log.Warnf("Health check search failed for user '%s': %v", ur.userConfig.Name, err)
		} else {
			healthCheckFound = len(healthResults) > 0
			if healthCheckFound {
				healthCheckItem = healthResults[0].Name
			}
			log.Infof("User '%s' health check: searched for '%s', found %d results", ur.userConfig.Name, healthCheckItem, len(healthResults))
		}
	}

	if err := ur.notifier.NotifyHeartbeat(ctx, healthCheckItem, healthCheckFound); err != nil {
		log.Warnf("Failed to send heartbeat notification for user '%s': %v", ur.userConfig.Name, err)
	}

	log.Infof("Search completed for user '%s', next search in %s", ur.userConfig.Name, ur.interval)
	return nil
}

// stop halts the user runner (internal method)
func (ur *userRunner) stop() {
	close(ur.stopChan)
}

// runOnce performs a single search and returns for this user (internal method)
func (ur *userRunner) runOnce(ctx context.Context) error {
	return ur.runSearch(ctx, false)
}

// SearchRunner manages search execution for one or more users
type SearchRunner struct {
	config      config.Config
	userRunners map[string]*userRunner
	stopChan    chan struct{}
	mu          sync.RWMutex
}

// NewRunner creates a new runner with the given configuration
// Supports both single-user and multi-user configurations
func NewRunner(cfg config.Config) (Runner, error) {
	if len(cfg.Users) == 0 {
		return nil, fmt.Errorf("no users configured")
	}

	userRunners := make(map[string]*userRunner)

	// Extract common item search strings from config (use code if set, otherwise name)
	var commonItemSearches []string
	for _, ci := range cfg.CommonItems {
		if ci.Code != "" {
			commonItemSearches = append(commonItemSearches, ci.Code)
		} else if ci.Name != "" {
			commonItemSearches = append(commonItemSearches, ci.Name)
		}
	}

	// Create userRunner for each user
	for _, userConfig := range cfg.Users {
		userRunner, err := newUserRunner(userConfig, cfg.Interval, cfg.UserAgent, commonItemSearches)
		if err != nil {
			return nil, fmt.Errorf("failed to create user runner for '%s': %w", userConfig.Name, err)
		}
		userRunners[userConfig.Name] = userRunner
	}

	return &SearchRunner{
		config:      cfg,
		userRunners: userRunners,
		stopChan:    make(chan struct{}),
	}, nil
}

// Start begins concurrent searches for all users
func (sr *SearchRunner) Start(ctx context.Context) error {
	sr.mu.RLock()
	userCount := len(sr.userRunners)
	sr.mu.RUnlock()

	log.Infof("Starting search runner with %d users", userCount)

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to collect errors from user runners
	errChan := make(chan error, userCount)

	// Start each user runner in its own goroutine
	sr.mu.RLock()
	for userName, ur := range sr.userRunners {
		go func(name string, runner *userRunner) {
			log.Infof("Starting user runner for '%s'", name)
			if err := runner.start(ctx); err != nil {
				log.Errorf("User runner for '%s' failed: %v", name, err)
				errChan <- fmt.Errorf("user '%s': %w", name, err)
			} else {
				log.Infof("User runner for '%s' completed", name)
				errChan <- nil
			}
		}(userName, ur)
	}
	sr.mu.RUnlock()

	// Wait for stop signal or context cancellation
	select {
	case <-sr.stopChan:
		log.Info("SearchRunner received stop signal")
		cancel() // Cancel context to stop all user runners
	case <-ctx.Done():
		log.Info("SearchRunner context cancelled")
	}

	// Stop all user runners
	sr.mu.RLock()
	for userName, ur := range sr.userRunners {
		log.Infof("Stopping user runner for '%s'", userName)
		ur.stop()
	}
	sr.mu.RUnlock()

	// Wait for all user runners to complete (with timeout)
	completedUsers := 0
	for completedUsers < userCount {
		select {
		case err := <-errChan:
			if err != nil {
				log.Errorf("User runner error: %v", err)
			}
			completedUsers++
		case <-time.After(30 * time.Second):
			log.Warn("Timeout waiting for user runners to complete")
			return fmt.Errorf("timeout waiting for user runners to complete")
		}
	}

	log.Info("All user runners stopped")
	return nil
}

// Stop halts all user runners
func (sr *SearchRunner) Stop() {
	close(sr.stopChan)
}

// RunOnce performs a single search for all users and returns
func (sr *SearchRunner) RunOnce(ctx context.Context) error {
	sr.mu.RLock()
	userCount := len(sr.userRunners)
	sr.mu.RUnlock()

	log.Infof("Running single search for %d users", userCount)

	// Channel to collect errors from user runners
	errChan := make(chan error, userCount)

	// Run search for each user concurrently
	sr.mu.RLock()
	for userName, ur := range sr.userRunners {
		go func(name string, runner *userRunner) {
			log.Infof("Running single search for user '%s'", name)
			if err := runner.runOnce(ctx); err != nil {
				log.Errorf("Single search failed for user '%s': %v", name, err)
				errChan <- fmt.Errorf("user '%s': %w", name, err)
			} else {
				log.Infof("Single search completed for user '%s'", name)
				errChan <- nil
			}
		}(userName, ur)
	}
	sr.mu.RUnlock()

	// Wait for all searches to complete
	var lastErr error
	completedUsers := 0
	for completedUsers < userCount {
		select {
		case err := <-errChan:
			if err != nil {
				log.Errorf("User search error: %v", err)
				lastErr = err
			}
			completedUsers++
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	log.Info("All user searches completed")
	return lastErr
}

// GetUserCount returns the number of configured users (for testing)
func (sr *SearchRunner) GetUserCount() int {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return len(sr.userRunners)
}

// HasUser returns true if a user with the given name is configured (for testing)
func (sr *SearchRunner) HasUser(name string) bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	_, exists := sr.userRunners[name]
	return exists
}
