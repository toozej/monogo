package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPodcastModel tests the Podcast model structure and relationships.
func TestPodcastModel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := &Podcast{
		Title:   "Test Podcast",
		Summary: "Test summary",
		Author:  "Test Author",
		Image:   "https://example.com/image.jpg",
		URL:     "https://example.com/feed.xml",
	}

	err := database.Create(podcast).Error
	require.NoError(t, err, "Should create podcast")
	assert.NotEmpty(t, podcast.ID, "Should have ID")
	assert.NotEmpty(t, podcast.CreatedAt, "Should have CreatedAt")
}

// TestPodcastItemModel tests the PodcastItem model structure.
func TestPodcastItemModel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := CreateTestPodcast(t, database)

	pubDate := time.Now()
	item := &PodcastItem{
		PodcastID:      podcast.ID,
		Title:          "Test Episode",
		Summary:        "Test summary",
		EpisodeType:    "full",
		Duration:       1800,
		PubDate:        pubDate,
		FileURL:        "https://example.com/episode.mp3",
		GUID:           "test-guid",
		Image:          "https://example.com/episode.jpg",
		DownloadStatus: NotDownloaded,
		IsPlayed:       false,
		FileSize:       25000000,
	}

	err := database.Create(item).Error
	require.NoError(t, err, "Should create podcast item")
	assert.NotEmpty(t, item.ID, "Should have ID")
	assert.Equal(t, podcast.ID, item.PodcastID, "Should link to podcast")
}

// TestDownloadStatus tests the DownloadStatus enum.
func TestDownloadStatus(t *testing.T) {
	tests := []struct {
		name   string
		status DownloadStatus
		value  int
	}{
		{"NotDownloaded", NotDownloaded, 0},
		{"Downloading", Downloading, 1},
		{"Downloaded", Downloaded, 2},
		{"Deleted", Deleted, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, int(tt.status), "Should have correct enum value")
		})
	}
}

// TestPodcastRelationships tests podcast-item relationships.
func TestPodcastRelationships(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := CreateTestPodcast(t, database)
	item1 := CreateTestPodcastItem(t, database, podcast.ID)
	item2 := CreateTestPodcastItem(t, database, podcast.ID)

	// Load podcast with items
	var loaded Podcast
	err := database.Preload("PodcastItems").First(&loaded, "id = ?", podcast.ID).Error
	require.NoError(t, err, "Should load podcast with items")

	assert.Len(t, loaded.PodcastItems, 2, "Should have 2 items")

	itemIDs := []string{loaded.PodcastItems[0].ID, loaded.PodcastItems[1].ID}
	assert.Contains(t, itemIDs, item1.ID, "Should contain item1")
	assert.Contains(t, itemIDs, item2.ID, "Should contain item2")
}

// TestPodcastTagRelationships tests many-to-many podcast-tag relationships.
func TestPodcastTagRelationships(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)
	tag1 := CreateTestTag(t, database, "Technology")
	tag2 := CreateTestTag(t, database, "Comedy")

	// Add tags to podcast
	err := AddTagToPodcast(podcast.ID, tag1.ID)
	require.NoError(t, err, "Should add tag1 to podcast")
	err = AddTagToPodcast(podcast.ID, tag2.ID)
	require.NoError(t, err, "Should add tag2 to podcast")

	// Load podcast with tags
	var loaded Podcast
	err = database.Preload("Tags").First(&loaded, "id = ?", podcast.ID).Error
	require.NoError(t, err, "Should load podcast with tags")

	assert.Len(t, loaded.Tags, 2, "Should have 2 tags")

	tagIDs := []string{loaded.Tags[0].ID, loaded.Tags[1].ID}
	assert.Contains(t, tagIDs, tag1.ID, "Should contain tag1")
	assert.Contains(t, tagIDs, tag2.ID, "Should contain tag2")
}

// TestSettingModel tests the Setting model structure.
func TestSettingModel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	setting := &Setting{
		DownloadOnAdd:          true,
		InitialDownloadCount:   5,
		AutoDownload:           true,
		MaxDownloadConcurrency: 10,
		UserAgent:              "TestAgent/1.0",
		BaseURL:                "http://test.local",
	}

	err := database.Create(setting).Error
	require.NoError(t, err, "Should create setting")
	assert.NotEmpty(t, setting.ID, "Should have ID")

	// Verify defaults and values
	assert.True(t, setting.DownloadOnAdd, "Should have DownloadOnAdd")
	assert.Equal(t, 5, setting.InitialDownloadCount, "Should have InitialDownloadCount")
	assert.Equal(t, 10, setting.MaxDownloadConcurrency, "Should have MaxDownloadConcurrency")
}

// TestJobLockModel tests the JobLock model and IsLocked method.
func TestJobLockModel(t *testing.T) {
	tests := []struct {
		lock       *JobLock
		name       string
		wantLocked bool
	}{
		{
			name: "locked_job",
			lock: &JobLock{
				Name:     "test-job",
				Date:     time.Now(),
				Duration: 30,
			},
			wantLocked: true,
		},
		{
			name: "unlocked_job",
			lock: &JobLock{
				Name:     "test-job",
				Date:     time.Time{},
				Duration: 0,
			},
			wantLocked: false,
		},
		{
			name:       "nil_lock",
			lock:       nil,
			wantLocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantLocked, tt.lock.IsLocked(), "IsLocked should return correct value")
		})
	}
}

// TestTagModel tests the Tag model structure.
func TestTagModel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	tag := &Tag{
		Label:       "Technology",
		Description: "Tech-related podcasts",
	}

	err := database.Create(tag).Error
	require.NoError(t, err, "Should create tag")
	assert.NotEmpty(t, tag.ID, "Should have ID")
	assert.Equal(t, "Technology", tag.Label, "Should have label")
}

// TestMigrationModel tests the Migration model structure.
func TestMigrationModel(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	migration := &Migration{
		Name: "2020_11_03_test_migration",
		Date: time.Now(),
	}

	err := database.Create(migration).Error
	require.NoError(t, err, "Should create migration record")
	assert.NotEmpty(t, migration.ID, "Should have ID")
	assert.Equal(t, "2020_11_03_test_migration", migration.Name, "Should have name")
}

// TestPodcastItemDownloadStatusTransitions tests status transitions.
func TestPodcastItemDownloadStatusTransitions(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadStatus: NotDownloaded,
	})

	// Transition: NotDownloaded -> Downloading
	item.DownloadStatus = Downloading
	database.Save(item)

	var retrieved PodcastItem
	database.First(&retrieved, "id = ?", item.ID)
	assert.Equal(t, Downloading, retrieved.DownloadStatus, "Should transition to Downloading")

	// Transition: Downloading -> Downloaded
	item.DownloadStatus = Downloaded
	item.DownloadPath = "/path/to/file.mp3"
	item.DownloadDate = time.Now()
	database.Save(item)

	database.First(&retrieved, "id = ?", item.ID)
	assert.Equal(t, Downloaded, retrieved.DownloadStatus, "Should transition to Downloaded")
	assert.NotEmpty(t, retrieved.DownloadPath, "Should have download path")

	// Transition: Downloaded -> Deleted
	item.DownloadStatus = Deleted
	database.Save(item)

	database.First(&retrieved, "id = ?", item.ID)
	assert.Equal(t, Deleted, retrieved.DownloadStatus, "Should transition to Deleted")
}

// TestPodcastComputedFields tests computed fields (non-persisted).
func TestPodcastComputedFields(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := CreateTestPodcast(t, database)

	// Set computed fields
	podcast.DownloadedEpisodesCount = 10
	podcast.AllEpisodesCount = 25
	podcast.DownloadedEpisodesSize = 100000000

	// Save podcast
	database.Save(podcast)

	// Load podcast again
	var retrieved Podcast
	database.First(&retrieved, "id = ?", podcast.ID)

	// Computed fields should not persist (gorm:"-" tag)
	assert.Equal(t, 0, retrieved.DownloadedEpisodesCount, "Computed field should not persist")
	assert.Equal(t, 0, retrieved.AllEpisodesCount, "Computed field should not persist")
	assert.Equal(t, int64(0), retrieved.DownloadedEpisodesSize, "Computed field should not persist")
}

// TestPodcastLastEpisodeDate tests last episode tracking.
func TestPodcastLastEpisodeDate(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	podcast := CreateTestPodcast(t, database)

	// Initially no last episode
	assert.Nil(t, podcast.LastEpisode, "Should have no last episode initially")

	// Update last episode date
	lastDate := time.Now()
	err := UpdateLastEpisodeDateForPodcast(podcast.ID, lastDate)
	require.NoError(t, err, "Should update last episode date")

	// Verify update
	var retrieved Podcast
	database.First(&retrieved, "id = ?", podcast.ID)
	require.NotNil(t, retrieved.LastEpisode, "Should have last episode date")
	assert.WithinDuration(t, lastDate, *retrieved.LastEpisode, time.Second, "Should have correct date")
}

// TestPodcastIsPaused tests podcast pause functionality.
func TestPodcastIsPaused(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	podcast := CreateTestPodcast(t, database, &Podcast{
		IsPaused: false,
	})

	// Verify default
	assert.False(t, podcast.IsPaused, "Should not be paused by default")

	// Pause podcast
	podcast.IsPaused = true
	database.Save(podcast)

	var retrieved Podcast
	database.First(&retrieved, "id = ?", podcast.ID)
	assert.True(t, retrieved.IsPaused, "Should be paused")
}
