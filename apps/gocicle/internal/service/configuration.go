package service

import (
	"encoding/json"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
)

type Membership struct {
	ProjectID string `json:"projectID"`
	UserID    string `json:"userID"`
	Role      string `json:"role"`
}
type ConfigJob struct {
	Job    storage.Job `json:"job"`
	Spec   jobs.Spec   `json:"spec"`
	Source jobs.Source `json:"source"`
}

// Configuration contains portable settings. It excludes sessions, tokens, and encrypted values.
type Configuration struct {
	APIVersion  string                        `json:"apiVersion"`
	Users       []storage.User                `json:"users"`
	Projects    []storage.Project             `json:"projects"`
	Jobs        []ConfigJob                   `json:"jobs"`
	Memberships []Membership                  `json:"memberships"`
	Secrets     []storage.Secret              `json:"credentials"`
	Providers   []storage.Provider            `json:"providers"`
	Resources   map[string][]storage.Resource `json:"resources"`
}

func (s *Service) ExportConfiguration(a Actor) (Configuration, error) {
	out := Configuration{APIVersion: "gocicle/v1", Resources: map[string][]storage.Resource{}}
	if err := Admin(a); err != nil {
		return out, err
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Find(&out.Users).Error; err != nil {
			return err
		}
		if err := tx.Find(&out.Projects).Error; err != nil {
			return err
		}
		if err := tx.Find(&out.Secrets).Error; err != nil {
			return err
		}
		if err := tx.Find(&out.Providers).Error; err != nil {
			return err
		}
		if err := tx.Table("memberships").Find(&out.Memberships).Error; err != nil {
			return err
		}
		var allJobs []storage.Job
		if err := tx.Find(&allJobs).Error; err != nil {
			return err
		}
		for _, job := range allJobs {
			var revision storage.Revision
			if err := tx.First(&revision, "id=?", job.RevisionID).Error; err != nil {
				return err
			}
			spec, err := Decode[jobs.Spec](revision.Spec)
			if err != nil {
				return err
			}
			source, err := Decode[jobs.Source](revision.Source)
			if err != nil {
				return err
			}
			out.Jobs = append(out.Jobs, ConfigJob{Job: job, Spec: spec, Source: source})
		}
		for _, kind := range []string{"connections", "ssh-keys", "notifications"} {
			var rows []storage.Resource
			if err := tx.Table(ResourceTable(kind)).Find(&rows).Error; err != nil {
				return err
			}
			out.Resources[kind] = rows
		}
		return nil
	})
	return out, err
}
func (s *Service) ImportConfiguration(a Actor, key string, input Configuration, mapping map[string]string) error {
	if err := Admin(a); err != nil {
		return err
	}
	if input.APIVersion != "gocicle/v1" || key == "" {
		return ErrInvalid
	}
	request := storage.Encode(map[string]any{"configuration": input, "credentials": mapping})
	// Detach caller-owned maps before replacing references.
	raw := storage.Encode(input)
	input = Configuration{}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		var result map[string]bool
		cached, err := svc.idempotency(a, key, request, &result)
		if err != nil || cached {
			return err
		}
		remap := func(old string) (string, error) {
			if old == "" {
				return "", nil
			}
			target, ok := mapping[old]
			if !ok {
				return "", invalid("map credential %s explicitly", old)
			}
			var n int64
			if err := tx.Model(&storage.Secret{}).Where("id=?", target).Count(&n).Error; err != nil {
				return "", err
			}
			if n != 1 {
				return "", invalid("mapped credential does not exist")
			}
			return target, nil
		}
		for _, secret := range input.Secrets {
			if _, err := remap(secret.ID); err != nil {
				return err
			}
		}
		for _, user := range input.Users {
			if user.ID == "" {
				return ErrInvalid
			}
			user.Enabled = false
			user.Administrator = false
			user.Version = 1
			if _, err := Decode[jobs.Preferences](user.Preferences); err != nil {
				return ErrInvalid
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}
		for _, provider := range input.Providers {
			cfg, err := Decode[providers.Config](provider.Config)
			if err != nil {
				return ErrInvalid
			}
			cfg.SecretID, err = remap(cfg.SecretID)
			if err != nil {
				return err
			}
			if _, err := providers.New(provider.Kind, cfg, ""); err != nil {
				return ErrInvalid
			}
			provider.Config = storage.Encode(cfg)
			provider.Version = 1
			if err := tx.Create(&provider).Error; err != nil {
				return err
			}
		}
		remapSource := func(source *jobs.Source) error {
			if source.Git != nil {
				ref, err := remap(source.Git.Credential)
				if err != nil {
					return err
				}
				source.Git.Credential = ref
			}
			return source.Validate()
		}
		for _, project := range input.Projects {
			if _, err := Decode[jobs.Preferences](project.Preferences); err != nil {
				return ErrInvalid
			}
			source, err := Decode[jobs.Source](project.Source)
			if err != nil {
				return ErrInvalid
			}
			if err := remapSource(&source); err != nil {
				return err
			}
			project.Source = storage.Encode(source)
			project.Version = 1
			if err := tx.Create(&project).Error; err != nil {
				return err
			}
		}
		for _, m := range input.Memberships {
			if m.Role != "viewer" && m.Role != "operator" && m.Role != "owner" {
				return ErrInvalid
			}
			if err := tx.Table("memberships").Create(&m).Error; err != nil {
				return err
			}
		}
		for kind, resources := range input.Resources {
			table := ResourceTable(kind)
			if table == "" {
				return ErrInvalid
			}
			for _, resource := range resources {
				if err := validateResourceConfig(kind, resource.Config); err != nil {
					return err
				}
				ref, err := remap(resource.SecretID)
				if err != nil {
					return err
				}
				resource.SecretID = ref
				resource.Version = 1
				if kind == "connections" {
					var cfg struct {
						ProviderID string `json:"providerID"`
					}
					if err := json.Unmarshal(resource.Config, &cfg); err != nil {
						return ErrInvalid
					}
					if err := tx.Table(table).Create(map[string]any{"id": resource.ID, "owner_id": resource.OwnerID, "name": resource.Name, "secret_id": ref, "provider_id": cfg.ProviderID, "config": resource.Config, "version": int64(1)}).Error; err != nil {
						return err
					}
				} else {
					if err := tx.Table(table).Create(&resource).Error; err != nil {
						return err
					}
				}
			}
		}
		for _, entry := range input.Jobs {
			if err := remapSource(&entry.Source); err != nil {
				return err
			}
			for name, ref := range entry.Spec.Secrets {
				target, err := remap(ref)
				if err != nil {
					return err
				}
				entry.Spec.Secrets[name] = target
			}
			entry.Spec.Defaults()
			if err := entry.Spec.Validate(entry.Source); err != nil {
				return ErrInvalid
			}
			job := entry.Job
			job.Version = 0
			job.Enabled = false
			job.RevisionID = ""
			if err := tx.Omit("revision_id").Create(&job).Error; err != nil {
				return err
			}
			if err := svc.revise(a, &job, entry.Spec, entry.Source, false); err != nil {
				return err
			}
		}
		if err := svc.Audit(a, "configuration.import", key); err != nil {
			return err
		}
		return svc.remember(a, key, request, map[string]bool{"imported": true})
	})
}

// LinkImportedIdentity binds the first identity of a disabled imported user. Additional identities require an authenticated browser session.
func (s *Service) LinkImportedIdentity(a Actor, userID, providerID, subject string) error {
	if err := Admin(a); err != nil {
		return err
	}
	if subject == "" {
		return ErrInvalid
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", userID).Error; err != nil {
			return err
		}
		var user storage.User
		if err := tx.First(&user, "id=? AND NOT enabled", userID).Error; err != nil {
			return ErrForbidden
		}
		var n int64
		if err := tx.Table("identities").Where("user_id=?", userID).Count(&n).Error; err != nil {
			return err
		}
		if n != 0 {
			return ErrForbidden
		}
		if err := tx.Exec("INSERT INTO identities(id,user_id,provider_id,subject) VALUES (?,?,?,?)", storage.ID(), userID, providerID, subject).Error; err != nil {
			return err
		}
		return s.With(tx).Audit(a, "identity.import-link", userID)
	})
}
