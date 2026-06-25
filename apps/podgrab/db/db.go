// Package db provides database models and data access functions.
package db

import (
	"fmt"
	"os"
	"path"

	"github.com/glebarez/sqlite"
	"github.com/toozej/podgrab/internal/logger"
	"gorm.io/gorm"
)

// DB is
var DB *gorm.DB

// Init is used to Initialize Database
func Init() (*gorm.DB, error) {
	configPath := os.Getenv("CONFIG")
	dbPath := path.Join(configPath, "podgrab.db")
	logger.Log.Info(dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		logger.Log.Debug("db err: ", err)
		return nil, err
	}

	localDB, err := db.DB()
	if err != nil {
		logger.Log.Debug("failed to get database connection: ", err)
	} else {
		localDB.SetMaxIdleConns(10)
	}
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
