// Package db provides database models and data access functions.
package db

import (
	"time"

	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

// Base is
type Base struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `gorm:"index"`
	ID        string     `sql:"type:uuid;primary_key"`
}

// BeforeCreate generates a UUID for new records before database insertion
func (base *Base) BeforeCreate(tx *gorm.DB) error {
	tx.Statement.SetColumn("ID", uuid.NewV4().String())
	return nil
}
