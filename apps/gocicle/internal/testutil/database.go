// Package testutil creates isolated PostgreSQL schemas for acceptance tests.
package testutil

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
)

func Database(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GOCICLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set GOCICLE_TEST_DATABASE_URL for PostgreSQL acceptance tests")
	}
	root, err := storage.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(storage.ID(), "-", "")
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := storage.Open(dsn + separator + "search_path=" + schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
		_ = root.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		sqlRoot, _ := root.DB()
		_ = sqlRoot.Close()
	})
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
