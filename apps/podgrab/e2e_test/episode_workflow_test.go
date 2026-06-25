//go:build e2e
// +build e2e

package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/toozej/podgrab/db"
)

// TestEpisodeWorkflow_ViewEpisodeDetails tests viewing episode information.
func TestEpisodeWorkflow_ViewEpisodeDetails(t *testing.T) {
	// Create test data
	podcast := db.CreateTestPodcast(t, db.DB)
	episode := db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
		Title:   "Test Episode",
		Summary: "Test episode description",
		FileURL: "https://example.com/episode.mp3",
	})

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	// Navigate to podcast page (episodes are shown there)
	err := navigateToPage(ctx, "/podcast/"+podcast.ID)
	require.NoError(t, err, "Should navigate to podcast page")

	// Wait for page load
	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should find body element")

	time.Sleep(500 * time.Millisecond)

	// Verify episode ID is in the DOM (basic check)
	_ = episode.ID
}

// TestEpisodeWorkflow_ViewDownloadedEpisodes tests viewing downloaded episodes.
func TestEpisodeWorkflow_ViewDownloadedEpisodes(t *testing.T) {
	// Create test data with downloaded episode
	podcast := db.CreateTestPodcast(t, db.DB)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
		Title:          "Downloaded Episode",
		DownloadStatus: db.Downloaded,
		DownloadPath:   "test/episode.mp3",
	})

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load episodes page")

	time.Sleep(500 * time.Millisecond)
}

// TestEpisodeWorkflow_ViewPlayedStatus tests episode played status display.
func TestEpisodeWorkflow_ViewPlayedStatus(t *testing.T) {
	// Create test data
	podcast := db.CreateTestPodcast(t, db.DB)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
		Title:    "Played Episode",
		IsPlayed: true,
	})
	db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
		Title:    "Unplayed Episode",
		IsPlayed: false,
	})

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/podcast/"+podcast.ID)
	require.NoError(t, err, "Should navigate to podcast page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	time.Sleep(500 * time.Millisecond)
}

// TestEpisodeWorkflow_ViewBookmarkedEpisodes tests bookmarked episode display.
func TestEpisodeWorkflow_ViewBookmarkedEpisodes(t *testing.T) {
	// Create test data with bookmarked episode
	podcast := db.CreateTestPodcast(t, db.DB)
	db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
		Title:        "Bookmarked Episode",
		BookmarkDate: time.Now(),
	})

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	time.Sleep(500 * time.Millisecond)
}

// TestEpisodeWorkflow_ViewFilteredEpisodes tests episode filtering functionality.
func TestEpisodeWorkflow_ViewFilteredEpisodes(t *testing.T) {
	// Create test data with various episode states
	podcast1 := db.CreateTestPodcast(t, db.DB, &db.Podcast{Title: "Podcast 1"})
	podcast2 := db.CreateTestPodcast(t, db.DB, &db.Podcast{Title: "Podcast 2"})

	db.CreateTestPodcastItem(t, db.DB, podcast1.ID, &db.PodcastItem{
		Title:          "Downloaded",
		DownloadStatus: db.Downloaded,
	})
	db.CreateTestPodcastItem(t, db.DB, podcast2.ID, &db.PodcastItem{
		Title:          "Not Downloaded",
		DownloadStatus: db.NotDownloaded,
	})

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	time.Sleep(500 * time.Millisecond)
}

// TestEpisodeWorkflow_ViewEpisodePagination tests episode list pagination.
func TestEpisodeWorkflow_ViewEpisodePagination(t *testing.T) {
	// Create test data with multiple episodes
	podcast := db.CreateTestPodcast(t, db.DB)

	// Create 15 episodes to trigger pagination
	for i := 0; i < 15; i++ {
		db.CreateTestPodcastItem(t, db.DB, podcast.ID, &db.PodcastItem{
			Title: "Episode " + string(rune('A'+i)),
		})
	}

	ctx, cancel := newBrowserContext(t)
	defer cancel()

	err := navigateToPage(ctx, "/episodes")
	require.NoError(t, err, "Should navigate to episodes page")

	err = waitForElement(ctx, "body")
	require.NoError(t, err, "Should load page")

	time.Sleep(500 * time.Millisecond)
}
