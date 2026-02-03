// Package service implements business logic for podcast management and downloads.
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: GPodder service tests require network access to gpodder.net API.
// Since BASE is a constant and cannot be mocked, these tests are skipped
// to avoid external dependencies. In a production environment, you would
// want to refactor the service to use dependency injection for testability.

// TestQuery tests GPodder podcast search.
func TestQuery(t *testing.T) {
	t.Skip("Skipping Query test - requires network access to gpodder.net")

	// Test with actual API (when network is available)
	results := Query("technology")
	assert.NotNil(t, results, "Should return results array")
}

// TestByTag tests GPodder tag-based podcast discovery.
func TestByTag(t *testing.T) {
	t.Skip("Skipping ByTag test - requires network access to gpodder.net")

	// Test with actual API (when network is available)
	results := ByTag("technology", 5)
	assert.NotNil(t, results, "Should return results array")
}

// TestTop tests GPodder top podcasts retrieval.
func TestTop(t *testing.T) {
	t.Skip("Skipping Top test - requires network access to gpodder.net")

	// Test with actual API (when network is available)
	results := Top(10)
	assert.NotNil(t, results, "Should return results array")
}

// TestTags tests GPodder popular tags retrieval.
func TestTags(t *testing.T) {
	t.Skip("Skipping Tags test - requires network access to gpodder.net")

	// Test with actual API (when network is available)
	results := Tags(10)
	assert.NotNil(t, results, "Should return results array")
}

// TestGPodder_Constants tests that GPodder constants are defined.
func TestGPodder_Constants(t *testing.T) {
	// Verify BASE constant is set
	assert.Equal(t, "https://gpodder.net", BASE, "Should have correct GPodder base URL")
}
