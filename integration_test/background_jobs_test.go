//go:build integration
// +build integration

package integration_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toozej/podgrab/db"
	testhelpers "github.com/toozej/podgrab/internal/testing"
	"github.com/toozej/podgrab/service"
	"gorm.io/gorm"
)

// TestBackgroundJob_RefreshEpisodes tests detecting new episodes from RSS feed.
func TestBackgroundJob_RefreshEpisodes(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Create podcast with initial RSS feed
	server := httptest.NewServer(testhelpers.CreateMockRSSHandler(testhelpers.ValidRSSFeed))
	defer server.Close()

	_, err := service.AddPodcast(server.URL)
	require.NoError(t, err, "Should add podcast")

	var podcast db.Podcast
	database.Where("url = ?", server.URL).First(&podcast)

	// Count initial episodes
	var initialCount int64
	database.Model(&db.PodcastItem{}).Where("podcast_id = ?", podcast.ID).Count(&initialCount)

	// Generate RSS with additional episodes
	newFeed := testhelpers.GenerateLargeRSSFeed(15) // More episodes than initial feed
	server.Config.Handler = testhelpers.CreateMockRSSHandler(newFeed)

	// Refresh episodes (simulates background job)
	err = service.RefreshEpisodes()
	require.NoError(t, err, "Should refresh episodes")

	// Verify new episodes were added
	var finalCount int64
	database.Model(&db.PodcastItem{}).Where("podcast_id = ?", podcast.ID).Count(&finalCount)
	assert.Greater(t, finalCount, initialCount, "Should add new episodes")
}

// TestBackgroundJob_DownloadMissingEpisodes tests auto-download queued episodes.
func TestBackgroundJob_DownloadMissingEpisodes(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	setting := db.CreateTestSetting(t, database)
	setting.AutoDownload = true
	setting.MaxDownloadConcurrency = 2
	database.Save(setting)

	// Create podcast with episodes
	podcast := db.CreateTestPodcast(t, database)

	// Mock file server
	mockFileServer := httptest.NewServer(testhelpers.CreateMockFileHandler("test audio"))
	defer mockFileServer.Close()

	// Create episodes queued for download
	item1 := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		FileURL:        mockFileServer.URL + "/ep1.mp3",
		DownloadStatus: db.NotDownloaded,
	})
	item2 := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		FileURL:        mockFileServer.URL + "/ep2.mp3",
		DownloadStatus: db.NotDownloaded,
	})

	// Run download job
	err := service.DownloadMissingEpisodes()
	require.NoError(t, err, "Should download queued episodes")

	// Verify downloads completed
	var updated1, updated2 db.PodcastItem
	database.First(&updated1, "id = ?", item1.ID)
	database.First(&updated2, "id = ?", item2.ID)

	assert.Equal(t, db.Downloaded, updated1.DownloadStatus, "Episode 1 should be downloaded")
	assert.Equal(t, db.Downloaded, updated2.DownloadStatus, "Episode 2 should be downloaded")
	assert.NotEmpty(t, updated1.DownloadPath, "Episode 1 should have download path")
	assert.NotEmpty(t, updated2.DownloadPath, "Episode 2 should have download path")

	// Verify files exist (DownloadPath is already absolute)
	_, err1 := os.Stat(updated1.DownloadPath)
	_, err2 := os.Stat(updated2.DownloadPath)
	assert.NoError(t, err1, "Episode 1 file should exist")
	assert.NoError(t, err2, "Episode 2 file should exist")
}

// TestBackgroundJob_CheckMissingFiles tests detection of manually deleted files.
func TestBackgroundJob_CheckMissingFiles(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	setting := db.CreateTestSetting(t, database)
	setting.DontDownloadDeletedFromDisk = true
	database.Save(setting)

	podcast := db.CreateTestPodcast(t, database)

	// Create the file initially
	podcastDir := filepath.Join(tmpDir, "test-podcast")
	os.MkdirAll(podcastDir, 0755)
	filePath := filepath.Join(podcastDir, "episode.mp3")
	os.WriteFile(filePath, []byte("test"), 0644)

	// Create downloaded episode with absolute path
	item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		DownloadStatus: db.Downloaded,
		DownloadPath:   filePath,
	})

	// Verify file exists
	_, err := os.Stat(filePath)
	require.NoError(t, err, "File should exist initially")

	// Delete file manually (simulates user action)
	os.Remove(filePath)

	// Run check missing files job
	err = service.CheckMissingFiles()
	require.NoError(t, err, "Should check missing files")

	// Verify episode status updated
	var updated db.PodcastItem
	database.First(&updated, "id = ?", item.ID)
	assert.Equal(t, db.Deleted, updated.DownloadStatus, "Status should be updated to Deleted")
}

// TestBackgroundJob_CreateBackup tests database backup creation.
func TestBackgroundJob_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(configDir, 0755)
	os.Setenv("CONFIG", configDir)
	defer os.Setenv("CONFIG", os.Getenv("CONFIG"))

	// Create a file-based database (not in-memory) for backup testing
	dbPath := filepath.Join(configDir, "podgrab.db")
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err, "Should create file-based database")

	// Run migrations
	err = database.AutoMigrate(&db.Podcast{}, &db.PodcastItem{}, &db.Setting{}, &db.Tag{}, &db.Migration{}, &db.JobLock{})
	require.NoError(t, err, "Should run migrations")

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create some data to backup
	db.CreateTestPodcast(t, database)
	db.CreateTestTag(t, database, "Test Tag")

	// Create backup
	_, err = service.CreateBackup()
	require.NoError(t, err, "Should create backup")

	// Verify backup file exists
	backupFiles, err := service.GetAllBackupFiles()
	require.NoError(t, err, "Should list backup files")
	assert.Greater(t, len(backupFiles), 0, "Should have at least one backup file")

	// Verify backup is recent
	if len(backupFiles) > 0 {
		backupPath := backupFiles[0]
		info, err := os.Stat(backupPath)
		require.NoError(t, err, "Backup file should exist")

		// Check file was created recently (within last minute)
		age := time.Since(info.ModTime())
		assert.Less(t, age, time.Minute, "Backup should be recent")
	}
}

// TestBackgroundJob_ConcurrencyLimit tests download concurrency enforcement.
func TestBackgroundJob_ConcurrencyLimit(t *testing.T) {
	database := db.SetupTestDB(t)
	defer db.TeardownTestDB(t, database)

	tmpDir := t.TempDir()
	os.Setenv("DATA", tmpDir)
	defer os.Setenv("DATA", os.Getenv("DATA"))

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Set low concurrency limit
	setting := db.CreateTestSetting(t, database)
	setting.MaxDownloadConcurrency = 1
	database.Save(setting)

	podcast := db.CreateTestPodcast(t, database)

	// Mock slow file server
	slowHandler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow download
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	}
	server := httptest.NewServer(http.HandlerFunc(slowHandler))
	defer server.Close()

	// Create multiple episodes with unique titles
	for i := 0; i < 3; i++ {
		db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
			Title:          fmt.Sprintf("Episode %d", i+1),
			FileURL:        server.URL + fmt.Sprintf("/ep%d.mp3", i),
			DownloadStatus: db.NotDownloaded,
		})
	}

	// Start download job
	start := time.Now()
	err := service.DownloadMissingEpisodes()
	duration := time.Since(start)

	require.NoError(t, err, "Should complete downloads")

	// With concurrency=1 and 3 episodes at 100ms each, should take ~300ms minimum
	// This verifies downloads are serialized, not parallel
	assert.Greater(t, duration, 250*time.Millisecond, "Should enforce concurrency limit")
}
