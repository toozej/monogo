package testhelpers

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/toozej/podgrab/db"
	applogger "github.com/toozej/podgrab/internal/logger"
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
		&db.Podcast{},
		&db.PodcastItem{},
		&db.Setting{},
		&db.Tag{},
		&db.Migration{},
		&db.JobLock{},
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

// SetupTestDataDir creates a temporary directory for test file operations.
// It sets the DATA environment variable and returns a cleanup function.
func SetupTestDataDir(t *testing.T) (dataDir string, cleanup func()) {
	t.Helper()

	dataDir = t.TempDir()
	oldDataDir := os.Getenv("DATA")
	_ = os.Setenv("DATA", dataDir) // Test helper - error unlikely

	cleanup = func() {
		_ = os.Setenv("DATA", oldDataDir) // Test cleanup - error unlikely
	}

	return dataDir, cleanup
}

// CreateMockRSSHandler creates an HTTP handler that returns RSS feed content.
func CreateMockRSSHandler(rssContent string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		// Test server handler - log error if write fails
		// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
		if _, err := w.Write([]byte(rssContent)); err != nil {
			applogger.Log.Errorw("Error writing RSS response", "error", err)
		}
	})
}

// CreateMockFileHandler creates an HTTP handler that returns file content.
func CreateMockFileHandler(content string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		// Test server handler - log error if write fails
		// nosemgrep: go.lang.security.audit.xss.no-direct-write-to-responsewriter
		if _, err := w.Write([]byte(content)); err != nil {
			applogger.Log.Errorw("Error writing file response", "error", err)
		}
	})
}
