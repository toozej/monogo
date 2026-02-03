// Package service implements business logic for podcast management and downloads.
package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akhilrex/podgrab/db"
	testhelpers "github.com/akhilrex/podgrab/internal/testing"
	"github.com/akhilrex/podgrab/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseOpml tests OPML file parsing.
func TestParseOpml(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError bool
		wantCount int
	}{
		{
			name: "valid_opml_single_podcast",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>My Podcasts</title>
  </head>
  <body>
    <outline text="Test Podcast" type="rss" xmlUrl="https://example.com/feed.xml" />
  </body>
</opml>`,
			wantError: false,
			wantCount: 1,
		},
		{
			name: "valid_opml_nested_podcasts",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline text="Category 1">
      <outline text="Podcast 1" type="rss" xmlUrl="https://example.com/feed1.xml" />
      <outline text="Podcast 2" type="rss" xmlUrl="https://example.com/feed2.xml" />
    </outline>
  </body>
</opml>`,
			wantError: false,
			wantCount: 2,
		},
		{
			name:      "invalid_xml",
			content:   "not valid xml",
			wantError: true,
			wantCount: 0,
		},
		{
			name: "empty_opml",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
  </body>
</opml>`,
			wantError: false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOpml(tt.content)

			if tt.wantError {
				assert.Error(t, err, "Expected error parsing OPML")
				return
			}

			require.NoError(t, err, "Should parse OPML without error")

			// Count total outlines (first level + nested)
			totalCount := len(result.Body.Outline)
			for _, outline := range result.Body.Outline {
				totalCount += len(outline.Outline)
			}

			assert.GreaterOrEqual(t, totalCount, tt.wantCount, "Should have expected number of outlines")
		})
	}
}

// TestFetchURL tests RSS feed fetching and parsing.
func TestFetchURL(t *testing.T) {
	tests := []struct {
		name          string
		feedContent   string
		wantTitle     string
		statusCode    int
		wantItemCount int
		wantError     bool
	}{
		{
			name:          "valid_rss_feed",
			feedContent:   testhelpers.ValidRSSFeed,
			statusCode:    http.StatusOK,
			wantError:     false,
			wantTitle:     "Test Podcast",
			wantItemCount: 2,
		},
		{
			name:        "invalid_xml",
			feedContent: testhelpers.InvalidXMLFeed,
			statusCode:  http.StatusOK,
			wantError:   true,
		},
		{
			name:          "empty_feed",
			feedContent:   testhelpers.EmptyRSSFeed,
			statusCode:    http.StatusOK,
			wantError:     false,
			wantTitle:     "Empty Podcast",
			wantItemCount: 0,
		},
		{
			name:          "feed_with_itunes_extensions",
			feedContent:   testhelpers.RSSFeedWithItunesExtensions,
			statusCode:    http.StatusOK,
			wantError:     false,
			wantTitle:     "Advanced Test Podcast",
			wantItemCount: 1,
		},
		{
			name:          "feed_with_special_characters",
			feedContent:   testhelpers.RSSFeedWithSpecialCharacters,
			statusCode:    http.StatusOK,
			wantError:     false,
			wantItemCount: 1,
		},
		{
			name:        "http_error",
			feedContent: "",
			statusCode:  http.StatusInternalServerError,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.feedContent)) // Test server - error handling not required
			}))
			defer server.Close()

			// Fetch URL
			data, body, err := FetchURL(server.URL)

			if tt.wantError {
				assert.Error(t, err, "Expected error fetching URL")
				return
			}

			require.NoError(t, err, "Should fetch URL without error")
			assert.NotNil(t, body, "Should return body bytes")

			if tt.wantTitle != "" {
				assert.Equal(t, tt.wantTitle, data.Channel.Title, "Should parse title correctly")
			}

			assert.Equal(t, tt.wantItemCount, len(data.Channel.Item), "Should parse correct number of items")
		})
	}
}

// TestGetItunesImageUrl tests iTunes image URL extraction.
func TestGetItunesImageUrl(t *testing.T) {
	tests := []struct {
		name    string
		wantURL string
		body    []byte
	}{
		{
			name:    "with_itunes_image",
			body:    []byte(testhelpers.RSSFeedWithItunesExtensions),
			wantURL: "https://example.com/advanced-podcast.jpg",
		},
		{
			name:    "without_itunes_image",
			body:    []byte(testhelpers.EmptyRSSFeed),
			wantURL: "https://example.com/empty-podcast.jpg",
		},
		{
			name:    "invalid_xml",
			body:    []byte("not xml"),
			wantURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := getItunesImageURL(tt.body)

			if tt.wantURL == "" {
				assert.Empty(t, url, "Should return empty URL")
			} else {
				assert.NotEmpty(t, url, "Should extract iTunes image URL")
			}
		})
	}
}

// TestGetAllPodcasts tests podcast retrieval with stats.
func TestGetAllPodcasts(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB for the service functions
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test podcasts
	podcast1 := db.CreateTestPodcast(t, database, &db.Podcast{
		Title:  "Podcast 1",
		Author: "Author 1",
		URL:    "https://example.com/feed1.xml",
	})

	podcast2 := db.CreateTestPodcast(t, database, &db.Podcast{
		Title:  "Podcast 2",
		Author: "Author 2",
		URL:    "https://example.com/feed2.xml",
	})

	// Create episodes for podcasts
	db.CreateTestPodcastItem(t, database, podcast1.ID, &db.PodcastItem{
		Title:          "Episode 1-1",
		DownloadStatus: db.Downloaded,
		FileSize:       1000000,
	})

	db.CreateTestPodcastItem(t, database, podcast1.ID, &db.PodcastItem{
		Title:          "Episode 1-2",
		DownloadStatus: db.NotDownloaded,
		FileSize:       2000000,
	})

	db.CreateTestPodcastItem(t, database, podcast2.ID, &db.PodcastItem{
		Title:          "Episode 2-1",
		DownloadStatus: db.Downloaded,
		FileSize:       3000000,
	})

	// Test retrieval
	podcasts := GetAllPodcasts("")

	require.NotNil(t, podcasts, "Should return podcasts")
	assert.Len(t, *podcasts, 2, "Should return all podcasts")

	// Find podcast1 in results
	var found *db.Podcast
	for i := range *podcasts {
		if (*podcasts)[i].ID == podcast1.ID {
			found = &(*podcasts)[i]
			break
		}
	}

	require.NotNil(t, found, "Should find podcast1 in results")
	assert.Equal(t, 1, found.DownloadedEpisodesCount, "Should have correct downloaded count")
	assert.Equal(t, 1, found.DownloadingEpisodesCount, "Should have correct downloading count")
	assert.Equal(t, 2, found.AllEpisodesCount, "Should have correct total count")
}

// TestGetPodcastPrefix tests filename prefix generation.
func TestGetPodcastPrefix(t *testing.T) {
	pubDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		setting    *db.Setting
		wantPrefix string
	}{
		{
			name: "no_prefix",
			setting: &db.Setting{
				AppendDateToFileName:          false,
				AppendEpisodeNumberToFileName: false,
			},
			wantPrefix: "",
		},
		{
			name: "date_only",
			setting: &db.Setting{
				AppendDateToFileName:          true,
				AppendEpisodeNumberToFileName: false,
			},
			wantPrefix: "2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &db.PodcastItem{
				PodcastID: "test-podcast-id",
				PubDate:   pubDate,
			}

			// Note: Episode number testing requires database setup, so we skip it here
			// and only test date prefixing

			prefix := GetPodcastPrefix(item, tt.setting)

			if tt.wantPrefix == "" {
				assert.Empty(t, prefix, "Should have no prefix")
			} else {
				assert.Contains(t, prefix, tt.wantPrefix, "Should contain expected prefix")
			}
		})
	}
}

// TestUpdateSettings tests settings update.
func TestUpdateSettings(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create initial settings
	db.CreateTestSetting(t, database)

	// Update settings
	err := UpdateSettings(
		false,               // downloadOnAdd
		10,                  // initialDownloadCount
		false,               // autoDownload
		true,                // appendDateToFileName
		true,                // appendEpisodeNumberToFileName
		true,                // darkMode
		true,                // downloadEpisodeImages
		false,               // generateNFOFile
		true,                // dontDownloadDeletedFromDisk
		"http://test.local", // baseURL
		10,                  // maxDownloadConcurrency
		"TestAgent/1.0",     // userAgent
	)

	require.NoError(t, err, "Should update settings without error")

	// Verify settings were updated
	setting := db.GetOrCreateSetting()
	assert.False(t, setting.DownloadOnAdd, "DownloadOnAdd should be updated")
	assert.Equal(t, 10, setting.InitialDownloadCount, "InitialDownloadCount should be updated")
	assert.False(t, setting.AutoDownload, "AutoDownload should be updated")
	assert.True(t, setting.AppendDateToFileName, "AppendDateToFileName should be updated")
	assert.True(t, setting.AppendEpisodeNumberToFileName, "AppendEpisodeNumberToFileName should be updated")
	assert.True(t, setting.DarkMode, "DarkMode should be updated")
	assert.True(t, setting.DownloadEpisodeImages, "DownloadEpisodeImages should be updated")
	assert.Equal(t, "http://test.local", setting.BaseURL, "BaseURL should be updated")
	assert.Equal(t, 10, setting.MaxDownloadConcurrency, "MaxDownloadConcurrency should be updated")
	assert.Equal(t, "TestAgent/1.0", setting.UserAgent, "UserAgent should be updated")
}

// TestSetPodcastItemPlayedStatus tests marking episodes as played/unplayed.
func TestSetPodcastItemPlayedStatus(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		IsPlayed: false,
	})

	// Mark as played
	err := SetPodcastItemPlayedStatus(item.ID, true)
	require.NoError(t, err, "Should mark as played without error")

	// Verify
	var updated db.PodcastItem
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.True(t, updated.IsPlayed, "Should be marked as played")

	// Mark as unplayed
	err = SetPodcastItemPlayedStatus(item.ID, false)
	require.NoError(t, err, "Should mark as unplayed without error")

	// Verify
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.False(t, updated.IsPlayed, "Should be marked as unplayed")
}

// TestSetPodcastItemBookmarkStatus tests bookmarking episodes.
func TestSetPodcastItemBookmarkStatus(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item := db.CreateTestPodcastItem(t, database, podcast.ID)

	// Set bookmark
	err := SetPodcastItemBookmarkStatus(item.ID, true)
	require.NoError(t, err, "Should set bookmark without error")

	// Verify
	var updated db.PodcastItem
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.False(t, updated.BookmarkDate.IsZero(), "Should have bookmark date")

	// Clear bookmark
	err = SetPodcastItemBookmarkStatus(item.ID, false)
	require.NoError(t, err, "Should clear bookmark without error")

	// Verify
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.True(t, updated.BookmarkDate.IsZero(), "Should clear bookmark date")
}

// TestSetPodcastItemAsQueuedForDownload tests queueing episodes for download.
func TestSetPodcastItemAsQueuedForDownload(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		DownloadStatus: db.Deleted,
	})

	// Queue for download
	err := SetPodcastItemAsQueuedForDownload(item.ID)
	require.NoError(t, err, "Should queue for download without error")

	// Verify
	var updated db.PodcastItem
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.Equal(t, db.NotDownloaded, updated.DownloadStatus, "Should be queued for download")
}

// TestAddTag tests tag creation.
func TestAddTag(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create new tag
	tag, err := AddTag("Comedy", "Comedy podcasts")
	require.NoError(t, err, "Should create tag without error")
	assert.Equal(t, "Comedy", tag.Label, "Should have correct label")
	assert.Equal(t, "Comedy podcasts", tag.Description, "Should have correct description")

	// Try to create duplicate
	_, err = AddTag("Comedy", "Different description")
	assert.Error(t, err, "Should error on duplicate tag")
}

// TestTogglePodcastPause tests pausing/unpausing podcasts.
func TestTogglePodcastPause(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test podcast
	podcast := db.CreateTestPodcast(t, database, &db.Podcast{
		IsPaused: false,
	})

	// Pause podcast
	err := TogglePodcastPause(podcast.ID, true)
	require.NoError(t, err, "Should pause podcast without error")

	// Verify
	var updated db.Podcast
	err = database.First(&updated, "id = ?", podcast.ID).Error
	require.NoError(t, err)
	assert.True(t, updated.IsPaused, "Should be paused")

	// Unpause podcast
	err = TogglePodcastPause(podcast.ID, false)
	require.NoError(t, err, "Should unpause podcast without error")

	// Verify
	err = database.First(&updated, "id = ?", podcast.ID).Error
	require.NoError(t, err)
	assert.False(t, updated.IsPaused, "Should be unpaused")
}

// TestDeleteTag tests tag deletion.
func TestDeleteTag(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create tag
	tag := db.CreateTestTag(t, database, "TestTag")

	// Delete tag
	err := DeleteTag(tag.ID)
	require.NoError(t, err, "Should delete tag without error")

	// Verify deletion
	var count int64
	err = database.Model(&db.Tag{}).Where("id = ?", tag.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "Tag should be deleted")
}

// TestGetPodcastById tests podcast retrieval by ID.
func TestGetPodcastById(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test podcast
	podcast := db.CreateTestPodcast(t, database)

	// Retrieve podcast
	retrieved := GetPodcastByID(podcast.ID)
	require.NotNil(t, retrieved, "Should retrieve podcast")
	assert.Equal(t, podcast.ID, retrieved.ID, "Should have correct ID")
	assert.Equal(t, podcast.Title, retrieved.Title, "Should have correct title")
}

// TestGetPodcastItemById tests episode retrieval by ID.
func TestGetPodcastItemById(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item := db.CreateTestPodcastItem(t, database, podcast.ID)

	// Retrieve episode
	retrieved := GetPodcastItemByID(item.ID)
	require.NotNil(t, retrieved, "Should retrieve episode")
	assert.Equal(t, item.ID, retrieved.ID, "Should have correct ID")
	assert.Equal(t, item.Title, retrieved.Title, "Should have correct title")
}

// TestMakeQuery tests HTTP request making (network error cases).
func TestMakeQuery_NetworkError(t *testing.T) {
	// Test with invalid URL
	_, err := makeQuery("http://invalid-domain-that-does-not-exist.local")
	assert.Error(t, err, "Should error on network failure")
}

// TestExportOmpl tests OPML export functionality.
func TestExportOmpl(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test podcasts
	db.CreateTestPodcast(t, database, &db.Podcast{
		Title:   "Podcast 1",
		Summary: "First podcast",
		URL:     "https://example.com/feed1.xml",
	})

	db.CreateTestPodcast(t, database, &db.Podcast{
		Title:   "Podcast 2",
		Summary: "Second podcast",
		URL:     "https://example.com/feed2.xml",
	})

	tests := []struct {
		name           string
		baseURL        string
		usePodgrabLink bool
	}{
		{
			name:           "export_with_original_urls",
			usePodgrabLink: false,
			baseURL:        "",
		},
		{
			name:           "export_with_podgrab_links",
			usePodgrabLink: true,
			baseURL:        "http://podgrab.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ExportOmpl(tt.usePodgrabLink, tt.baseURL)
			require.NoError(t, err, "Should export OPML without error")
			assert.NotEmpty(t, data, "Should return OPML data")

			// Verify it's valid XML
			assert.Contains(t, string(data), "<?xml version", "Should contain XML declaration")
			assert.Contains(t, string(data), "<opml", "Should contain OPML tag")
			assert.Contains(t, string(data), "Podcast 1", "Should contain podcast title")
			assert.Contains(t, string(data), "Podcast 2", "Should contain podcast title")

			if tt.usePodgrabLink {
				assert.Contains(t, string(data), tt.baseURL, "Should use Podgrab base URL")
			} else {
				assert.Contains(t, string(data), "https://example.com/feed1.xml", "Should use original URL")
			}
		})
	}
}

// TestSetPodcastItemAsNotDownloaded tests marking episodes as not downloaded.
func TestSetPodcastItemAsNotDownloaded(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
		DownloadStatus: db.Downloaded,
		DownloadPath:   "/path/to/episode.mp3",
		DownloadDate:   time.Now(),
	})

	// Mark as not downloaded
	err := SetPodcastItemAsNotDownloaded(item.ID, db.Deleted)
	require.NoError(t, err, "Should mark as not downloaded without error")

	// Verify
	var updated db.PodcastItem
	err = database.First(&updated, "id = ?", item.ID).Error
	require.NoError(t, err)
	assert.Equal(t, db.Deleted, updated.DownloadStatus, "Should have correct status")
	assert.Empty(t, updated.DownloadPath, "Should clear download path")
	assert.True(t, updated.DownloadDate.IsZero(), "Should clear download date")
}

// TestGetSearchFromItunes tests iTunes search result conversion.
func TestGetSearchFromItunes(t *testing.T) {
	itunesResult := model.ItunesSingleResult{
		TrackName:     "Test Podcast",
		FeedURL:       "https://example.com/feed.xml",
		ArtworkURL600: "https://example.com/artwork.jpg",
	}

	result := GetSearchFromItunes(itunesResult)

	require.NotNil(t, result, "Should convert iTunes result")
	assert.Equal(t, "Test Podcast", result.Title, "Should have correct title")
	assert.Equal(t, "https://example.com/feed.xml", result.URL, "Should have correct URL")
	assert.Equal(t, "https://example.com/artwork.jpg", result.Image, "Should have correct image")
}

// TestGetSearchFromGpodder tests GPodder search result conversion.
func TestGetSearchFromGpodder(t *testing.T) {
	gpodderResult := model.GPodcast{
		Title:       "Test Podcast",
		URL:         "https://example.com/feed.xml",
		LogoURL:     "https://example.com/logo.jpg",
		Description: "Test description",
	}

	result := GetSearchFromGpodder(gpodderResult)

	require.NotNil(t, result, "Should convert GPodder result")
	assert.Equal(t, "Test Podcast", result.Title, "Should have correct title")
	assert.Equal(t, "https://example.com/feed.xml", result.URL, "Should have correct URL")
	assert.Equal(t, "https://example.com/logo.jpg", result.Image, "Should have correct image")
	assert.Equal(t, "Test description", result.Description, "Should have correct description")
}

// TestAddPodcast_ErrorCases tests error handling in AddPodcast.
func TestAddPodcast_NetworkError(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings
	db.CreateTestSetting(t, database)

	// Try to add podcast with invalid URL
	_, err := AddPodcast("http://invalid-domain-that-does-not-exist.local/feed.xml")
	assert.Error(t, err, "Should error on network failure")
}

// TestAddPodcast_DuplicateURL tests duplicate podcast handling.
func TestAddPodcast_DuplicateURL(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings and existing podcast
	db.CreateTestSetting(t, database)
	existingURL := "https://example.com/existing-feed.xml"
	db.CreateTestPodcast(t, database, &db.Podcast{
		URL: existingURL,
	})

	// Try to add duplicate
	_, err := AddPodcast(existingURL)
	assert.Error(t, err, "Should error on duplicate URL")

	// Verify it's the correct error type
	var podcastErr *model.PodcastAlreadyExistsError
	assert.True(t, errors.As(err, &podcastErr), "Should be PodcastAlreadyExistsError")
}

// TestGetAllPodcastItemsByIds tests batch episode retrieval.
func TestGetAllPodcastItemsByIds(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast := db.CreateTestPodcast(t, database)
	item1 := db.CreateTestPodcastItem(t, database, podcast.ID)
	item2 := db.CreateTestPodcastItem(t, database, podcast.ID)

	// Retrieve by IDs
	items, err := GetAllPodcastItemsByIDs([]string{item1.ID, item2.ID})
	require.NoError(t, err, "Should retrieve items without error")
	require.NotNil(t, items, "Should return items")
	assert.Len(t, *items, 2, "Should return both items")
}

// TestGetAllPodcastItemsByPodcastIds tests batch episode retrieval by podcast IDs.
func TestGetAllPodcastItemsByPodcastIds(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test data
	podcast1 := db.CreateTestPodcast(t, database)
	podcast2 := db.CreateTestPodcast(t, database, &db.Podcast{
		Title: "Podcast 2",
		URL:   "https://example.com/feed2.xml",
	})

	db.CreateTestPodcastItem(t, database, podcast1.ID)
	db.CreateTestPodcastItem(t, database, podcast1.ID)
	db.CreateTestPodcastItem(t, database, podcast2.ID)

	// Retrieve by podcast IDs
	items := GetAllPodcastItemsByPodcastIDs([]string{podcast1.ID, podcast2.ID})
	require.NotNil(t, items, "Should return items")
	assert.Len(t, *items, 3, "Should return all items from both podcasts")
}

// TestGetTagsByIds tests batch tag retrieval.
func TestGetTagsByIds(t *testing.T) {
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	// Set the global DB
	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create test tags
	tag1 := db.CreateTestTag(t, database, "Tag1")
	tag2 := db.CreateTestTag(t, database, "Tag2")

	// Retrieve by IDs
	tags := GetTagsByIDs([]string{tag1.ID, tag2.ID})
	require.NotNil(t, tags, "Should return tags")
	assert.Len(t, *tags, 2, "Should return both tags")
}
