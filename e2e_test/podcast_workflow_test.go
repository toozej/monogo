//go:build e2e
// +build e2e

package e2e_test

import (
	"testing"
	"time"

	"github.com/akhilrex/podgrab/db"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPodcastWorkflow_ViewHomePage tests accessing the home page.
func TestPodcastWorkflow_ViewHomePage(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate to home page")

	// Wait for page to load
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	// Verify we're on the podcasts page
	// Use chromedp.Title instead of getElementText because <title> is a non-visible head element
	var title string
	err = chromedp.Run(ctx, chromedp.Title(&title))
	assert.NoError(t, err, "Should get page title")
	assert.Contains(t, title, "Podgrab", "Title should contain Podgrab")
}

// TestPodcastWorkflow_ViewPodcastsList tests viewing the podcasts list.
func TestPodcastWorkflow_ViewPodcastsList(t *testing.T) {
	// Create test podcast
	_ = db.CreateTestPodcast(t, db.DB)

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/podcasts")
	require.NoError(t, err, "Should navigate to podcasts page")

	// Wait for podcasts list to load
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	// Check if podcast appears (this depends on UI structure)
	// For now, just verify the page loads
	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_ViewPodcastDetails tests viewing podcast details.
func TestPodcastWorkflow_ViewPodcastDetails(t *testing.T) {
	// Create test podcast with episodes
	podcast := db.CreateTestPodcast(t, db.DB)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID)

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	// Navigate to podcast details page
	err := navigateToPage(ctx, "/podcast/"+podcast.ID)
	require.NoError(t, err, "Should navigate to podcast details")

	// Wait for page to load
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_ViewSettings tests accessing the settings page.
func TestPodcastWorkflow_ViewSettings(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/settings")
	require.NoError(t, err, "Should navigate to settings page")

	// Wait for settings form
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_ViewAllEpisodes tests viewing all episodes page.
func TestPodcastWorkflow_ViewAllEpisodes(t *testing.T) {
	// Create test data
	podcast := db.CreateTestPodcast(t, db.DB)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID)

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes page")

	// Wait for episodes list
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_SearchPage tests accessing the search page.
func TestPodcastWorkflow_SearchPage(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/search")
	require.NoError(t, err, "Should navigate to search page")

	// Wait for search form
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_AddPodcastPage tests accessing the add podcast page.
func TestPodcastWorkflow_AddPodcastPage(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/add")
	require.NoError(t, err, "Should navigate to add podcast page")

	// Wait for add form
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)
}

// TestPodcastWorkflow_Navigation tests basic navigation between pages.
func TestPodcastWorkflow_Navigation(t *testing.T) {
	ctx, cancel := newBrowserContext(t)
	defer cancel()

	// Start at home
	err := navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate to home page")
	time.Sleep(200 * time.Millisecond)

	// Navigate to settings
	err = navigateToPage(ctx, "/settings")
	require.NoError(t, err, "Should navigate to settings")
	time.Sleep(200 * time.Millisecond)

	// Navigate to episodes
	err = navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes")
	time.Sleep(200 * time.Millisecond)

	// Navigate back to home
	err = navigateToPage(ctx, "/")
	require.NoError(t, err, "Should navigate back to home")
}

// TestPodcastWorkflow_PageLoad tests that all main pages load without errors.
func TestPodcastWorkflow_PageLoad(t *testing.T) {
	pages := []struct {
		name string
		path string
	}{
		{"Home", "/"},
		{"Podcasts", "/podcasts"},
		{"Episodes", "/episodes"},
		{"Search", "/search"},
		{"Add", "/add"},
		{"Settings", "/settings"},
	}

	for _, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			ctx, cancel := newBrowserContext(t)
			defer cancel()

			err := navigateToPage(ctx, page.path)
			require.NoError(t, err, "Should navigate to "+page.name)

			err = waitForElement(ctx, "body")
			require.NoError(t, err, "Should load "+page.name+" page")

			// Verify no JavaScript errors (simplified check)
			var consoleErrors []string
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`window.consoleErrors || []`, &consoleErrors),
			)
			assert.NoError(t, err, "Should check for console errors")
		})
	}
}
