// Package service implements business logic for podcast management and downloads.
package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/akhilrex/podgrab/db"
	testhelpers "github.com/akhilrex/podgrab/internal/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetFileName tests filename generation and sanitization.
func TestGetFileName(t *testing.T) {
	tests := []struct {
		name             string
		link             string
		title            string
		defaultExtension string
		wantContains     string
		wantExtension    string
	}{
		{
			name:             "simple_title",
			link:             "https://example.com/episode.mp3",
			title:            "My Episode",
			defaultExtension: ".mp3",
			wantContains:     "My",
			wantExtension:    ".mp3",
		},
		{
			name:             "title_with_special_chars",
			link:             "https://example.com/file",
			title:            "Episode: The \"Best\" One!",
			defaultExtension: ".mp3",
			wantContains:     "Episode",
			wantExtension:    ".mp3",
		},
		{
			name:             "url_with_extension",
			link:             "https://example.com/audio.m4a",
			title:            "Test Episode",
			defaultExtension: ".mp3",
			wantContains:     "Test",
			wantExtension:    ".m4a",
		},
		{
			name:             "url_without_extension",
			link:             "https://example.com/download",
			title:            "Episode Title",
			defaultExtension: ".mp3",
			wantContains:     "Episode",
			wantExtension:    ".mp3",
		},
		{
			name:             "unicode_title",
			link:             "https://example.com/file.mp3",
			title:            "Épisode spécial",
			defaultExtension: ".mp3",
			wantContains:     "pisode",
			wantExtension:    ".mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := getFileName(tt.link, tt.title, tt.defaultExtension)

			assert.NotEmpty(t, fileName, "Should return non-empty filename")
			assert.Contains(t, fileName, tt.wantContains, "Should contain part of title")
			assert.True(t, filepath.Ext(fileName) == tt.wantExtension, "Should have correct extension")

			// Verify filename is safe (no directory traversal)
			assert.NotContains(t, fileName, "..", "Should not contain directory traversal")
			assert.NotContains(t, fileName, "/", "Should not contain path separators")
			assert.NotContains(t, fileName, "\\", "Should not contain Windows path separators")
		})
	}
}

// TestCleanFileName tests filename sanitization.
func TestCleanFileName(t *testing.T) {
	tests := []struct {
		name     string
		original string
		wantSafe bool
	}{
		{
			name:     "simple_name",
			original: "Simple Name",
			wantSafe: true,
		},
		{
			name:     "special_characters",
			original: "Name: With <Special> Characters!",
			wantSafe: true,
		},
		{
			name:     "path_traversal",
			original: "../../../etc/passwd",
			wantSafe: true,
		},
		{
			name:     "unicode",
			original: "日本語のファイル",
			wantSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := cleanFileName(tt.original)

			if tt.wantSafe {
				assert.NotContains(t, cleaned, "..", "Should remove directory traversal")
				assert.NotContains(t, cleaned, "<", "Should remove angle brackets")
				assert.NotContains(t, cleaned, ">", "Should remove angle brackets")
			}
		})
	}
}

// TestFileExists tests file existence checking.
func TestFileExists(t *testing.T) {
	dataDir, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Create a test file
	testFile := filepath.Join(dataDir, "test-file.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0600)
	require.NoError(t, err)

	tests := []struct {
		name       string
		filePath   string
		wantExists bool
	}{
		{
			name:       "existing_file",
			filePath:   testFile,
			wantExists: true,
		},
		{
			name:       "non_existent_file",
			filePath:   filepath.Join(dataDir, "does-not-exist.txt"),
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := FileExists(tt.filePath)
			assert.Equal(t, tt.wantExists, exists, "Should correctly detect file existence")
		})
	}
}

// TestDeleteFile tests file deletion.
func TestDeleteFile(t *testing.T) {
	dataDir, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Create a test file
	testFile := filepath.Join(dataDir, "test-delete.txt")
	err := os.WriteFile(testFile, []byte("to be deleted"), 0600)
	require.NoError(t, err)

	// Delete the file
	err = DeleteFile(testFile)
	assert.NoError(t, err, "Should delete file without error")

	// Verify deletion
	assert.False(t, FileExists(testFile), "File should no longer exist")

	// Try deleting non-existent file
	err = DeleteFile(testFile)
	assert.Error(t, err, "Should error when deleting non-existent file")
	assert.True(t, os.IsNotExist(err), "Should return os.ErrNotExist")
}

// TestGetFileSize tests file size retrieval.
func TestGetFileSize(t *testing.T) {
	dataDir, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Create test file with known size
	testContent := []byte("Hello, this is test content!")
	testFile := filepath.Join(dataDir, "test-size.txt")
	err := os.WriteFile(testFile, testContent, 0600)
	require.NoError(t, err)

	// Get file size
	size, err := GetFileSize(testFile)
	require.NoError(t, err, "Should get file size without error")
	assert.Equal(t, int64(len(testContent)), size, "Should return correct file size")

	// Test non-existent file
	_, err = GetFileSize(filepath.Join(dataDir, "does-not-exist.txt"))
	assert.Error(t, err, "Should error for non-existent file")
}

// TestGetFileSizeFromUrl tests HTTP HEAD request for file size.
func TestGetFileSizeFromUrl(t *testing.T) {
	tests := []struct {
		name       string
		size       string
		statusCode int
		wantSize   int64
		wantError  bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			size:       "25000000",
			wantError:  false,
			wantSize:   25000000,
		},
		{
			name:       "not_found",
			statusCode: http.StatusNotFound,
			size:       "0",
			wantError:  true,
		},
		{
			name:       "missing_content_length",
			statusCode: http.StatusOK,
			size:       "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "HEAD" {
					t.Errorf("Expected HEAD request, got %s", r.Method)
				}

				w.Header().Set("Content-Length", tt.size)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Test
			size, err := GetFileSizeFromURL(server.URL)

			if tt.wantError {
				assert.Error(t, err, "Expected error")
				return
			}

			require.NoError(t, err, "Should get file size without error")
			assert.Equal(t, tt.wantSize, size, "Should return correct size")
		})
	}
}

// TestCreateDataFolderIfNotExists tests podcast folder creation.
func TestCreateDataFolderIfNotExists(t *testing.T) {
	dataDir, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	tests := []struct {
		name        string
		folderName  string
		wantCreated bool
	}{
		{
			name:        "simple_folder",
			folderName:  "Test Podcast",
			wantCreated: true,
		},
		{
			name:        "folder_with_special_chars",
			folderName:  "Podcast: The Best One!",
			wantCreated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folderPath := createDataFolderIfNotExists(tt.folderName)

			require.NotEmpty(t, folderPath, "Should return folder path")
			assert.DirExists(t, folderPath, "Should create folder")
			assert.Contains(t, folderPath, dataDir, "Should be in data directory")
		})
	}
}

// TestDownload tests episode download with HTTP mocking.
func TestDownload(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Set up database
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings
	db.CreateTestSetting(t, database)

	tests := []struct {
		name         string
		episodeTitle string
		podcastName  string
		prefix       string
		content      []byte
		statusCode   int
		wantError    bool
	}{
		{
			name:         "successful_download",
			content:      []byte("fake mp3 content"),
			statusCode:   http.StatusOK,
			episodeTitle: "Test Episode",
			podcastName:  "Test Podcast",
			prefix:       "",
			wantError:    false,
		},
		{
			name:         "download_with_prefix",
			content:      []byte("fake mp3 content"),
			statusCode:   http.StatusOK,
			episodeTitle: "Episode 2",
			podcastName:  "Test Podcast",
			prefix:       "2024-01-15",
			wantError:    false,
		},
		{
			name:         "http_error",
			content:      []byte{},
			statusCode:   http.StatusInternalServerError,
			episodeTitle: "Failed Episode",
			podcastName:  "Test Podcast",
			prefix:       "",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(tt.content) // Test server - error handling not required
			}))
			defer server.Close()

			// Download
			filePath, err := Download(server.URL, tt.episodeTitle, tt.podcastName, tt.prefix)

			if tt.wantError {
				assert.Error(t, err, "Expected error on failed download")
				return
			}

			require.NoError(t, err, "Should download without error")
			assert.NotEmpty(t, filePath, "Should return file path")
			assert.FileExists(t, filePath, "Should create file")

			// Verify content
			content, err := os.ReadFile(filePath) // nolint:gosec // Test code with controlled file path
			require.NoError(t, err)
			assert.Equal(t, tt.content, content, "Should save correct content")

			// Verify prefix in filename if provided
			if tt.prefix != "" {
				fileName := filepath.Base(filePath)
				assert.Contains(t, fileName, tt.prefix, "Should include prefix in filename")
			}
		})
	}
}

// TestDownload_EmptyLink tests error handling for empty download link.
func TestDownload_EmptyLink(t *testing.T) {
	_, err := Download("", "Episode", "Podcast", "")
	assert.Error(t, err, "Should error on empty link")
	assert.Contains(t, err.Error(), "empty", "Error should mention empty path")
}

// TestDownload_ExistingFile tests skipping download if file exists.
func TestDownload_ExistingFile(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Set up database
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content")) // Test server - error handling not required
	}))
	defer server.Close()

	// First download
	filePath1, err := Download(server.URL, "Episode", "Podcast", "")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "Should make HTTP request on first download")

	// Second download (should skip because file exists)
	filePath2, err := Download(server.URL, "Episode", "Podcast", "")
	require.NoError(t, err)
	assert.Equal(t, filePath1, filePath2, "Should return same path")
	assert.Equal(t, 1, callCount, "Should not make HTTP request for existing file")
}

// TestDownloadPodcastCoverImage tests podcast image download.
func TestDownloadPodcastCoverImage(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Set up database
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Create test server
	imageContent := []byte("fake image data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imageContent) // Test server - error handling not required
	}))
	defer server.Close()

	// Download image
	imagePath, err := DownloadPodcastCoverImage(server.URL, "Test Podcast")
	require.NoError(t, err, "Should download image without error")
	assert.NotEmpty(t, imagePath, "Should return image path")
	assert.FileExists(t, imagePath, "Should create image file")

	// Verify content
	content, err := os.ReadFile(imagePath) // nolint:gosec // Test code with controlled file path
	require.NoError(t, err)
	assert.Equal(t, imageContent, content, "Should save correct image data")
}

// TestDownloadPodcastCoverImage_EmptyLink tests error handling.
func TestDownloadPodcastCoverImage_EmptyLink(t *testing.T) {
	_, err := DownloadPodcastCoverImage("", "Podcast")
	assert.Error(t, err, "Should error on empty link")
}

// TestDownloadImage tests episode image download.
func TestDownloadImage(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Set up database
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	db.CreateTestSetting(t, database)

	// Create test server
	imageContent := []byte("episode image data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imageContent) // Test server - error handling not required
	}))
	defer server.Close()

	// Download episode image
	imagePath, err := DownloadImage(server.URL, "episode-id-123", "Test Podcast")
	require.NoError(t, err, "Should download image without error")
	assert.NotEmpty(t, imagePath, "Should return image path")
	assert.FileExists(t, imagePath, "Should create image file")

	// Verify it's in an images subdirectory
	assert.Contains(t, imagePath, "images", "Should be in images folder")

	// Verify content
	content, err := os.ReadFile(imagePath) // nolint:gosec // Test code with controlled file path
	require.NoError(t, err)
	assert.Equal(t, imageContent, content, "Should save correct image data")
}

// TestCreateNfoFile tests NFO file generation for media centers.
func TestCreateNfoFile(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	podcast := &db.Podcast{
		Title: "Test Podcast",
		Image: "https://example.com/podcast-art.jpg",
	}

	err := CreateNfoFile(podcast)
	require.NoError(t, err, "Should create NFO file without error")

	// Verify file was created
	nfoPath := path.Join(createDataFolderIfNotExists(podcast.Title), "album.nfo")
	assert.FileExists(t, nfoPath, "Should create album.nfo file")

	// Verify content
	content, err := os.ReadFile(nfoPath) // nolint:gosec // Test code with controlled file path
	require.NoError(t, err)

	assert.Contains(t, string(content), "<?xml version", "Should be valid XML")
	assert.Contains(t, string(content), "<album>", "Should have album tag")
	assert.Contains(t, string(content), podcast.Title, "Should contain podcast title")
	assert.Contains(t, string(content), podcast.Image, "Should contain image URL")
	assert.Contains(t, string(content), "Broadcast", "Should have type Broadcast")
}

// TestGetPodcastLocalImagePath tests image path generation.
func TestGetPodcastLocalImagePath(t *testing.T) {
	dataDir, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Now using dataDir

	imagePath := GetPodcastLocalImagePath("https://example.com/image.jpg", "Test Podcast")

	assert.NotEmpty(t, imagePath, "Should return image path")
	assert.Contains(t, imagePath, dataDir, "Should be in data directory")
	assert.Contains(t, imagePath, ".jpg", "Should have jpg extension")
}

// TestDeletePodcastFolder tests podcast folder deletion.
func TestDeletePodcastFolder(t *testing.T) {
	_, cleanup := testhelpers.SetupTestDataDir(t)
	defer cleanup()

	// Create podcast folder with files
	podcastName := "Test Podcast To Delete"
	folderPath := createDataFolderIfNotExists(podcastName)

	testFile := filepath.Join(folderPath, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Delete folder
	err = deletePodcastFolder(podcastName)
	require.NoError(t, err, "Should delete folder without error")

	// Verify deletion
	assert.NoDirExists(t, folderPath, "Folder should be deleted")
	assert.NoFileExists(t, testFile, "Files should be deleted")
}

// TestHttpClient tests HTTP client configuration.
func TestHttpClient(t *testing.T) {
	client := httpClient()
	require.NotNil(t, client, "Should create HTTP client")

	// Verify it handles redirects
	assert.NotNil(t, client.CheckRedirect, "Should have redirect handler")
}

// TestGetRequest tests HTTP request creation with user agent.
func TestGetRequest(t *testing.T) {
	// Set up database
	database := testhelpers.SetupTestDB(t)
	defer testhelpers.TeardownTestDB(t, database)

	originalDB := db.DB
	db.DB = database
	defer func() { db.DB = originalDB }()

	// Create settings with custom user agent
	setting := db.CreateTestSetting(t, database)
	setting.UserAgent = "CustomAgent/1.0"
	err := db.UpdateSettings(setting)
	require.NoError(t, err, "Should update settings")

	// Create request
	req, err := getRequest("https://example.com/feed.xml")
	require.NoError(t, err, "Should create request without error")
	assert.NotNil(t, req, "Should return request")

	// Verify user agent is set
	userAgent := req.Header.Get("User-Agent")
	assert.Equal(t, "CustomAgent/1.0", userAgent, "Should set custom user agent")
}

// TestGetAllBackupFiles tests backup file listing.
func TestGetAllBackupFiles(t *testing.T) {
	// Set up config directory
	configDir := t.TempDir()
	oldConfigDir := os.Getenv("CONFIG")
	_ = os.Setenv("CONFIG", configDir) // Test setup - error unlikely
	defer func() { _ = os.Setenv("CONFIG", oldConfigDir) }()

	// Create backup folder and files
	backupFolder := filepath.Join(configDir, "backups")
	err := os.MkdirAll(backupFolder, 0755) // nolint:gosec // Test directory - 0755 is appropriate
	require.NoError(t, err, "Should create backup folder")

	// Create test backup files
	backupFiles := []string{
		"podgrab_backup_2024.01.01_100000.tar.gz",
		"podgrab_backup_2024.01.02_100000.tar.gz",
		"podgrab_backup_2024.01.03_100000.tar.gz",
	}

	for _, file := range backupFiles {
		filePath := filepath.Join(backupFolder, file)
		err := os.WriteFile(filePath, []byte("backup"), 0600)
		require.NoError(t, err)
	}

	// Get backup files
	files, err := GetAllBackupFiles()
	require.NoError(t, err, "Should list backup files without error")
	assert.Len(t, files, 3, "Should find all backup files")

	// Verify files are sorted in reverse order (newest first)
	for i := 0; i < len(files)-1; i++ {
		assert.True(t, files[i] > files[i+1], "Files should be sorted in reverse order")
	}
}
