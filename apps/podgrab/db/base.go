// Package db provides database models and data access functions.
package db

import (
	"time"

	uuid "github.com/gofrs/uuid/v5"
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
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}
	tx.Statement.SetColumn("ID", id.String())
	return nil
}
