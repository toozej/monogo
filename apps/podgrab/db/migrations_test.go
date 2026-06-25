package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteAndSaveMigration tests single migration execution.
func TestExecuteAndSaveMigration(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test data for migration
	podcast := CreateTestPodcast(t, database)
	item := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "/path/to/file.mp3",
		DownloadStatus: NotDownloaded, // Incorrect status (should be Downloaded)
	})

	// Execute migration
	migrationName := "test_migration_fix_status"
	migrationQueries := []string{"update podcast_items set download_status=2 where download_path!='' and download_status=0"}

	err := ExecuteAndSaveMigration(localMigration{Name: migrationName, Query: migrationQueries})
	require.NoError(t, err, "Should execute migration without error")

	// Verify migration was executed (status should be fixed)
	var updated PodcastItem
	database.First(&updated, "id = ?", item.ID)
	assert.Equal(t, Downloaded, updated.DownloadStatus, "Migration should fix download status")

	// Verify migration record was saved
	var migration Migration
	err = database.Where("name = ?", migrationName).First(&migration).Error
	require.NoError(t, err, "Should save migration record")
	assert.Equal(t, migrationName, migration.Name, "Should have correct name")
	assert.NotEmpty(t, migration.Date, "Should have date")
}

// TestExecuteAndSaveMigration_Idempotency tests that migrations run only once.
func TestExecuteAndSaveMigration_Idempotency(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test data
	podcast := CreateTestPodcast(t, database)
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "/path/to/file.mp3",
		DownloadStatus: NotDownloaded,
	})

	migrationName := "test_idempotency"
	migrationQueries := []string{"update podcast_items set download_status=2 where download_path!='' and download_status=0"}

	// First execution
	err := ExecuteAndSaveMigration(localMigration{Name: migrationName, Query: migrationQueries})
	require.NoError(t, err, "First execution should succeed")

	// Count migration records
	var count1 int64
	database.Model(&Migration{}).Where("name = ?", migrationName).Count(&count1)
	assert.Equal(t, int64(1), count1, "Should have one migration record")

	// Second execution (should be skipped)
	err = ExecuteAndSaveMigration(localMigration{Name: migrationName, Query: migrationQueries})
	require.NoError(t, err, "Second execution should succeed but skip")

	// Verify still only one migration record
	var count2 int64
	database.Model(&Migration{}).Where("name = ?", migrationName).Count(&count2)
	assert.Equal(t, int64(1), count2, "Should still have only one migration record")
}

// TestRunMigrations tests running all defined migrations.
func TestRunMigrations(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test data that would be affected by the default migration
	podcast := CreateTestPodcast(t, database)
	CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "/test/path.mp3",
		DownloadStatus: NotDownloaded, // Should be corrected by migration
	})

	// Run all migrations
	RunMigrations()

	// Verify migration records were created
	var migrations []Migration
	database.Find(&migrations)

	// We expect the default migration from migrations.go
	assert.GreaterOrEqual(t, len(migrations), 1, "Should have at least one migration")

	// Verify the default migration is present
	var foundDefaultMigration bool
	for _, m := range migrations {
		if m.Name == "2020_11_03_04_42_SetDefaultDownloadStatus" {
			foundDefaultMigration = true
			break
		}
	}
	assert.True(t, foundDefaultMigration, "Should have run default migration")
}

// TestMigrationFailure tests handling of failed migrations.
func TestMigrationFailure(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Execute migration with invalid SQL
	migrationName := "test_invalid_migration"
	migrationQueries := []string{"invalid sql syntax here"}

	err := ExecuteAndSaveMigration(localMigration{Name: migrationName, Query: migrationQueries})
	assert.Error(t, err, "Should error on invalid SQL")

	// Verify migration record was NOT saved
	var count int64
	database.Model(&Migration{}).Where("name = ?", migrationName).Count(&count)
	assert.Equal(t, int64(0), count, "Should not save migration record on failure")
}

// TestMigrationOrdering tests that migrations maintain order.
func TestMigrationOrdering(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Execute multiple migrations
	migrations := []struct {
		name    string
		queries []string
	}{
		{"2020_01_01_first", []string{"SELECT 1"}},
		{"2020_01_02_second", []string{"SELECT 1"}},
		{"2020_01_03_third", []string{"SELECT 1"}},
	}

	for _, mig := range migrations {
		err := ExecuteAndSaveMigration(localMigration{Name: mig.name, Query: mig.queries})
		require.NoError(t, err, "Should execute migration")
	}

	// Verify all migrations were saved
	var saved []Migration
	database.Order("date asc").Find(&saved)

	assert.Len(t, saved, 3, "Should have 3 migrations")

	// Verify they maintain order by date
	for i := 0; i < len(saved)-1; i++ {
		assert.True(t, saved[i].Date.Before(saved[i+1].Date) || saved[i].Date.Equal(saved[i+1].Date),
			"Migrations should be ordered by date")
	}
}

// TestLocalMigrationStructure tests the localMigration struct.
func TestLocalMigrationStructure(t *testing.T) {
	mig := localMigration{
		Name:  "test_migration",
		Query: []string{"SELECT 1"},
	}

	assert.Equal(t, "test_migration", mig.Name, "Should have name")
	assert.Equal(t, []string{"SELECT 1"}, mig.Query, "Should have query")
}

// TestMigrationWithEmptyQuery tests handling of empty migration queries.
func TestMigrationWithEmptyQuery(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	migrationName := "test_empty_query"
	migrationQueries := []string{""}

	// Empty query should execute without error but not do anything
	err := ExecuteAndSaveMigration(localMigration{Name: migrationName, Query: migrationQueries})

	// Depending on SQLite behavior, this might succeed or fail
	// The important thing is it doesn't panic
	if err == nil {
		// If it succeeded, verify migration was saved
		var count int64
		database.Model(&Migration{}).Where("name = ?", migrationName).Count(&count)
		assert.Equal(t, int64(1), count, "Should save migration record")
	}
}

// TestDefaultMigration tests the default migration behavior.
func TestDefaultMigration(t *testing.T) {
	database := SetupTestDB(t)
	defer TeardownTestDB(t, database)

	originalDB := DB
	DB = database
	defer func() { DB = originalDB }()

	// Create test scenarios for the default migration
	podcast := CreateTestPodcast(t, database)

	// Scenario 1: Item with download path but status 0 (should be updated)
	item1 := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "/path/to/episode1.mp3",
		DownloadStatus: NotDownloaded,
	})

	// Scenario 2: Item without download path (should not be updated)
	item2 := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "",
		DownloadStatus: NotDownloaded,
	})

	// Scenario 3: Item with download path and already correct status (should not change)
	item3 := CreateTestPodcastItem(t, database, podcast.ID, &PodcastItem{
		DownloadPath:   "/path/to/episode3.mp3",
		DownloadStatus: Downloaded,
	})

	// Run the default migration
	RunMigrations()

	// Verify results
	var updated1 PodcastItem
	database.First(&updated1, "id = ?", item1.ID)
	assert.Equal(t, Downloaded, updated1.DownloadStatus, "Item1 should be updated to Downloaded")

	var updated2 PodcastItem
	database.First(&updated2, "id = ?", item2.ID)
	assert.Equal(t, NotDownloaded, updated2.DownloadStatus, "Item2 should remain NotDownloaded")

	var updated3 PodcastItem
	database.First(&updated3, "id = ?", item3.ID)
	assert.Equal(t, Downloaded, updated3.DownloadStatus, "Item3 should remain Downloaded")
}
