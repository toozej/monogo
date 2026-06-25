// Package controllers provides HTTP request handlers for the podgrab application.
package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toozej/podgrab/db"
	testhelpers "github.com/toozej/podgrab/internal/testing"
	"gorm.io/gorm"
)

// setupTestRouter creates a gin router for controller testing.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	return router
}

// setupTestDBAndEnv sets up both test database and data directory.
func setupTestDBAndEnv(t *testing.T) (database *gorm.DB, dataDir string, cleanup func()) {
	database = testhelpers.SetupTestDB(t)
	dataDir, dataCleanup := testhelpers.SetupTestDataDir(t)

	// Set the global db.DB for the controllers to use
	oldDB := db.DB
	db.DB = database

	cleanup = func() {
		dataCleanup()
		db.DB = oldDB
		testhelpers.TeardownTestDB(t, database)
	}

	return
}

func TestGetPodcastItemFileByID(t *testing.T) {
	database, baseDataDir, cleanup := setupTestDBAndEnv(t)
	defer cleanup()

	router := setupTestRouter()
	router.GET("/podcastitems/:id/file", GetPodcastItemFileByID)

	tests := []struct {
		setup          func(t *testing.T, dataDir string) (episodeID string, podcastDir string)
		wantHeader     map[string]string
		name           string
		wantBody       string
		wantStatusCode int
	}{
		{
			name: "file_exists_at_stored_path",
			setup: func(t *testing.T, dataDir string) (string, string) {
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Test Podcast",
				})

				// Use sanitized path (no spaces)
				podcastDir := filepath.Join(dataDir, "Test-Podcast")
				episodeID := uuid.New().String()
				filePath := filepath.Join(podcastDir, episodeID+".mp3")

				// Create directory and file
				require.NoError(t, os.MkdirAll(podcastDir, 0o755))
				require.NoError(t, os.WriteFile(filePath, []byte("fake audio content"), 0o644))

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Test Episode",
					DownloadPath:   filePath,
					DownloadStatus: db.Downloaded,
				})

				return item.ID, podcastDir
			},
			wantStatusCode: http.StatusOK,
			wantHeader: map[string]string{
				"Content-Type":        "application/octet-stream",
				"Content-Disposition": "attachment; filename=",
			},
		},
		{
			name: "file_not_found_redirects_to_remote_url",
			setup: func(t *testing.T, dataDir string) (string, string) {
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Test Podcast 2",
				})

				nonExistentPath := filepath.Join(dataDir, "NonExistent", "file.mp3")

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Test Episode",
					DownloadPath:   nonExistentPath,
					FileURL:        "https://example.com/remote-file.mp3",
					DownloadStatus: db.Downloaded,
				})

				return item.ID, ""
			},
			wantStatusCode: http.StatusFound, // 302 redirect
			wantHeader: map[string]string{
				"Location": "https://example.com/remote-file.mp3",
			},
		},
		{
			name: "backward_compatibility_finds_file_in_old_location",
			setup: func(t *testing.T, dataDir string) (string, string) {
				// Simulate old naming convention with spaces in folder name
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Old Named Podcast",
				})

				// Create the episode first to get its ID
				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Test Episode",
					DownloadStatus: db.Downloaded,
				})
				episodeID := item.ID

				// Create file in old-style directory (with spaces) using the episode ID
				oldStyleDir := filepath.Join(dataDir, "Old Named Podcast")
				oldStylePath := filepath.Join(oldStyleDir, episodeID+".mp3")
				require.NoError(t, os.MkdirAll(oldStyleDir, 0o755))
				require.NoError(t, os.WriteFile(oldStylePath, []byte("fake audio content"), 0o644))

				// But database has new-style path (no spaces)
				newStylePath := filepath.Join(dataDir, "Old-Named-Podcast", episodeID+".mp3")

				// Update the item with the new-style path
				database.Model(&db.PodcastItem{}).Where("id = ?", episodeID).Update("download_path", newStylePath)

				return episodeID, oldStyleDir
			},
			wantStatusCode: http.StatusOK,
			wantHeader: map[string]string{
				"Content-Type": "application/octet-stream",
			},
		},
		{
			name: "episode_not_found",
			setup: func(t *testing.T, dataDir string) (string, string) {
				return uuid.New().String(), ""
			},
			wantStatusCode: http.StatusNotFound,
			wantBody:       `{"error":"Episode not found"}`,
		},
		{
			name: "file_not_found_no_remote_url",
			setup: func(t *testing.T, dataDir string) (string, string) {
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Test Podcast 3",
				})

				nonExistentPath := filepath.Join(dataDir, "NonExistent", "file.mp3")

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Test Episode",
					DownloadPath:   nonExistentPath,
					DownloadStatus: db.Downloaded,
				})
				// Clear the FileURL which has a default value
				database.Model(&db.PodcastItem{}).Where("id = ?", item.ID).Update("file_url", "")

				return item.ID, ""
			},
			wantStatusCode: http.StatusNotFound,
			wantBody:       `{"error":"File not found"}`,
		},
		{
			name: "invalid_id_format",
			setup: func(t *testing.T, dataDir string) (string, string) {
				// "invalid-id" passes binding validation (it's a non-empty string)
				// but fails to find an episode, resulting in 404
				return "invalid-id", ""
			},
			wantStatusCode: http.StatusNotFound,
			wantBody:       `{"error":"Episode not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique subdirectory for each test to avoid file leakage
			testDataDir := filepath.Join(baseDataDir, uuid.New().String())
			require.NoError(t, os.MkdirAll(testDataDir, 0o755))
			// Temporarily change DATA env var for this test
			oldData := os.Getenv("DATA")
			os.Setenv("DATA", testDataDir)
			defer os.Setenv("DATA", oldData)

			episodeID, _ := tt.setup(t, testDataDir)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/podcastitems/%s/file", episodeID), http.NoBody)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code, "Unexpected status code")

			if tt.wantBody != "" {
				assert.Contains(t, w.Body.String(), tt.wantBody, "Unexpected response body")
			}

			for headerName, expectedValue := range tt.wantHeader {
				actualValue := w.Header().Get(headerName)
				if headerName == "Content-Disposition" {
					assert.Contains(t, actualValue, expectedValue, "Unexpected header %s", headerName)
				} else {
					assert.Equal(t, expectedValue, actualValue, "Unexpected header %s", headerName)
				}
			}
		})
	}
}

func TestFindEpisodeFile(t *testing.T) {
	database, dataDir, cleanup := setupTestDBAndEnv(t)
	defer cleanup()

	tests := []struct {
		setupFiles   func(t *testing.T, dataDir string) (item db.PodcastItem, expectedPath string)
		name         string
		wantContains string
		wantFound    bool
	}{
		{
			name: "file_found_in_podcast_folder",
			setupFiles: func(t *testing.T, dataDir string) (db.PodcastItem, string) {
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Test Podcast",
				})
				podcastDir := filepath.Join(dataDir, "Test-Podcast")
				filePath := filepath.Join(podcastDir, "my-episode.mp3")

				require.NoError(t, os.MkdirAll(podcastDir, 0o755))
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "My Episode",
					DownloadStatus: db.Downloaded,
				})
				// Preload the podcast relationship
				database.Preload("Podcast").First(item, "id = ?", item.ID)

				return *item, filePath
			},
			wantFound:    true,
			wantContains: ".mp3",
		},
		{
			name: "file_found_with_old_style_folder_name",
			setupFiles: func(t *testing.T, dataDir string) (db.PodcastItem, string) {
				// Create podcast with spaces in name (old style)
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Old Style Podcast",
				})
				// Create file in old-style folder (with spaces)
				oldStyleDir := filepath.Join(dataDir, "Old-Style-Podcast")
				filePath := filepath.Join(oldStyleDir, "episode.mp3")

				require.NoError(t, os.MkdirAll(oldStyleDir, 0o755))
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Episode",
					DownloadStatus: db.Downloaded,
				})
				database.Preload("Podcast").First(item, "id = ?", item.ID)

				return *item, filePath
			},
			wantFound:    true,
			wantContains: ".mp3",
		},
		{
			name: "file_found_by_fallback_walk",
			setupFiles: func(t *testing.T, dataDir string) (db.PodcastItem, string) {
				// Create podcast but put file in unexpected location
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Some Podcast",
				})
				// Put file in a different folder
				otherDir := filepath.Join(dataDir, "Other-Folder")
				filePath := filepath.Join(otherDir, "random-episode.mp3")

				require.NoError(t, os.MkdirAll(otherDir, 0o755))
				require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Random Episode",
					DownloadStatus: db.Downloaded,
				})
				database.Preload("Podcast").First(item, "id = ?", item.ID)

				return *item, filePath
			},
			wantFound:    true,
			wantContains: ".mp3",
		},
		{
			name: "file_not_found",
			setupFiles: func(t *testing.T, dataDir string) (db.PodcastItem, string) {
				// Create podcast but no file
				podcast := db.CreateTestPodcast(t, database, &db.Podcast{
					Title: "Empty Podcast",
				})

				item := db.CreateTestPodcastItem(t, database, podcast.ID, &db.PodcastItem{
					Title:          "Missing Episode",
					DownloadStatus: db.Downloaded,
				})
				database.Preload("Podcast").First(item, "id = ?", item.ID)

				return *item, ""
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique subdirectory for each test to avoid file leakage
			testDataDir := filepath.Join(dataDir, uuid.New().String())
			require.NoError(t, os.MkdirAll(testDataDir, 0o755))
			// Temporarily change DATA env var for this test
			oldData := os.Getenv("DATA")
			os.Setenv("DATA", testDataDir)
			defer os.Setenv("DATA", oldData)

			item, expectedPath := tt.setupFiles(t, testDataDir)

			foundPath := findEpisodeFile(&item)

			if tt.wantFound {
				assert.NotEmpty(t, foundPath, "Expected to find file but didn't")
				assert.Contains(t, foundPath, tt.wantContains, "Found path should contain expected string")
				if expectedPath != "" {
					assert.Equal(t, expectedPath, foundPath, "Found path should match expected")
				}
			} else {
				assert.Empty(t, foundPath, "Expected not to find file but did: %s", foundPath)
			}
		})
	}
}

func TestGetFileContentType(t *testing.T) {
	_, dataDir, cleanup := setupTestDBAndEnv(t)
	defer cleanup()

	tests := []struct {
		name         string
		setupFile    func() string
		wantContains string
	}{
		{
			name: "mp3_file",
			setupFile: func() string {
				filePath := filepath.Join(dataDir, "test.mp3")
				// Write MP3 header bytes (ID3 tag)
				content := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
				require.NoError(t, os.WriteFile(filePath, content, 0o644))
				return filePath
			},
			wantContains: "audio",
		},
		{
			name: "text_file",
			setupFile: func() string {
				filePath := filepath.Join(dataDir, "test.txt")
				// Write more than 512 bytes to ensure file.Read() doesn't return io.EOF
				// which would cause GetFileContentType to return application/octet-stream
				content := make([]byte, 600)
				copy(content, "# Text File\nThis is a text file with some content.\n"+
					"Line 1: Some text here\nLine 2: More text here\nLine 3: Even more text\n"+
					"Lorem ipsum dolor sit amet, consectetur adipiscing elit. "+
					"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "+
					"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris. "+
					"Nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in "+
					"reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. "+
					"Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia "+
					"deserunt mollit anim id est laborum. ")
				require.NoError(t, os.WriteFile(filePath, content, 0o644))
				return filePath
			},
			wantContains: "text/plain",
		},
		{
			name: "non_existent_file",
			setupFile: func() string {
				return filepath.Join(dataDir, "does-not-exist.mp3")
			},
			wantContains: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile()
			contentType := GetFileContentType(filePath)
			assert.Contains(t, contentType, tt.wantContains, "Unexpected content type")
		})
	}
}
