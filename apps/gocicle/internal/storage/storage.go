// Package storage uses PostgreSQL as the authoritative store and execution queue.
package storage

import (
	"context"
	"database/sql/driver"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrations embed.FS

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}
func (j *JSON) Scan(value any) error {
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], v...)
	case nil:
		*j = nil
	default:
		return errors.New("invalid JSONB value")
	}
	return nil
}
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}
func (j *JSON) UnmarshalJSON(b []byte) error { *j = append((*j)[:0], b...); return nil }
func Encode(v any) JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return JSON(b)
}
func ID() string { return uuid.NewString() }
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
	if err != nil {
		return nil, errors.New("cannot connect to PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(718606943)").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())").Error; err != nil {
			return err
		}
		entries, err := migrations.ReadDir("migrations")
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			var count int64
			if err := tx.Table("schema_migrations").Where("version = ?", entry.Name()).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			data, err := migrations.ReadFile("migrations/" + entry.Name())
			if err != nil {
				return err
			}
			if err := tx.Exec(string(data)).Error; err != nil {
				return fmt.Errorf("migration %s: %w", entry.Name(), err)
			}
			if err := tx.Exec("INSERT INTO schema_migrations(version) VALUES (?)", entry.Name()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func Check(db *gorm.DB) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	var n int64
	if err = db.Table("schema_migrations").Count(&n).Error; err != nil || int(n) != len(entries) {
		return errors.New("database migrations are required: run gocicle migrate")
	}
	return nil
}

type User struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Administrator bool   `json:"administrator"`
	Preferences   JSON   `json:"preferences" swaggertype:"object"`
	Version       int64  `json:"version"`
}
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	OwnerID     string `json:"ownerID"`
	Source      JSON   `json:"source" swaggertype:"object"`
	Preferences JSON   `json:"preferences" swaggertype:"object"`
	Version     int64  `json:"version"`
}
type Job struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectID"`
	Name       string `json:"name"`
	RevisionID string `json:"revisionID"`
	Version    int64  `json:"version"`
	Enabled    bool   `json:"enabled"`
}
type Revision struct {
	ID        string `json:"id"`
	JobID     string `json:"jobID"`
	Spec      JSON   `json:"spec" swaggertype:"object"`
	Source    JSON   `json:"source" swaggertype:"object"`
	CreatedBy string `json:"createdBy"`
}

func (Revision) TableName() string { return "job_revisions" }

type Run struct {
	ID              string     `json:"id"`
	JobID           string     `json:"jobID"`
	RevisionID      string     `json:"revisionID"`
	ProjectID       string     `json:"projectID"`
	Preferences     JSON       `json:"preferences" swaggertype:"object"`
	Status          string     `json:"status"`
	ScheduledAt     *time.Time `json:"scheduledAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	RunnerID        *string    `json:"runnerID,omitempty"`
	SourceCommit    string     `json:"sourceCommit"`
	ImageIdentity   string     `json:"imageIdentity"`
	Result          JSON       `json:"result" swaggertype:"object"`
	PushResult      string     `json:"pushResult"`
	CancelRequested bool       `json:"cancelRequested"`
	LogBytes        int64      `json:"logBytes"`
	LogTruncated    bool       `json:"logTruncated"`
}
type Runner struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	TokenHash     string     `json:"-"`
	Labels        JSON       `json:"labels" swaggertype:"object"`
	ApprovedRoots JSON       `json:"approvedRoots" swaggertype:"object"`
	Enabled       bool       `json:"enabled"`
	LastSeen      *time.Time `json:"lastSeen,omitempty"`
	Version       int64      `json:"version"`
}
type Secret struct {
	ID        string `json:"id"`
	OwnerID   string `json:"ownerID"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Encrypted JSON   `json:"-" swaggertype:"object"`
	Version   int64  `json:"version"`
}
type Resource struct {
	ID       string `json:"id"`
	OwnerID  string `json:"ownerID"`
	Name     string `json:"name"`
	SecretID string `json:"secretID"`
	Config   JSON   `json:"config" swaggertype:"object"`
	Version  int64  `json:"version"`
}
type Provider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Config  JSON   `json:"config" swaggertype:"object"`
	Version int64  `json:"version"`
}

func (Provider) TableName() string { return "provider_instances" }
