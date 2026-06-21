package miniflux

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

var limiter = rate.NewLimiter(1, 5) // Allow 1 request per second with a burst size of 1

// GetCategoryID retrieves the category ID for a given category name from Miniflux.
// Returns the category ID if found, or an error if the category is not found.
func GetCategoryID(apiEndpoint, apiKey, category string) (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(`%s/v1/categories`, apiEndpoint), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- apiEndpoint is from config, not user input
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check for a successful response
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch categories, status code: %d", resp.StatusCode)
	}

	// Parse the JSON response
	var categories []Category
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
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

	req, err := http.NewRequest("POST", fmt.Sprintf(`%s/v1/feeds`, apiEndpoint), strings.NewReader(fmt.Sprintf(`{"feed_url": "%s", "category_id": %d}`, rssFeed, categoryId)))
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- apiEndpoint and rssFeed are from config, not user input
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Debugf("Got response %s with response code %d\n", resp.Status, resp.StatusCode)
		return fmt.Errorf("failed to subscribe, status code: %d", resp.StatusCode)
	}

	log.Info("Subscribed to RSS feed: ", rssFeed)
	return nil
}

func GetCategoryFeeds(apiEndpoint string, apiKey string, categoryId int) ([]int, error) {
	// Construct the URL for the request
	url := fmt.Sprintf("%s/v1/categories/%d/feeds", apiEndpoint, categoryId)

	// Create a new GET request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request using an HTTP client
	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- apiEndpoint is from config, not user input
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check for a valid response status
	if resp.StatusCode != http.StatusOK {
		log.Debugf("Received unexpected status code %d when fetching feeds for category %d", resp.StatusCode, categoryId)
		return nil, fmt.Errorf("failed to fetch feeds, status code: %d", resp.StatusCode)
	}

	// Parse the response body
	var feeds []Feed
	if err := json.NewDecoder(resp.Body).Decode(&feeds); err != nil {
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
	url := fmt.Sprintf("%s/v1/feeds/%d", apiEndpoint, feedId)

	// Create a new DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make the request using an HTTP client
	client := &http.Client{}
	resp, err := client.Do(req) // #nosec G704 -- apiEndpoint is from config, not user input
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode == http.StatusNoContent {
		// HTTP 204 No Content means successful deletion
		log.Infof("Successfully deleted feed with ID %d", feedId)
		return nil
	} else if resp.StatusCode >= 400 {
		log.Debugf("Got response %s with response code %d when deleting feed ID %d", resp.Status, resp.StatusCode, feedId)
		return fmt.Errorf("failed to delete feed, status code: %d", resp.StatusCode)
	}

	return nil
}
