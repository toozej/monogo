package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/toozej/podgrab/db"
	"gorm.io/gorm"
)

func TestRunReturnsErrorWhenDatabaseInitFails(t *testing.T) {
	oldDB := db.DB
	oldInitDB := initDB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = oldDB
		initDB = oldInitDB
	})
	initDB = db.Init

	configPath := filepath.Join(t.TempDir(), "missing", "config")
	t.Setenv("CONFIG", configPath)

	exitCode := run()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when db init fails, got %d", exitCode)
	}
}

func TestRunReturnsErrorWhenSQLiteInitFails(t *testing.T) {
	oldDB := db.DB
	oldInitDB := initDB
	db.DB = nil
	t.Cleanup(func() {
		db.DB = oldDB
		initDB = oldInitDB
	})

	initDB = func() (*gorm.DB, error) {
		return nil, errors.New("failed to initialize sqlite driver")
	}

	exitCode := run()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for CGO-disabled sqlite init, got %d", exitCode)
	}
}
