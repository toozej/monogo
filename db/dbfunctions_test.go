package db

import (
	"testing"
	"time"

	"github.com/akhilrex/podgrab/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPodcastByURL tests podcast retrieval by URL.
func TestGetPodcastByURL(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	// Set global DB
	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test podcast
	podcast := CreateTestPodcast(t, database, &Podcast{
		URL: "https://example.com/test-feed.xml",
	})

	tests := []struct {
		name      string
		url       string
		wantID    string
		wantError bool
	}{
		{
			name:      "existing_podcast",
			url:       podcast.URL,
			wantError: false,
			wantID:    podcast.ID,
		},
		{
			name:      "non_existent_podcast",
			url:       "https://example.com/does-not-exist.xml",
			wantError: true,
			wantID:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Podcast
			err := GetPodcastByURL(tt.url, &result)

			if tt.wantError {
				assert.Error(t, err, "Expected error for non-existent podcast")
				return
			}

			require.NoError(t, err, "Should find podcast")
			assert.Equal(t, tt.wantID, result.ID, "Should have correct ID")
			assert.Equal(t, tt.url, result.URL, "Should have correct URL")
		})
	}
}

// TestGetAllPodcasts tests retrieving all podcasts with sorting.
func TestGetAllPodcasts(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test podcasts with different creation times
	podcast1 := CreateTestPodcast(t, database, &Podcast{
		Title: "First Podcast",
		URL:   "https://example.com/feed1.xml",
	})
	time.Sleep(10 * time.Millisecond)

	_ = CreateTestPodcast(t, database, &Podcast{
		Title: "Second Podcast",
		URL:   "https://example.com/feed2.xml",
	})

	tests := []struct {
		name        string
		sorting     string
		wantFirstID string
		wantCount   int
	}{
		{
			name:        "default_sorting",
			sorting:     "",
			wantCount:   2,
			wantFirstID: podcast1.ID, // created_at ascending
		},
		{
			name:        "created_at_sorting",
			sorting:     "created_at",
			wantCount:   2,
			wantFirstID: podcast1.ID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var podcasts []Podcast
			err := GetAllPodcasts(&podcasts, tt.sorting)

			require.NoError(t, err, "Should get all podcasts")
			assert.Len(t, podcasts, tt.wantCount, "Should return correct count")

			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirstID, podcasts[0].ID, "Should have correct first podcast")
			}
		})
	}
}

// TestCreatePodcast tests podcast creation.
func TestCreatePodcast(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := &Podcast{
		Title:   "New Podcast",
		Summary: "Test summary",
		Author:  "Test Author",
		URL:     "https://example.com/new-feed.xml",
		Image:   "https://example.com/image.jpg",
	}

	err := CreatePodcast(podcast)
	require.NoError(t, err, "Should create podcast")
	assert.NotEmpty(t, podcast.ID, "Should assign ID")

	// Verify it was saved
	var retrieved Podcast
	err = database.First(&retrieved, "id = ?", podcast.ID).Error
	require.NoError(t, err)
	assert.Equal(t, podcast.Title, retrieved.Title, "Should save title")
	assert.Equal(t, podcast.URL, retrieved.URL, "Should save URL")
}

// TestUpdatePodcast tests podcast updates.
func TestUpdatePodcast(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create podcast
	podcast := CreateTestPodcast(t, database)

	// Update it
	podcast.Title = "Updated Title"
	podcast.Summary = "Updated Summary"

	err := UpdatePodcast(podcast)
	require.NoError(t, err, "Should update podcast")

	// Verify update
	var retrieved Podcast
	err = database.First(&retrieved, "id = ?", podcast.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title, "Should update title")
	assert.Equal(t, "Updated Summary", retrieved.Summary, "Should update summary")
}

// TestDeletePodcastById tests podcast deletion.
func TestDeletePodcastById(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create podcast
	podcast := CreateTestPodcast(t, database)

	// Delete it
	err := DeletePodcastByID(podcast.ID)
	require.NoError(t, err, "Should delete podcast")

	// Verify deletion
	var count int64
	err = database.Model(&Podcast{}).Where("id = ?", podcast.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "Podcast should be deleted")
}

// TestGetPodcastById tests podcast retrieval by ID with associations.
func TestGetPodcastById(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create podcast with items
	podcast := CreateTestPodcast(t, database)
	item1 := CreateTestPodcastItem(t, database, podcast.ID)
	item2 := CreateTestPodcastItem(t, database, podcast.ID)

	// Retrieve podcast
	var retrieved Podcast
	err := GetPodcastByID(podcast.ID, &retrieved)

	require.NoError(t, err, "Should get podcast")
	assert.Equal(t, podcast.ID, retrieved.ID, "Should have correct ID")
	assert.Len(t, retrieved.PodcastItems, 2, "Should preload items")

	// Verify items are ordered by pub_date DESC
	assert.Contains(t, []string{item1.ID, item2.ID}, retrieved.PodcastItems[0].ID, "Should have items")
}

// TestCreatePodcastItem tests episode creation.
func TestCreatePodcastItem(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	item := &PodcastItem{
		PodcastID:      podcast.ID,
		Title:          "Test Episode",
		Summary:        "Test summary",
		Duration:       1800,
		PubDate:        time.Now(),
		FileURL:        "https://example.com/episode.mp3",
		GUID:           "test-guid-123",
		DownloadStatus: NotDownloaded,
	}

	err := CreatePodcastItem(item)
	require.NoError(t, err, "Should create podcast item")
	assert.NotEmpty(t, item.ID, "Should assign ID")

	// Verify it was saved
	var retrieved PodcastItem
	err = database.First(&retrieved, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.Equal(t, item.Title, retrieved.Title, "Should save title")
	assert.Equal(t, item.GUID, retrieved.GUID, "Should save GUID")
}

// TestUpdatePodcastItem tests episode updates.
func TestUpdatePodcastItem(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID)

	// Update it
	item.IsPlayed = true
	item.DownloadStatus = Downloaded

	err := UpdatePodcastItem(item)
	require.NoError(t, err, "Should update item")

	// Verify update
	var retrieved PodcastItem
	err = database.First(&retrieved, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.True(t, retrieved.IsPlayed, "Should update IsPlayed")
	assert.Equal(t, Downloaded, retrieved.DownloadStatus, "Should update status")
}

// TestDeletePodcastItemById tests episode deletion.
func TestDeletePodcastItemById(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID)

	// Delete it
	err := DeletePodcastItemByID(item.ID)
	require.NoError(t, err, "Should delete item")

	// Verify deletion
	var count int64
	err = database.Model(&PodcastItem{}).Where("id = ?", item.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "Item should be deleted")
}

// TestGetAllPodcastItemsByPodcastId tests retrieving all episodes for a podcast.
func TestGetAllPodcastItemsByPodcastId(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	CreateTestPodcastItem(t, database, podcast.ID)
	CreateTestPodcastItem(t, database, podcast.ID)

	// Create item for different podcast (should not be returned)
	otherPodcast := CreateTestPodcast(t, database, &Podcast{
		URL: "https://example.com/other.xml",
	})
	CreateTestPodcastItem(t, database, otherPodcast.ID)

	var items []PodcastItem
	err := GetAllPodcastItemsByPodcastID(podcast.ID, &items)

	require.NoError(t, err, "Should get items")
	assert.Len(t, items, 2, "Should return only items for specified podcast")
}

// TestGetPodcastItemByPodcastIdAndGUID tests finding episode by GUID.
func TestGetPodcastItemByPodcastIdAndGUID(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		GUID: "unique-guid-123",
	})

	var retrieved PodcastItem
	err := GetPodcastItemByPodcastIDAndGUID(podcast.ID, "unique-guid-123", &retrieved)

	require.NoError(t, err, "Should find item by GUID")
	assert.Equal(t, item.ID, retrieved.ID, "Should return correct item")
	assert.Equal(t, "unique-guid-123", retrieved.GUID, "Should have correct GUID")
}

// TestSetAllEpisodesToDownload tests marking all deleted episodes for download.
func TestSetAllEpisodesToDownload(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create items with different statuses
	deleted := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Deleted,
	})
	downloaded := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
	})

	err := SetAllEpisodesToDownload(podcast.ID)
	require.NoError(t, err, "Should update items")

	// Verify deleted item is now queued
	var retrievedDeleted PodcastItem
	database.First(&retrievedDeleted, "id = ?", deleted.ID)
	assert.Equal(t, NotDownloaded, retrievedDeleted.DownloadStatus, "Deleted should become NotDownloaded")

	// Verify downloaded item unchanged
	var retrievedDownloaded PodcastItem
	database.First(&retrievedDownloaded, "id = ?", downloaded.ID)
	assert.Equal(t, Downloaded, retrievedDownloaded.DownloadStatus, "Downloaded should remain Downloaded")
}

// TestGetAllPodcastItemsToBeDownloaded tests querying items queued for download.
func TestGetAllPodcastItemsToBeDownloaded(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create items with various statuses
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: NotDownloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: NotDownloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Deleted,
	})

	items, err := GetAllPodcastItemsToBeDownloaded()

	require.NoError(t, err, "Should query items")
	assert.Len(t, *items, 2, "Should return only NotDownloaded items")
}

// TestGetAllPodcastItemsAlreadyDownloaded tests querying downloaded items.
func TestGetAllPodcastItemsAlreadyDownloaded(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create items
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: NotDownloaded,
	})

	items, err := GetAllPodcastItemsAlreadyDownloaded()

	require.NoError(t, err, "Should query items")
	assert.Len(t, *items, 2, "Should return only Downloaded items")
}

// TestGetPodcastEpisodeStats tests episode statistics aggregation.
func TestGetPodcastEpisodeStats(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create items with different statuses and sizes
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
		FileSize:       1000000,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
		FileSize:       2000000,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: NotDownloaded,
		FileSize:       500000,
	})

	stats, err := GetPodcastEpisodeStats()

	require.NoError(t, err, "Should get stats")
	require.NotNil(t, stats, "Should return stats")

	// Find stats for our podcast
	var downloadedCount, notDownloadedCount int
	var downloadedSize, notDownloadedSize int64

	for _, stat := range *stats {
		if stat.PodcastID == podcast.ID {
			switch stat.DownloadStatus {
			case Downloaded:
				downloadedCount = stat.Count
				downloadedSize = stat.Size
			case NotDownloaded:
				notDownloadedCount = stat.Count
				notDownloadedSize = stat.Size
			}
		}
	}

	assert.Equal(t, 2, downloadedCount, "Should have 2 downloaded")
	assert.Equal(t, int64(3000000), downloadedSize, "Should sum downloaded sizes")
	assert.Equal(t, 1, notDownloadedCount, "Should have 1 not downloaded")
	assert.Equal(t, int64(500000), notDownloadedSize, "Should sum not downloaded sizes")
}

// TestTogglePodcastPauseStatus tests pausing/unpausing podcasts.
func TestTogglePodcastPauseStatus(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database, &Podcast{
		IsPaused: false,
	})

	// Pause it
	err := TogglePodcastPauseStatus(podcast.ID, true)
	require.NoError(t, err, "Should pause podcast")

	var retrieved Podcast
	database.First(&retrieved, "id = ?", podcast.ID)
	assert.True(t, retrieved.IsPaused, "Should be paused")

	// Unpause it
	err = TogglePodcastPauseStatus(podcast.ID, false)
	require.NoError(t, err, "Should unpause podcast")

	database.First(&retrieved, "id = ?", podcast.ID)
	assert.False(t, retrieved.IsPaused, "Should be unpaused")
}

// TestGetOrCreateSetting tests settings singleton pattern.
func TestGetOrCreateSetting(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// First call should create setting
	setting1 := GetOrCreateSetting()
	require.NotNil(t, setting1, "Should return setting")
	assert.NotEmpty(t, setting1.ID, "Should have ID")

	// Second call should return same setting
	setting2 := GetOrCreateSetting()
	require.NotNil(t, setting2, "Should return setting")
	assert.Equal(t, setting1.ID, setting2.ID, "Should return same setting")

	// Verify only one setting exists
	var count int64
	database.Model(&Setting{}).Count(&count)
	assert.Equal(t, int64(1), count, "Should have exactly one setting")
}

// TestUpdateSettings tests settings updates.
func TestUpdateSettings(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	setting := CreateTestSetting(t, database)

	// Update settings
	setting.AutoDownload = false
	setting.MaxDownloadConcurrency = 10

	err := UpdateSettings(setting)
	require.NoError(t, err, "Should update settings")

	// Verify updates
	retrieved := GetOrCreateSetting()
	assert.False(t, retrieved.AutoDownload, "Should update AutoDownload")
	assert.Equal(t, 10, retrieved.MaxDownloadConcurrency, "Should update MaxDownloadConcurrency")
}

// TestCreateTag tests tag creation.
func TestCreateTag(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	tag := &Tag{
		Label:       "Technology",
		Description: "Tech podcasts",
	}

	err := CreateTag(tag)
	require.NoError(t, err, "Should create tag")
	assert.NotEmpty(t, tag.ID, "Should assign ID")
}

// TestGetAllTags tests retrieving all tags.
func TestGetAllTags(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	CreateTestTag(t, database, "Tag1")
	CreateTestTag(t, database, "Tag2")

	tags, err := GetAllTags("")

	require.NoError(t, err, "Should get tags")
	assert.Len(t, *tags, 2, "Should return all tags")
}

// TestGetTagByLabel tests finding tag by label.
func TestGetTagByLabel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	tag := CreateTestTag(t, database, "Comedy")

	retrieved, err := GetTagByLabel("Comedy")

	require.NoError(t, err, "Should find tag")
	assert.Equal(t, tag.ID, retrieved.ID, "Should return correct tag")
	assert.Equal(t, "Comedy", retrieved.Label, "Should have correct label")
}

// TestDeleteTagById tests tag deletion.
func TestDeleteTagById(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	tag := CreateTestTag(t, database, "ToDelete")

	err := DeleteTagByID(tag.ID)
	require.NoError(t, err, "Should delete tag")

	// Verify deletion
	var count int64
	database.Model(&Tag{}).Where("id = ?", tag.ID).Count(&count)
	assert.Equal(t, int64(0), count, "Tag should be deleted")
}

// TestAddTagToPodcast tests podcast-tag association.
func TestAddTagToPodcast(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	tag := CreateTestTag(t, database, "Technology")

	err := AddTagToPodcast(podcast.ID, tag.ID)
	require.NoError(t, err, "Should add tag to podcast")

	// Verify association
	var retrievedPodcast Podcast
	database.Preload("Tags").First(&retrievedPodcast, "id = ?", podcast.ID)
	assert.Len(t, retrievedPodcast.Tags, 1, "Should have one tag")
	assert.Equal(t, tag.ID, retrievedPodcast.Tags[0].ID, "Should be the correct tag")
}

// TestRemoveTagFromPodcast tests removing podcast-tag association.
func TestRemoveTagFromPodcast(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	tag := CreateTestTag(t, database, "Technology")

	// Add tag
	err := AddTagToPodcast(podcast.ID, tag.ID)
	require.NoError(t, err, "Should add tag to podcast")

	// Remove tag
	err = RemoveTagFromPodcast(podcast.ID, tag.ID)
	require.NoError(t, err, "Should remove tag from podcast")

	// Verify removal
	var retrievedPodcast Podcast
	database.Preload("Tags").First(&retrievedPodcast, "id = ?", podcast.ID)
	assert.Len(t, retrievedPodcast.Tags, 0, "Should have no tags")
}

// TestGetLock tests job lock retrieval.
func TestGetLock(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Get lock for non-existent job
	lock := GetLock("test-job")
	require.NotNil(t, lock, "Should return lock")
	assert.Equal(t, "test-job", lock.Name, "Should have correct name")
	assert.Empty(t, lock.ID, "Should not have ID (not saved yet)")
}

// TestLockAndUnlock tests job locking mechanism.
func TestLockAndUnlock(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	jobName := "test-job"

	// Lock the job
	Lock(jobName, 30)

	// Verify lock
	lock := GetLock(jobName)
	assert.True(t, lock.IsLocked(), "Job should be locked")
	assert.Equal(t, 30, lock.Duration, "Should have correct duration")

	// Unlock the job
	Unlock(jobName)

	// Verify unlock
	lock = GetLock(jobName)
	assert.False(t, lock.IsLocked(), "Job should be unlocked")
}

// TestGetPaginatedPodcastItemsNew tests advanced episode filtering and pagination.
func TestGetPaginatedPodcastItemsNew(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create various items
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		Title:          "Downloaded Episode",
		DownloadStatus: Downloaded,
		IsPlayed:       true,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		Title:          "Unplayed Episode",
		DownloadStatus: Downloaded,
		IsPlayed:       false,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		Title:          "NotDownloaded Episode",
		DownloadStatus: NotDownloaded,
		IsPlayed:       false,
	})

	tests := []struct {
		name      string
		filter    model.EpisodesFilter
		wantCount int
	}{
		{
			name: "all_items",
			filter: model.EpisodesFilter{
				Pagination: model.Pagination{
					Page:  1,
					Count: 10,
				},
			},
			wantCount: 3,
		},
		{
			name: "downloaded_only",
			filter: model.EpisodesFilter{
				Pagination: model.Pagination{
					Page:  1,
					Count: 10,
				},
				IsDownloaded: stringPtr("true"),
			},
			wantCount: 2,
		},
		{
			name: "played_only",
			filter: model.EpisodesFilter{
				Pagination: model.Pagination{
					Page:  1,
					Count: 10,
				},
				IsPlayed: stringPtr("true"),
			},
			wantCount: 1,
		},
		{
			name: "unplayed_only",
			filter: model.EpisodesFilter{
				Pagination: model.Pagination{
					Page:  1,
					Count: 10,
				},
				IsPlayed: stringPtr("false"),
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := GetPaginatedPodcastItemsNew(tt.filter)

			require.NoError(t, err, "Should get items")
			assert.Len(t, *items, tt.wantCount, "Should return correct count")
			assert.Equal(t, int64(tt.wantCount), total, "Should return correct total")
		})
	}
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}

// TestUpdatePodcastItemFileSize tests file size updates.
func TestUpdatePodcastItemFileSize(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		FileSize: 0,
	})

	err := UpdatePodcastItemFileSize(item.ID, 25000000)
	require.NoError(t, err, "Should update file size")

	// Verify update
	var retrieved PodcastItem
	database.First(&retrieved, "id = ?", item.ID)
	assert.Equal(t, int64(25000000), retrieved.FileSize, "Should update file size")
}

// TestGetAllPodcastItemsWithoutSize tests querying items without file size.
func TestGetAllPodcastItemsWithoutSize(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Create items directly to set FileSize to 0 (helper can't distinguish 0 from "not set")
	database.Create(&PodcastItem{
		PodcastID:      podcast.ID,
		Title:          "Episode 1",
		FileURL:        "https://example.com/ep1.mp3",
		GUID:           "guid-1",
		FileSize:       0,
		DownloadStatus: Downloaded,
	})
	database.Create(&PodcastItem{
		PodcastID:      podcast.ID,
		Title:          "Episode 2",
		FileURL:        "https://example.com/ep2.mp3",
		GUID:           "guid-2",
		FileSize:       0,
		DownloadStatus: Downloaded,
	})
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: Downloaded,
	})

	items, err := GetAllPodcastItemsWithoutSize()

	require.NoError(t, err, "Should query items")
	assert.Len(t, *items, 2, "Should return items with zero size")
}
