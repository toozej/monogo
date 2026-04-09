// migrate_to_fork migrates podcast files from akhilrex/podgrab to toozej/podgrab.
// This script is idempotent - safe to re-run multiple times.
//
// Usage:
//
//	export CONFIG=/path/to/config
//	export DATA=/path/to/assets
//	go run scripts/migrate_to_fork.go
//
// Options:
//
//	--dry-run    Preview changes without executing them
//	--verbose    Enable verbose logging
package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/toozej/podgrab/db"
	"github.com/toozej/podgrab/internal/logger"
	"github.com/toozej/podgrab/internal/sanitize"
	"github.com/toozej/podgrab/service"
)

// MigrationStats tracks the results of the migration.
type MigrationStats struct {
	TotalEpisodes        int
	FilesMoved           int
	FilesAlreadyMigrated int
	FilesNotFound        int
	Errors               int
	SkippedDryRun        int
}

func main() {
	var (
		dryRun  = flag.Bool("dry-run", false, "Preview changes without executing them")
		verbose = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	// Set log level
	if *verbose {
		if err := os.Setenv("LOG_LEVEL", "debug"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set LOG_LEVEL: %v\n", err)
		}
	} else {
		if err := os.Setenv("LOG_LEVEL", "info"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set LOG_LEVEL: %v\n", err)
		}
	}

	fmt.Println("========================================")
	fmt.Println("Podgrab Migration Tool")
	fmt.Println("Migrating from akhilrex/podgrab to toozej/podgrab")
	fmt.Println("========================================")
	fmt.Println()

	if *dryRun {
		fmt.Println("*** DRY RUN MODE - No changes will be made ***")
		fmt.Println()
	}

	// Validate environment
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "."
		fmt.Printf("CONFIG not set, using default: %s\n", configPath)
	}

	dataPath := os.Getenv("DATA")
	if dataPath == "" {
		dataPath = "./assets"
		fmt.Printf("DATA not set, using default: %s\n", dataPath)
	}

	// Validate paths to prevent path traversal
	if err := isSubPath(".", configPath); err != nil {
		logger.Log.Fatalw("Invalid CONFIG path", "error", err)
	}
	if err := isSubPath(".", dataPath); err != nil {
		logger.Log.Fatalw("Invalid DATA path", "error", err)
	}

	// Initialize database
	fmt.Println("Initializing database...")
	if _, err := db.Init(); err != nil {
		logger.Log.Fatalw("Failed to initialize database", "error", err)
	}

	// Create backup before making changes
	if !*dryRun {
		fmt.Println("Creating database backup...")
		backupFile, err := createBackup(configPath)
		if err != nil {
			logger.Log.Fatalw("Failed to create backup", "error", err)
		}
		fmt.Printf("Backup created: %s\n", backupFile)
		fmt.Println()
	}

	// Get current settings
	setting := db.GetOrCreateSetting()
	fmt.Printf("Current filename format: %s\n", setting.FileNameFormat)
	fmt.Println()

	// Run migrations
	stats := migrateEpisodes(dataPath, *dryRun)

	// Print summary
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Migration Summary")
	fmt.Println("========================================")
	fmt.Printf("Total episodes processed: %d\n", stats.TotalEpisodes)
	fmt.Printf("Files moved:              %d\n", stats.FilesMoved)
	fmt.Printf("Files already migrated:   %d\n", stats.FilesAlreadyMigrated)
	fmt.Printf("Files not found:          %d\n", stats.FilesNotFound)
	fmt.Printf("Errors:                   %d\n", stats.Errors)
	if *dryRun {
		fmt.Printf("Would move (dry run):     %d\n", stats.SkippedDryRun)
	}
	fmt.Println()

	if stats.Errors > 0 {
		fmt.Println("WARNING: Some errors occurred during migration. Please review the logs above.")
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("Dry run completed. Run without --dry-run to execute the migration.")
	} else {
		fmt.Println("Migration completed successfully!")
	}
}

// migrateEpisodes migrates all downloaded episodes to the new naming convention.
func migrateEpisodes(dataPath string, dryRun bool) MigrationStats {
	var stats MigrationStats

	// Get all downloaded episodes
	items, err := db.GetAllPodcastItemsAlreadyDownloaded()
	if err != nil {
		logger.Log.Fatalw("Failed to get downloaded episodes", "error", err)
	}

	stats.TotalEpisodes = len(*items)
	if stats.TotalEpisodes == 0 {
		fmt.Println("No downloaded episodes found.")
		return stats
	}

	fmt.Printf("Found %d downloaded episodes to process...\n", stats.TotalEpisodes)
	fmt.Println()

	// Get settings for filename formatting
	setting := db.GetOrCreateSetting()

	for i := range *items {
		item := &(*items)[i]
		if err := migrateEpisode(item, dataPath, setting.FileNameFormat, dryRun, &stats); err != nil {
			logger.Log.Errorw("Failed to migrate episode",
				"episode", item.Title,
				"error", err)
			stats.Errors++
		}

		// Progress indicator every 10 episodes
		if (i+1)%10 == 0 || i == stats.TotalEpisodes-1 {
			fmt.Printf("Progress: %d/%d episodes processed...\n", i+1, stats.TotalEpisodes)
		}
	}

	return stats
}

// migrateEpisode migrates a single episode if needed.
func migrateEpisode(item *db.PodcastItem, dataPath, fileNameFormat string, dryRun bool, stats *MigrationStats) error {
	// Skip if no download path
	if item.DownloadPath == "" {
		return nil
	}

	// Validate paths to prevent path traversal
	if err := isSubPath(dataPath, item.DownloadPath); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	// Get podcast info
	var podcast db.Podcast
	if err := db.GetPodcastByID(item.PodcastID, &podcast); err != nil {
		return fmt.Errorf("failed to get podcast: %w", err)
	}

	// Calculate what the new filename should be
	newFileName := service.FormatFileName(item, fileNameFormat)

	// Extract extension from current path
	ext := filepath.Ext(item.DownloadPath)
	if ext == "" {
		ext = ".mp3" // Default fallback
	}

	// Build new path
	sanitizedPodcastName := sanitize.Name(podcast.Title)
	newPath := path.Join(dataPath, sanitizedPodcastName, newFileName+ext)

	// Validate new path to prevent path traversal
	if err := isSubPath(dataPath, newPath); err != nil {
		return fmt.Errorf("invalid target path: %w", err)
	}

	// Normalize paths for comparison
	oldPathNormalized := filepath.Clean(item.DownloadPath)
	newPathNormalized := filepath.Clean(newPath)

	// Check if already at correct location
	if oldPathNormalized == newPathNormalized {
		logger.Log.Debugw("Episode already at correct location",
			"episode", item.Title,
			"path", newPath)
		stats.FilesAlreadyMigrated++
		return nil
	}

	// Check if old file exists
	if _, err := os.Stat(item.DownloadPath); os.IsNotExist(err) {
		logger.Log.Warnw("File not found at old path, updating database only",
			"episode", item.Title,
			"old_path", item.DownloadPath,
			"new_path", newPath)
		stats.FilesNotFound++

		if !dryRun {
			// Update database to point to new path even if file doesn't exist
			if err := updateEpisodePath(item.ID, newPath); err != nil {
				return fmt.Errorf("failed to update database: %w", err)
			}
		}
		return nil
	}

	// Check if destination already exists (would cause overwrite)
	// #nosec G703 - newPath is validated via isSubPath before this point
	if _, err := os.Stat(newPath); err == nil && oldPathNormalized != newPathNormalized {
		// Destination exists but is different from source
		logger.Log.Warnw("Destination file already exists, skipping to avoid overwrite",
			"episode", item.Title,
			"source", item.DownloadPath,
			"destination", newPath)
		stats.Errors++
		return fmt.Errorf("destination file already exists: %s", newPath)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(newPath)
	if !dryRun {
		// #nosec G703 - destDir is validated via isSubPath before this point
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	logger.Log.Infow("Migrating episode",
		"episode", item.Title,
		"podcast", podcast.Title,
		"old_path", item.DownloadPath,
		"new_path", newPath)

	if dryRun {
		stats.SkippedDryRun++
		return nil
	}

	// Move the file
	// #nosec G703 - both paths validated via isSubPath before this point
	if err := os.Rename(item.DownloadPath, newPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// Update database
	if err := updateEpisodePath(item.ID, newPath); err != nil {
		// Attempt to rollback file move
		logger.Log.Errorw("Failed to update database, attempting rollback",
			"error", err)
		// #nosec G703 - paths validated via isSubPath before this point
		if rollbackErr := os.Rename(newPath, item.DownloadPath); rollbackErr != nil {
			logger.Log.Errorw("CRITICAL: Failed to rollback file move",
				"source", newPath,
				"destination", item.DownloadPath,
				"error", rollbackErr)
		}
		return fmt.Errorf("failed to update database: %w", err)
	}

	stats.FilesMoved++
	return nil
}

// updateEpisodePath updates the download path for an episode in the database.
func updateEpisodePath(episodeID, newPath string) error {
	result := db.DB.Model(&db.PodcastItem{}).
		Where("id = ?", episodeID).
		Update("download_path", newPath)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no rows updated for episode %s", episodeID)
	}

	return nil
}

// createBackup creates a backup of the database before migration.
func createBackup(configPath string) (string, error) {
	timestamp := time.Now().Format("2006.01.02_150405")
	backupFileName := fmt.Sprintf("podgrab_migration_backup_%s.tar.gz", timestamp)

	backupDir := filepath.Join(configPath, "backups")
	// #nosec G703 - backupDir is under configPath which is validated at startup
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupPath := filepath.Join(backupDir, backupFileName)

	// Get database path
	dbPath := filepath.Join(configPath, "podgrab.db")

	// Validate db path to prevent path traversal
	if err := isSubPath(configPath, dbPath); err != nil {
		return "", fmt.Errorf("invalid database path: %w", err)
	}

	// Check if database exists
	// #nosec G703 - dbPath is validated via isSubPath before this point
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", fmt.Errorf("database not found at %s", dbPath)
	}

	// Validate backup path
	if err := isSubPath(configPath, backupPath); err != nil {
		return "", fmt.Errorf("invalid backup path: %w", err)
	}

	// Create tar.gz backup
	if err := createTarGz(backupPath, dbPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// createTarGz creates a tar.gz archive containing the specified file.
func createTarGz(archivePath, filePath string) error {
	// #nosec G703 - archivePath and filePath are validated via isSubPath before this point
	// #nosec G304 - archivePath and filePath are validated via isSubPath before this point
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("could not create archive file: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return addFileToTarWriter(filePath, tarWriter)
}

// addFileToTarWriter adds a file to a tar writer.
func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	// #nosec G703 - filePath is validated via isSubPath before this point
	// #nosec G304 - filePath is validated via isSubPath before this point
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not get stat for file '%s': %w", filePath, err)
	}

	header := &tar.Header{
		Name:    filepath.Base(filePath),
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil { //nolint:gocritic
		return fmt.Errorf("could not write header: %w", err)
	}

	if _, err := io.Copy(tarWriter, file); err != nil { //nolint:gocritic
		return fmt.Errorf("could not copy file data: %w", err)
	}

	return nil
}

// isSubPath checks if the given path is a subpath of the base directory.
// Returns an error if the path attempts to escape the base directory.
func isSubPath(basePath, targetPath string) error {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for base: %w", err)
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target: %w", err)
	}
	absTarget = filepath.Clean(absTarget)

	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
		return fmt.Errorf("path traversal attempt detected: %s is not under %s", targetPath, basePath)
	}
	return nil
}
