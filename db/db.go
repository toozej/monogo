// Package db provides database models and data access functions.
package db

import (
	"fmt"
	"log"
	"os"
	"path"

	"gorm.io/driver/sqlite"

	"gorm.io/gorm"
)

// DB is
var DB *gorm.DB

// Init is used to Initialize Database
func Init() (*gorm.DB, error) {
	// github.com/mattn/go-sqlite3
	configPath := os.Getenv("CONFIG")
	dbPath := path.Join(configPath, "podgrab.db")
	log.Println(dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Println("db err: ", err)
		return nil, err
	}

	localDB, _ := db.DB()
	localDB.SetMaxIdleConns(10)
	// db.LogMode(true)
	DB = db
	return DB, nil
}

// Migrate Database
func Migrate() {
	if err := DB.AutoMigrate(&Podcast{}, &PodcastItem{}, &Setting{}, &Migration{}, &JobLock{}, &Tag{}); err != nil {
		panic(fmt.Sprintf("failed to auto-migrate database: %v", err))
	}
	RunMigrations()
}

// GetDB returns the database connection for creating a connection pool.
func GetDB() *gorm.DB {
	return DB
}
