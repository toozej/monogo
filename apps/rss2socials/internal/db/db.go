package db

import (
	"fmt"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/rss2socials/internal/rss"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type TootedPost struct {
	Link           string `gorm:"primaryKey"`
	ContentHash    string
	Timestamp      string
	StartupTime    string
	MastodonPosted bool `gorm:"default:false"`
	BlueskyPosted  bool `gorm:"default:false"`
	ThreadsPosted  bool `gorm:"default:false"`
	PendingUpdate  bool `gorm:"default:false"`
}

type FeedState struct {
	Key string `gorm:"primaryKey"`
}

const initialSnapshotState = "initial_snapshot_seeded"

var DB *gorm.DB

func InitDB(path ...string) error {
	dbPath := "./tooted_posts.db"
	if len(path) > 0 && path[0] != "" {
		dbPath = path[0]
	} else if p := os.Getenv("DB_PATH"); p != "" {
		dbPath = p
	}

	log.Debugf("Opening database at %s", dbPath)
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open database %q: %w", dbPath, err)
	}

	if err := database.AutoMigrate(&TootedPost{}, &FeedState{}); err != nil {
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return fmt.Errorf("migrate database %q: %w", dbPath, err)
	}
	DB = database
	return nil
}

func CloseDB() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get database connection: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// StoreUpdatedPost records new content and makes every site eligible to post
// that content. It is separate from StoreTootedPost so retries of unchanged
// content preserve per-site success flags.
func StoreUpdatedPost(link string, content string, startupTime string) error {
	contentHash := fmt.Sprintf("%x", rss.HashContent(content))
	post := TootedPost{
		Link:          link,
		ContentHash:   contentHash,
		Timestamp:     time.Now().Format(time.RFC3339),
		StartupTime:   startupTime,
		PendingUpdate: true,
	}
	result := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "link"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"content_hash", "timestamp", "startup_time", "pending_update",
			"mastodon_posted", "bluesky_posted", "threads_posted",
		}),
	}).Create(&post)
	return result.Error
}

func StoreTootedPost(link string, content string, startupTime string) error {
	contentHash := fmt.Sprintf("%x", rss.HashContent(content))
	post := TootedPost{
		Link:        link,
		ContentHash: contentHash,
		Timestamp:   time.Now().Format(time.RFC3339),
		StartupTime: startupTime,
	}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "link"}},
		DoUpdates: clause.AssignmentColumns([]string{"content_hash", "timestamp", "startup_time"}),
	}).Create(&post)
	return result.Error
}

func SeedPost(link string, content string, startupTime string) error {
	return seedPost(DB, link, content, startupTime)
}

func SeedPosts(posts []rss.RSSItem, startupTime string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, post := range posts {
			if err := seedPost(tx, post.Link, post.Content, startupTime); err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&FeedState{Key: initialSnapshotState}).Error
	})
}

func seedPost(database *gorm.DB, link string, content string, startupTime string) error {
	contentHash := fmt.Sprintf("%x", rss.HashContent(content))
	post := TootedPost{
		Link:           link,
		ContentHash:    contentHash,
		Timestamp:      time.Now().Format(time.RFC3339),
		StartupTime:    startupTime,
		MastodonPosted: true,
		BlueskyPosted:  true,
		ThreadsPosted:  true,
	}
	return database.Clauses(clause.OnConflict{DoNothing: true}).Create(&post).Error
}

var validSites = map[string]string{
	"mastodon": "mastodon_posted",
	"bluesky":  "bluesky_posted",
	"threads":  "threads_posted",
}

func MarkSitePosted(link string, site string) error {
	column, ok := validSites[site]
	if !ok {
		return fmt.Errorf("unknown site: %s", site)
	}
	result := DB.Model(&TootedPost{}).Where("link = ?", link).Update(column, true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no post found with link: %s", link)
	}
	return nil
}

func IsSitePosted(link string, site string) (bool, error) {
	column, ok := validSites[site]
	if !ok {
		return false, fmt.Errorf("unknown site: %s", site)
	}
	var post TootedPost
	result := DB.Select(column).Where("link = ?", link).First(&post)
	if result.Error == gorm.ErrRecordNotFound {
		return false, nil
	}
	if result.Error != nil {
		return false, result.Error
	}

	switch site {
	case "mastodon":
		return post.MastodonPosted, nil
	case "bluesky":
		return post.BlueskyPosted, nil
	case "threads":
		return post.ThreadsPosted, nil
	}
	return false, fmt.Errorf("unknown site: %s", site)
}

func HasPostChanged(link string, content string) (exists bool, updated bool, err error) {
	var post TootedPost
	result := DB.Select("content_hash").Where("link = ?", link).First(&post)
	if result.Error == gorm.ErrRecordNotFound {
		return false, false, nil
	}
	if result.Error != nil {
		return false, false, result.Error
	}

	newHash := fmt.Sprintf("%x", rss.HashContent(content))
	if post.ContentHash != newHash {
		return true, true, nil
	}
	return true, false, nil
}

func IsUpdatePending(link string) (bool, error) {
	var post TootedPost
	result := DB.Select("pending_update").Where("link = ?", link).First(&post)
	if result.Error != nil {
		return false, result.Error
	}
	return post.PendingUpdate, nil
}

func MarkUpdateComplete(link string) error {
	result := DB.Model(&TootedPost{}).Where("link = ?", link).Update("pending_update", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no post found with link: %s", link)
	}
	return nil
}

func IsFirstCycleE() (bool, error) {
	var state FeedState
	result := DB.Select("key").Where("key = ?", initialSnapshotState).First(&state)
	if result.Error == nil {
		return false, nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return false, result.Error
	}

	// Databases created by older versions have no FeedState row. Preserve
	// their established behavior by treating any stored post as evidence that
	// the initial snapshot was already processed.
	var count int64
	if err := DB.Model(&TootedPost{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func IsFirstCycle() bool {
	first, err := IsFirstCycleE()
	if err != nil {
		log.Errorf("Error counting posts: %v", err)
		return false
	}
	return first
}
