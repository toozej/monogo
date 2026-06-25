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
}

var DB *gorm.DB

func InitDB(path ...string) {
	var err error
	dbPath := "./tooted_posts.db"
	if len(path) > 0 && path[0] != "" {
		dbPath = path[0]
	} else if p := os.Getenv("DB_PATH"); p != "" {
		dbPath = p
	}

	log.Debugf("Opening database at %s", dbPath)
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	err = DB.AutoMigrate(&TootedPost{})
	if err != nil {
		log.Fatal("Failed to auto-migrate database:", err)
	}
}

func CloseDB() {
	sqlDB, err := DB.DB()
	if err != nil {
		log.Error("Error getting underlying sql.DB: ", err)
		return
	}
	err = sqlDB.Close()
	if err != nil {
		log.Error("Error closing SQLite database connection: ", err)
	}
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

func IsFirstCycle() bool {
	var count int64
	if err := DB.Model(&TootedPost{}).Count(&count).Error; err != nil {
		log.Errorf("Error counting posts: %v", err)
		return false
	}
	return count == 0
}
