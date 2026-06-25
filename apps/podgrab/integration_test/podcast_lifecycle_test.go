//go:build integration
// +build integration

package integration_test

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toozej/podgrab/db"
	testhelpers "github.com/toozej/podgrab/internal/testing"
	"github.com/toozej/podgrab/service"
)

// TestPodcastLifecycle_AddDownloadDelete tests the complete lifecycle of a podcast.
func TestPodcastLifecycle_AddDownloadDelete(t *testing.T) {
	// Setup: real DB + real file system + mock HTTP
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	oldData := os.Getenv("DATA")
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", oldData)

	// Set global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings
	db.CreateTestSetting(t, database)

	// Mock RSS server
	server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
	defer server.Close()

	// Step 1: Add podcast (RSS parsing)
	podcast, err := service.AddPodcast(server.URL)
	require.NoError(t, err, "Should add podcast")
	assert.Equal(t, "Test Podcast", podcast.Title, "Should have correct title")

	// Add episodes (AddPodcast only creates podcast record, not episodes)
	err = service.AddPodcastItems(&podcast, true)
	require.NoError(t, err, "Should add podcast episodes")

	// Step 2: Verify episodes were created
	var episodes []db.PodcastItem
	err = database.Where("podcast_id = ?", podcast.ID).Find(&episodes).Error
	require.NoError(t, err, "Should query episodes")
	assert.Greater(t, len(episodes), 0, "Should have created episodes")

	// Verify episode data
	episode := episodes[0]
	assert.NotEmpty(t, episode.Title, "Episode should have title")
	assert.NotEmpty(t, episode.FileURL, "Episode should have file URL")
	assert.Equal(t, db.NotDownloaded, episode.DownloadStatus, "Episode should not be downloaded initially")

	// Step 3: Download episode
	mockFileServer := httptest.NewServer(testhelpers.CreateMockFileHandler("test audio content"))
	defer mockFileServer.Close()

	// Update episode with mock file URL
	episode.FileURL = mockFileServer.URL
	database.Save(&episode)

	err = service.DownloadSingleEpisode(episode.ID)
	require.NoError(t, err, "Should download episode")

	// Verify download
	var updated db.PodcastItem
	database.First(&updated, "id = ?", episode.ID)
	assert.Equal(t, db.Downloaded, updated.DownloadStatus, "Episode should be marked as downloaded")
	assert.NotEmpty(t, updated.DownloadPath, "Episode should have download path")

	// Verify file exists (DownloadPath is already an absolute path)
	_, err = os.Stat(updated.DownloadPath)
	assert.NoError(t, err, "Downloaded file should exist")

	// Step 4: Delete podcast
	err = db.DeletePodcastByID(podcast.ID)
	require.NoError(t, err, "Should delete podcast")

	// Verify podcast is deleted
	err = database.Where("id = ?", podcast.ID).First(&podcast).Error
	assert.Error(t, err, "Podcast should not be found")

	// Verify episodes are deleted
	var remainingEpisodes []db.PodcastItem
	database.Where("podcast_id = ?", podcast.ID).Find(&remainingEpisodes)
	assert.Empty(t, remainingEpisodes, "Episodes should be deleted with podcast")
}

// TestPodcastLifecycle_DuplicateDetection tests duplicate podcast prevention.
func TestPodcastLifecycle_DuplicateDetection(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Mock RSS server
	server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
	defer server.Close()

	// Add podcast first time
	_, err := service.AddPodcast(server.URL)
	require.NoError(t, err, "First add should succeed")

	// Count podcasts
	var count1 int64
	database.Model(&db.Podcast{}).Count(&count1)
	assert.Equal(t, int64(1), count1, "Should have one podcast")

	// Try to add same podcast again
	_, err = service.AddPodcast(server.URL)
	assert.Error(t, err, "Should reject duplicate podcast")

	// Verify still only one podcast
	var count2 int64
	database.Model(&db.Podcast{}).Count(&count2)
	assert.Equal(t, int64(1), count2, "Should still have only one podcast")
}

// TestPodcastLifecycle_EpisodeDeduplication tests episode GUID-based deduplication.
func TestPodcastLifecycle_EpisodeDeduplication(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Mock RSS server
	server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
	defer server.Close()

	// Add podcast
	podcast, err := service.AddPodcast(server.URL)
	require.NoError(t, err, "Should add podcast")

	// Add episodes first time
	err = service.AddPodcastItems(&podcast, true)
	require.NoError(t, err, "Should add episodes")

	// Count episodes after first addition
	var count1 int64
	database.Model(&db.PodcastItem{}).Where("podcast_id = ?", podcast.ID).Count(&count1)
	assert.Greater(t, count1, int64(0), "Should have episodes after first addition")

	// Re-parse same RSS feed (simulates refresh - should detect duplicates)
	err = service.AddPodcastItems(&podcast, false)
	require.NoError(t, err, "Should process items")

	// Verify episode count unchanged (no duplicates)
	var count2 int64
	database.Model(&db.PodcastItem{}).Where("podcast_id = ?", podcast.ID).Count(&count2)
	assert.Equal(t, count1, count2, "Should not create duplicate episodes")
}

// TestPodcastLifecycle_DownloadOnAdd tests automatic download setting.
func TestPodcastLifecycle_DownloadOnAdd(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings with DownloadOnAdd enabled
	setting := db.CreateTestSetting(t, database)
	setting.DownloadOnAdd = true
	setting.InitialDownloadCount = 2
	database.Save(setting)

	// Mock RSS server
	server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
	defer server.Close()

	// Add podcast
	podcast, err := service.AddPodcast(server.URL)
	require.NoError(t, err, "Should add podcast")

	// Add episodes (with DownloadOnAdd=true, they should be queued)
	err = service.AddPodcastItems(&podcast, true)
	require.NoError(t, err, "Should add episodes")

	// Verify some episodes queued for download
	var queuedEpisodes []db.PodcastItem
	database.Where("download_status = ?", db.NotDownloaded).Find(&queuedEpisodes)
	assert.GreaterOrEqual(t, len(queuedEpisodes), 2, "Should queue episodes for download")
}

// TestPodcastLifecycle_PlayedStatus tests marking episodes as played/unplayed.
func TestPodcastLifecycle_PlayedStatus(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	podcast := db.CreateTestPodcast(t, database)
	episode := db.CreateTestPodcastItem(t, database, podcast.ID)

	// Mark as played
	err := service.SetPodcastItemPlayedStatus(episode.ID, true)
	require.NoError(t, err, "Should mark as played")

	var updated db.PodcastItem
	database.First(&updated, "id = ?", episode.ID)
	assert.True(t, updated.IsPlayed, "Should be marked as played")

	// Mark as unplayed
	err = service.SetPodcastItemPlayedStatus(episode.ID, false)
	require.NoError(t, err, "Should mark as unplayed")

	database.First(&updated, "id = ?", episode.ID)
	assert.False(t, updated.IsPlayed, "Should be marked as unplayed")
}

// TestPodcastLifecycle_BookmarkStatus tests episode bookmarking.
func TestPodcastLifecycle_BookmarkStatus(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	podcast := db.CreateTestPodcast(t, database)
	episode := db.CreateTestPodcastItem(t, database, podcast.ID)

	// Bookmark episode
	err := service.SetPodcastItemBookmarkStatus(episode.ID, true)
	require.NoError(t, err, "Should bookmark episode")

	var updated db.PodcastItem
	database.First(&updated, "id = ?", episode.ID)
	assert.False(t, updated.BookmarkDate.IsZero(), "Should be bookmarked")

	// Remove bookmark
	err = service.SetPodcastItemBookmarkStatus(episode.ID, false)
	require.NoError(t, err, "Should remove bookmark")

	database.First(&updated, "id = ?", episode.ID)
	assert.True(t, updated.BookmarkDate.IsZero(), "Should not be bookmarked")
}
