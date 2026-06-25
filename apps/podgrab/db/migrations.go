// Package db provides database models and data access functions.
package db

import (
	"errors"
	"time"

	"github.com/toozej/podgrab/internal/logger"
	"gorm.io/gorm"
)

type localMigration struct {
	Name      string
	Condition []string
	Query     []string
}

var migrations = []localMigration{
	{
		Name:  "2020_11_03_04_42_SetDefaultDownloadStatus",
		Query: []string{"update podcast_items set download_status=2 where download_path!='' and download_status=0"},
	},
	{
		Name:      "2023_10_17_AddMaxDownloadKeepColumn",
		Condition: []string{"SELECT CASE WHEN COUNT(*) = 0 THEN 1 ELSE 0 END FROM pragma_table_info('settings') WHERE name = 'max_download_keep'"},
		Query:     []string{"ALTER TABLE settings ADD COLUMN max_download_keep INT DEFAULT 0"},
	},
	{
		Name:  "2025_09_17_AddPassthroughPodcastGuidColumn",
		Query: []string{"ALTER TABLE settings ADD COLUMN passthrough_podcast_guiid BOOLEAN NOT NULL DEFAULT FALSE"},
	},
	{
		Name: "2021_06_01_00_00_ConvertFileNameFormat",
		Condition: []string{
			"SELECT COUNT(*) > 0 FROM (SELECT name FROM pragma_table_info('settings') where name is 'append_date_to_file_name')",
			"SELECT COUNT(*) > 0 FROM (SELECT name FROM pragma_table_info('settings') where name is 'append_episode_number_to_file_name')",
		},
		Query: []string{
			"UPDATE settings SET file_name_format = CASE WHEN append_date_to_file_name AND append_episode_number_to_file_name THEN '%EpisodeNumber%-%EpisodeDate%-%EpisodeTitle%' WHEN append_date_to_file_name THEN '%EpisodeDate%-%EpisodeTitle%' WHEN append_episode_number_to_file_name THEN '%EpisodeNumber%-%EpisodeTitle%' ELSE '%EpisodeTitle%' END",
		},
	},
	{
		Name:      "2026_02_22_AddFileNameFormatColumn",
		Condition: []string{"SELECT CASE WHEN COUNT(*) = 0 THEN 1 ELSE 0 END FROM pragma_table_info('settings') WHERE name = 'file_name_format'"},
		Query:     []string{"ALTER TABLE settings ADD COLUMN file_name_format TEXT DEFAULT '%EpisodeTitle%'"},
	},
}

// RunMigrations run migrations.
func RunMigrations() {
	for _, mig := range migrations {
		if err := ExecuteAndSaveMigration(mig); err != nil {
			logger.Log.Warnw("migration failed", "name", mig.Name, "error", err)
		}
	}
}

// ExecuteAndSaveMigration execute and save migration.
func ExecuteAndSaveMigration(mig localMigration) error {
	var migration Migration
	result := DB.Where("name=?", mig.Name).First(&migration)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		var rawResult string
		var shouldMigrate = true
		for _, q := range mig.Condition {
			logger.Log.Debug("condition: " + q)
			result = DB.Raw(q).Scan(&rawResult)
			if result.Error != nil {
				logger.Log.Debugw("migration condition check failed", "error", result.Error)
				return result.Error
			}
			shouldMigrate = shouldMigrate && rawResult == "1"
		}
		if shouldMigrate {
			for _, q := range mig.Query {
				logger.Log.Debug("exec: " + q)
				result = DB.Exec(q)
				if result.Error != nil {
					logger.Log.Debugw("migration execution failed", "error", result.Error)
					return result.Error
				}
			}
		} else {
			logger.Log.Debug("migration not required")
		}
		DB.Save(&Migration{
			Date: time.Now(),
			Name: mig.Name,
		})
		return result.Error
	}
	return nil
}
