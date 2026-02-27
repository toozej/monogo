package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB creates an in-memory SQLite database for testing.
// It automatically runs migrations and returns the database connection.
// The database is isolated per test and will be cleaned up automatically.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Create in-memory database with unique name for test isolation
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.New().String())
	database, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Suppress SQL logs in tests
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	err = database.AutoMigrate(
		&Podcast{},
		&PodcastItem{},
		&Setting{},
		&Tag{},
		&Migration{},
		&JobLock{},
	)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return database
}

// TeardownTestDB closes the database connection and cleans up resources.
func TeardownTestDB(t *testing.T, database *gorm.DB) {
	t.Helper()

	sqlDB, err := database.DB()
	if err != nil {
		t.Logf("Warning: failed to get underlying database: %v", err)
		return
	}

	if err := sqlDB.Close(); err != nil {
		t.Logf("Warning: failed to close database: %v", err)
	}
}

// AssertNoPodcastsExist verifies the database has no podcasts.
func AssertNoPodcastsExist(t *testing.T, database *gorm.DB) {
	t.Helper()

	var count int64
	if err := database.Model(&Podcast{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count podcasts: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 podcasts, found %d", count)
	}
}

// AssertPodcastExists verifies a podcast exists with the given URL.
func AssertPodcastExists(t *testing.T, database *gorm.DB, url string) *Podcast {
	t.Helper()

	var podcast Podcast
	err := database.Where("url = ?", url).First(&podcast).Error
	if err != nil {
		t.Fatalf("Expected podcast with URL %s to exist, but got error: %v", url, err)
	}

	return &podcast
}

// AssertPodcastItemCount verifies the expected number of podcast items exist.
func AssertPodcastItemCount(t *testing.T, database *gorm.DB, podcastID string, expectedCount int) {
	t.Helper()

	var count int64
	err := database.Model(&PodcastItem{}).Where("podcast_id = ?", podcastID).Count(&count).Error
	if err != nil {
		t.Fatalf("Failed to count podcast items: %v", err)
	}

	if int(count) != expectedCount {
		t.Errorf("Expected %d podcast items, found %d", expectedCount, count)
	}
}

// CreateTestPodcast creates a test podcast with default values.
func CreateTestPodcast(t *testing.T, database *gorm.DB, overrides ...*Podcast) *Podcast {
	t.Helper()

	podcast := &Podcast{
		Title:   "Test Podcast",
		Summary: "A test podcast for unit testing",
		Author:  "Test Author",
		Image:   "https://example.com/image.jpg",
		URL:     "https://example.com/feed.xml",
	}

	// Apply overrides if provided
	if len(overrides) > 0 && overrides[0] != nil {
		override := overrides[0]
		if override.Title != "" {
			podcast.Title = override.Title
		}
		if override.Summary != "" {
			podcast.Summary = override.Summary
		}
		if override.Author != "" {
			podcast.Author = override.Author
		}
		if override.Image != "" {
			podcast.Image = override.Image
		}
		if override.URL != "" {
			podcast.URL = override.URL
		}
		if override.IsPaused {
			podcast.IsPaused = override.IsPaused
		}
	}

	if err := database.Create(podcast).Error; err != nil {
		t.Fatalf("Failed to create test podcast: %v", err)
	}

	return podcast
}

// CreateTestPodcastItem creates a test podcast item with default values.
func CreateTestPodcastItem(t *testing.T, database *gorm.DB, podcastID string, overrides ...*PodcastItem) *PodcastItem {
	t.Helper()

	pubDate := time.Now().Add(-24 * time.Hour)
	item := &PodcastItem{
		PodcastID:      podcastID,
		Title:          "Test Episode",
		Summary:        "A test episode for unit testing",
		EpisodeType:    "full",
		Duration:       1800, // 30 minutes
		PubDate:        pubDate,
		FileURL:        "https://example.com/episode.mp3",
		GUID:           uuid.New().String(),
		Image:          "https://example.com/episode-image.jpg",
		DownloadStatus: NotDownloaded,
		IsPlayed:       false,
		FileSize:       25000000, // 25MB
	}

	// Apply overrides if provided
	if len(overrides) > 0 && overrides[0] != nil {
		override := overrides[0]
		if override.Title != "" {
			item.Title = override.Title
		}
		if override.Summary != "" {
			item.Summary = override.Summary
		}
		if override.EpisodeType != "" {
			item.EpisodeType = override.EpisodeType
		}
		if override.Duration > 0 {
			item.Duration = override.Duration
		}
		if !override.PubDate.IsZero() {
			item.PubDate = override.PubDate
		}
		if override.FileURL != "" {
			item.FileURL = override.FileURL
		}
		if override.GUID != "" {
			item.GUID = override.GUID
		}
		if override.Image != "" {
			item.Image = override.Image
		}
		if override.DownloadStatus > 0 {
			item.DownloadStatus = override.DownloadStatus
		}
		if override.IsPlayed {
			item.IsPlayed = override.IsPlayed
		}
		if override.FileSize > 0 {
			item.FileSize = override.FileSize
		}
		if override.DownloadPath != "" {
			item.DownloadPath = override.DownloadPath
		}
	}

	if err := database.Create(item).Error; err != nil {
		t.Fatalf("Failed to create test podcast item: %v", err)
	}

	return item
}

// CreateTestTag creates a test tag with default values.
func CreateTestTag(t *testing.T, database *gorm.DB, label string) *Tag {
	t.Helper()

	tag := &Tag{
		Label:       label,
		Description: fmt.Sprintf("Test tag: %s", label),
	}

	if err := database.Create(tag).Error; err != nil {
		t.Fatalf("Failed to create test tag: %v", err)
	}

	return tag
}

// CreateTestSetting creates test settings with default values.
func CreateTestSetting(t *testing.T, database *gorm.DB) *Setting {
	t.Helper()

	setting := &Setting{
		DownloadOnAdd:          true,
		InitialDownloadCount:   5,
		AutoDownload:           true,
		FileNameFormat:         "%EpisodeTitle%",
		DarkMode:               false,
		DownloadEpisodeImages:  false,
		GenerateNFOFile:        false,
		MaxDownloadConcurrency: 5,
		UserAgent:              "Podgrab/Test",
	}

	if err := database.Create(setting).Error; err != nil {
		t.Fatalf("Failed to create test settings: %v", err)
	}

	return setting
}
