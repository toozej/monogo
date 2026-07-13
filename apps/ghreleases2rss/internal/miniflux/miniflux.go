package miniflux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	log "github.com/sirupsen/logrus"
)

type Feed struct {
	ID int `json:"id"`
	// Other fields in the feed struct can be added as needed
}

// Category represents a Miniflux category.
type Category struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

var limiter = rate.NewLimiter(1, 5) // Allow 1 request per second with a burst size of 5.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const maxResponseBytes = 10 << 20

func endpoint(apiEndpoint string, segments ...string) (string, error) {
	base, err := url.ParseRequestURI(apiEndpoint)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", fmt.Errorf("invalid miniflux API endpoint")
	}
	parts := append([]string{strings.TrimRight(apiEndpoint, "/")}, segments...)
	return url.JoinPath(parts[0], parts[1:]...)
}

// GetCategoryID retrieves the category ID for a given category name from Miniflux.
// Returns the category ID if found, or an error if the category is not found.
func GetCategoryID(apiEndpoint, apiKey, category string) (int, error) {
	requestURL, err := endpoint(apiEndpoint, "v1", "categories")
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req) // #nosec G704 -- validated configured endpoint
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for a successful response
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch categories, status code: %d", resp.StatusCode)
	}

	// Parse the JSON response
	var categories []Category
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&categories); err != nil {
		return 0, err
	}

	// Search for the category by title (case insensitive)
	for _, cat := range categories {
		if strings.EqualFold(cat.Title, category) {
			log.Debugf("Found RSS reader category %s which has ID %d\n", category, cat.ID)
			return cat.ID, nil
		}
	}

	// Return 0 if the category is not found
	return 0, fmt.Errorf("category %s not found", category)
}

// SubscribeToRSS subscribes to an RSS feed in Miniflux, optionally within a specific category.
// If the category ID is non-zero, the feed is subscribed within the category.
func SubscribeToFeed(apiEndpoint string, apiKey string, categoryId int, rssFeed string) error {
	// Wait for permission to proceed from the rate limiter
	err := limiter.Wait(context.Background())
	if err != nil {
		return err
	}

	requestURL, err := endpoint(apiEndpoint, "v1", "feeds")
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"feed_url": rssFeed, "category_id": categoryId})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req) // #nosec G704 -- validated configured endpoint
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		log.Debugf("Got response %s with response code %d\n", resp.Status, resp.StatusCode)
		return fmt.Errorf("failed to subscribe, status code: %d", resp.StatusCode)
	}

	log.Info("Subscribed to RSS feed: ", rssFeed)
	return nil
}

func GetCategoryFeeds(apiEndpoint string, apiKey string, categoryId int) ([]int, error) {
	// Construct the URL for the request
	requestURL, err := endpoint(apiEndpoint, "v1", "categories", strconv.Itoa(categoryId), "feeds")
	if err != nil {
		return nil, err
	}

	// Create a new GET request
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request using an HTTP client
	resp, err := httpClient.Do(req) // #nosec G704 -- validated configured endpoint
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for a valid response status
	if resp.StatusCode != http.StatusOK {
		log.Debugf("Received unexpected status code %d when fetching feeds for category %d", resp.StatusCode, categoryId)
		return nil, fmt.Errorf("failed to fetch feeds, status code: %d", resp.StatusCode)
	}

	// Parse the response body
	var feeds []Feed
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&feeds); err != nil {
		return nil, err
	}

	// Extract the feed IDs from the feeds
	var feedIDs []int
	for _, feed := range feeds {
		feedIDs = append(feedIDs, feed.ID)
	}

	log.Infof("Found %d feeds for category ID %d", len(feedIDs), categoryId)
	return feedIDs, nil
}

func DeleteFeed(apiEndpoint string, apiKey string, feedId int) error {
	// Wait for permission to proceed from the rate limiter
	err := limiter.Wait(context.Background())
	if err != nil {
		return err
	}

	// Construct the URL for the DELETE request
	requestURL, err := endpoint(apiEndpoint, "v1", "feeds", strconv.Itoa(feedId))
	if err != nil {
		return err
	}

	// Create a new DELETE request
	req, err := http.NewRequest(http.MethodDelete, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request using an HTTP client
	resp, err := httpClient.Do(req) // #nosec G704 -- validated configured endpoint
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check the response status
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		// HTTP 204 No Content means successful deletion
		log.Infof("Successfully deleted feed with ID %d", feedId)
		return nil
	}
	log.Debugf("Got response %s with response code %d when deleting feed ID %d", resp.Status, resp.StatusCode, feedId)
	return fmt.Errorf("failed to delete feed, status code: %d", resp.StatusCode)
}
