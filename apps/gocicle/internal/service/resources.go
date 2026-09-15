package service

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
)

// ResourceTable is an allowlist. Never pass user input to a table name directly.
func ResourceTable(kind string) string {
	return map[string]string{"connections": "connections", "ssh-keys": "ssh_keys", "notifications": "notification_destinations"}[kind]
}
func (s *Service) SaveResource(a Actor, kind string, r storage.Resource) (storage.Resource, error) {
	table := ResourceTable(kind)
	if table == "" || !a.Can("write") {
		return r, ErrForbidden
	}
	var secret storage.Secret
	if err := s.DB.First(&secret, "id=? AND owner_id=?", r.SecretID, a.User.ID).Error; err != nil {
		return r, ErrForbidden
	}
	if r.Name == "" || !json.Valid(r.Config) {
		return r, ErrInvalid
	}
	if err := validateResourceConfig(kind, r.Config); err != nil {
		return r, err
	}
	if r.ID == "" {
		r.ID = storage.ID()
		r.OwnerID = a.User.ID
		r.Version = 1
		if kind == "connections" {
			var cfg struct {
				ProviderID string `json:"providerID"`
			}
			if json.Unmarshal(r.Config, &cfg) != nil || cfg.ProviderID == "" {
				return r, ErrInvalid
			}
			return r, s.DB.Table(table).Create(map[string]any{"id": r.ID, "owner_id": r.OwnerID, "name": r.Name, "secret_id": r.SecretID, "provider_id": cfg.ProviderID, "config": r.Config, "version": r.Version}).Error
		}
		return r, s.DB.Table(table).Create(&r).Error
	}
	updates := map[string]any{"name": r.Name, "secret_id": r.SecretID, "config": r.Config, "version": r.Version + 1}
	if kind == "connections" {
		var cfg struct{ ProviderID string }
		if strictConfig(r.Config, &cfg) != nil {
			return r, ErrInvalid
		}
		updates["provider_id"] = cfg.ProviderID
	}
	q := s.DB.Table(table).Where("id=? AND owner_id=? AND version=?", r.ID, a.User.ID, r.Version).Updates(updates)
	if q.Error != nil {
		return r, q.Error
	}
	if q.RowsAffected == 0 {
		return r, ErrConflict
	}
	r.Version++
	return r, nil
}

func validateResourceConfig(kind string, config storage.JSON) error {
	switch kind {
	case "notifications":
		var cfg struct{ Type, Endpoint string }
		if strictConfig(config, &cfg) != nil {
			return ErrInvalid
		}
		switch cfg.Type {
		case "gotify", "slack", "telegram", "discord", "pushover", "pushbullet":
		default:
			return ErrInvalid
		}
	case "connections":
		var cfg struct{ ProviderID string }
		if strictConfig(config, &cfg) != nil || cfg.ProviderID == "" {
			return ErrInvalid
		}
	case "ssh-keys":
		var cfg struct{ PublicKey, Fingerprint string }
		if strictConfig(config, &cfg) != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
func (s *Service) UpdateUser(a Actor, user storage.User) error {
	if err := Admin(a); err != nil {
		return err
	}
	if user.ID == a.User.ID && !user.Enabled {
		return invalid("an administrator cannot disable their own account")
	}
	q := s.DB.Model(&storage.User{}).Where("id=? AND version=?", user.ID, user.Version).Updates(map[string]any{"enabled": user.Enabled, "administrator": user.Administrator, "name": user.Name, "version": user.Version + 1})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return ErrConflict
	}
	return s.Audit(a, "user.update", user.ID)
}
func (s *Service) UpdateRunner(a Actor, r storage.Runner) error {
	if err := Admin(a); err != nil {
		return err
	}
	if !json.Valid(r.Labels) || !json.Valid(r.ApprovedRoots) {
		return ErrInvalid
	}
	q := s.DB.Model(&storage.Runner{}).Where("id=? AND version=?", r.ID, r.Version).Updates(map[string]any{"name": r.Name, "enabled": r.Enabled, "labels": r.Labels, "approved_roots": r.ApprovedRoots, "version": r.Version + 1})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return ErrConflict
	}
	return s.Audit(a, "runner.update", r.ID)
}
func (s *Service) RunnerGrant(a Actor, runner, user string, allow bool) error {
	if err := Admin(a); err != nil {
		return err
	}
	if allow {
		return s.DB.Exec("INSERT INTO runner_grants VALUES (?,?) ON CONFLICT DO NOTHING", runner, user).Error
	}
	return s.DB.Exec("DELETE FROM runner_grants WHERE runner_id=? AND user_id=?", runner, user).Error
}
func (s *Service) DeleteProject(a Actor, id string, version int64) error {
	if err := s.Authorize(a, id, "owner"); err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&storage.Run{}).Where("project_id=? AND status IN ('queued','running')", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return invalid("cancel active runs before deleting the project")
		}
		q := tx.Where("id=? AND version=?", id, version).Delete(&storage.Project{})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			return ErrConflict
		}
		return s.With(tx).Audit(a, "project.delete", id)
	})
}
func (s *Service) DeleteJob(a Actor, id string, version int64) error {
	var job storage.Job
	if err := s.DB.First(&job, "id=?", id).Error; err != nil {
		return err
	}
	if err := s.Authorize(a, job.ProjectID, "operator"); err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&storage.Run{}).Where("job_id=? AND status IN ('queued','running')", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return invalid("cancel active runs before deleting the job")
		}
		q := tx.Where("id=? AND version=?", id, version).Delete(&storage.Job{})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			return ErrConflict
		}
		return nil
	})
}

func (s *Service) Invite(a Actor) (string, error) {
	if err := Admin(a); err != nil {
		return "", err
	}
	token := security.Token()
	return token, s.DB.Exec("INSERT INTO invitations(hash,expires_at) VALUES (?,?)", security.Hash(token), time.Now().Add(24*time.Hour)).Error
}

func strictConfig(raw storage.JSON, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}
