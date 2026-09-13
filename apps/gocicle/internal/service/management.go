package service

import (
	"strings"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenMetadata struct {
	ID        string       `json:"id"`
	Scopes    storage.JSON `json:"scopes" swaggertype:"object"`
	ExpiresAt time.Time    `json:"expiresAt"`
}

func (s *Service) Tokens(a Actor, offset int) ([]TokenMetadata, error) {
	if !a.Can("read") {
		return nil, ErrForbidden
	}
	var rows []TokenMetadata
	err := s.DB.Table("api_tokens").Select("id,scopes,expires_at").Where("user_id=?", a.User.ID).Order("id").Limit(100).Offset(offset).Find(&rows).Error
	return rows, err
}

func (s *Service) RevokeToken(a Actor, id string) error {
	if !a.Can("write") {
		return ErrForbidden
	}
	return s.DB.Exec("DELETE FROM api_tokens WHERE id=? AND user_id=?", id, a.User.ID).Error
}

func (s *Service) RenameProject(a Actor, id, name string, version int64) error {
	if err := s.Authorize(a, id, "owner"); err != nil {
		return err
	}
	if !jobs.ValidName(name) {
		return ErrInvalid
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&storage.Project{}).Where("id=? AND version=?", id, version).Updates(map[string]any{"name": name, "version": version + 1})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return ErrConflict
		}
		return s.With(tx).Audit(a, "project.rename", id)
	})
}

func (s *Service) CreateUser(a Actor, name string) (storage.User, error) {
	user := storage.User{ID: storage.ID(), Name: name, Preferences: storage.Encode(map[string]any{}), Version: 1}
	if err := Admin(a); err != nil {
		return user, err
	}
	if strings.TrimSpace(name) == "" {
		return user, ErrInvalid
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return s.With(tx).Audit(a, "user.create-disabled", user.ID)
	})
	return user, err
}

// DeleteUser preserves references in run history. The administrator must first remove owned resources.
func (s *Service) DeleteUser(a Actor, id string, version int64) error {
	if err := Admin(a); err != nil {
		return err
	}
	if id == a.User.ID {
		return invalid("an administrator cannot delete their own account")
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var user storage.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id=?", id).Error; err != nil {
			return err
		}
		if user.Version != version {
			return ErrConflict
		}
		if user.Enabled {
			return invalid("disable the user before deleting the account")
		}
		if err := tx.Exec("DELETE FROM oauth_states WHERE user_id=?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE audit_events SET actor_id=NULL WHERE actor_id=?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&user).Error; err != nil {
			return invalid("remove owned resources and retain referenced users until their run history expires")
		}
		return s.With(tx).Audit(a, "user.delete", id)
	})
}

func (s *Service) SaveProvider(a Actor, row storage.Provider) (storage.Provider, error) {
	if err := Admin(a); err != nil {
		return row, err
	}
	cfg, err := Decode[providers.Config](row.Config)
	if err != nil || strings.TrimSpace(row.Name) == "" {
		return row, ErrInvalid
	}
	adapter, err := providers.New(row.Kind, cfg, "")
	if err != nil {
		return row, ErrInvalid
	}
	row.Config = storage.Encode(adapter.Config)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if cfg.SecretID != "" {
			var n int64
			if err := tx.Model(&storage.Secret{}).Where("id=? AND kind='oauth'", cfg.SecretID).Count(&n).Error; err != nil {
				return err
			}
			if n != 1 {
				return invalid("provider OAuth credential is missing")
			}
		}
		if row.ID == "" {
			row.ID, row.Version = storage.ID(), 1
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			var old storage.Provider
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, "id=?", row.ID).Error; err != nil {
				return err
			}
			if old.Version != row.Version {
				return ErrConflict
			}
			oldConfig, err := Decode[providers.Config](old.Config)
			if err != nil {
				return err
			}
			previous, err := providers.New(old.Kind, oldConfig, "")
			if err != nil {
				return err
			}
			if old.Kind != row.Kind || previous.Config.BaseURL != adapter.Config.BaseURL {
				return invalid("create a new provider instance to change its type or origin")
			}
			row.Version++
			if err := tx.Model(&old).Updates(map[string]any{"name": row.Name, "config": row.Config, "version": row.Version}).Error; err != nil {
				return err
			}
		}
		return s.With(tx).Audit(a, "provider.save", row.ID)
	})
	return row, err
}

func (s *Service) DeleteProvider(a Actor, id string, version int64) error {
	if err := Admin(a); err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM oauth_states WHERE provider_id=?", id).Error; err != nil {
			return err
		}
		q := tx.Where("id=? AND version=?", id, version).Delete(&storage.Provider{})
		if q.Error != nil {
			return invalid("provider instances with identities or connections cannot be deleted")
		}
		if q.RowsAffected != 1 {
			return ErrConflict
		}
		return s.With(tx).Audit(a, "provider.delete", id)
	})
}
