package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	_ "github.com/glebarez/sqlite"
	"github.com/toozej/rss2socials/internal/rss"
)

var DB *sql.DB

func InitDB(path ...string) {
	var err error
	dbPath := "./tooted_posts.db"
	if len(path) > 0 && path[0] != "" {
		dbPath = path[0]
	} else if p := os.Getenv("DB_PATH"); p != "" {
		dbPath = p
	}
	log.Debugf("Opening database at %s", dbPath)
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	query := `CREATE TABLE IF NOT EXISTS tooted_posts (
		link TEXT PRIMARY KEY,
		content_hash TEXT,
		timestamp TEXT,
		startup_time TEXT,
		mastodon_posted INTEGER DEFAULT 0,
		bluesky_posted INTEGER DEFAULT 0,
		threads_posted INTEGER DEFAULT 0
	)`
	_, err = DB.Exec(query)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}

	migrateDB()
}

func migrateDB() {
	type migration struct {
		query string
		name  string
	}
	migrations := []migration{
		{"ALTER TABLE tooted_posts ADD COLUMN mastodon_posted INTEGER DEFAULT 0", "mastodon_posted"},
		{"ALTER TABLE tooted_posts ADD COLUMN bluesky_posted INTEGER DEFAULT 0", "bluesky_posted"},
		{"ALTER TABLE tooted_posts ADD COLUMN threads_posted INTEGER DEFAULT 0", "threads_posted"},
		{"ALTER TABLE tooted_posts ADD COLUMN startup_time TEXT", "startup_time"},
	}
	for _, m := range migrations {
		_, err := DB.Exec(m.query)
		if err != nil {
			log.Debugf("Column %s already exists: %v", m.name, err)
		}
	}
}

func CloseDB() {
	err := DB.Close()
	if err != nil {
		log.Error("Error closing SQLite database connection: ", err)
	}
}

func StoreTootedPost(link string, content string, startupTime string) error {
	query := `INSERT INTO tooted_posts(link, content_hash, timestamp, startup_time, mastodon_posted, bluesky_posted, threads_posted) VALUES (?, ?, ?, ?, 0, 0, 0) ON CONFLICT(link) DO UPDATE SET content_hash = excluded.content_hash, timestamp = excluded.timestamp, startup_time = excluded.startup_time`
	contentHash := rss.HashContent(content)
	_, err := DB.Exec(query, link, fmt.Sprintf("%x", contentHash), time.Now().Format(time.RFC3339), startupTime)
	return err
}

func MarkSitePosted(link string, site string) error {
	switch site {
	case "mastodon":
		_, err := DB.Exec("UPDATE tooted_posts SET mastodon_posted = 1 WHERE link = ?", link)
		return err
	case "bluesky":
		_, err := DB.Exec("UPDATE tooted_posts SET bluesky_posted = 1 WHERE link = ?", link)
		return err
	case "threads":
		_, err := DB.Exec("UPDATE tooted_posts SET threads_posted = 1 WHERE link = ?", link)
		return err
	default:
		return fmt.Errorf("unknown site: %s", site)
	}
}

func IsSitePosted(link string, site string) (bool, error) {
	var row *sql.Row
	switch site {
	case "mastodon":
		row = DB.QueryRow("SELECT mastodon_posted FROM tooted_posts WHERE link = ?", link)
	case "bluesky":
		row = DB.QueryRow("SELECT bluesky_posted FROM tooted_posts WHERE link = ?", link)
	case "threads":
		row = DB.QueryRow("SELECT threads_posted FROM tooted_posts WHERE link = ?", link)
	default:
		return false, fmt.Errorf("unknown site: %s", site)
	}
	var posted int
	err := row.Scan(&posted)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return posted == 1, nil
}

func HasPostChanged(link string, content string) (exists bool, updated bool, err error) {
	query := `SELECT content_hash FROM tooted_posts WHERE link = ?`
	row := DB.QueryRow(query, link)

	var storedHash string
	err = row.Scan(&storedHash)
	if err == sql.ErrNoRows {
		return false, false, nil
	} else if err != nil {
		return false, false, err
	}

	newHash := fmt.Sprintf("%x", rss.HashContent(content))
	if storedHash != newHash {
		return true, true, nil
	}

	return true, false, nil
}

func IsFirstCycle() bool {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM tooted_posts").Scan(&count)
	if err != nil {
		return true
	}
	return count == 0
}
