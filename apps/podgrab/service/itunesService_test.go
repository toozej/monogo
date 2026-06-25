// Package service implements business logic for podcast management and downloads.
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: iTunes service tests require network access to itunes.apple.com API.
// Since ITUNES_BASE is a constant and cannot be mocked, these tests are skipped
// to avoid external dependencies. In a production environment, you would
// want to refactor the service to use dependency injection for testability.

// TestItunesService_Query tests iTunes API podcast search.
func TestItunesService_Query(t *testing.T) {
	t.Skip("Skipping iTunes Query test - requires network access to itunes.apple.com")

	// Test with actual API (when network is available)
	service := ItunesService{}
	results := service.Query("podcast")
	assert.NotNil(t, results, "Should return results array")
}

// TestItunesService_Constants tests that iTunes constants are defined.
func TestItunesService_Constants(t *testing.T) {
	// Verify ItunesBase constant is set
	assert.Equal(t, "https://itunes.apple.com", ItunesBase, "Should have correct iTunes base URL")
}

// TestPodcastIndexService_Credentials tests that Podcast Index credentials can be retrieved.
func TestPodcastIndexService_Credentials(t *testing.T) {
	// Verify credentials can be retrieved
	key, secret := getPodcastIndexCredentials()
	assert.NotEmpty(t, key, "Should have Podcast Index API key")
	assert.NotEmpty(t, secret, "Should have Podcast Index API secret")
}

// TestPodcastIndexService_Query tests Podcast Index API search.
func TestPodcastIndexService_Query(t *testing.T) {
	t.Skip("Skipping Podcast Index Query test - requires network access to podcastindex API")

	// Test with actual API (when network is available)
	service := PodcastIndexService{}
	results := service.Query("technology")
	assert.NotNil(t, results, "Should return results array")
}
