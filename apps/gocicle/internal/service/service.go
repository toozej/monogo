// Package service applies authorization and transactional state changes.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrApprovalRequired = errors.New("an administrator must approve this account")
var ErrForbidden = errors.New("access is denied")
var ErrConflict = errors.New("revision or idempotency key conflicts with stored data")
var ErrInvalid = errors.New("request is invalid")

type Actor struct {
	User     storage.User
	Scopes   []string
	CSRFHash string
	Session  bool
}

func (a Actor) Can(scope string) bool {
	if !a.User.Enabled {
		return false
	}
	for _, s := range a.Scopes {
		if s == scope || s == "admin" {
			return true
		}
	}
	return false
}

type Service struct {
	DB   *gorm.DB
	Keys security.Keyring
}

func New(db *gorm.DB, keys security.Keyring) *Service { return &Service{DB: db, Keys: keys} }
func (s *Service) With(db *gorm.DB) *Service          { return New(db, s.Keys) }
func (s *Service) Authenticate(ctx context.Context, token string, session bool) (Actor, error) {
	var row struct {
		UserID   string
		Scopes   storage.JSON
		CSRFHash string
	}
	table := "api_tokens"
	fields := "user_id, scopes"
	if session {
		table = "sessions"
		fields = "user_id, csrf_hash"
	}
	if err := s.DB.WithContext(ctx).Table(table).Select(fields).Where("hash = ? AND expires_at > now()", security.Hash(token)).Take(&row).Error; err != nil {
		return Actor{}, ErrForbidden
	}
	var a Actor
	if err := s.DB.WithContext(ctx).First(&a.User, "id = ? AND enabled", row.UserID).Error; err != nil {
		return a, ErrForbidden
	}
	a.Session = session
	a.CSRFHash = row.CSRFHash
	if session {
		a.Scopes = []string{"read", "write"}
		if a.User.Administrator {
			a.Scopes = append(a.Scopes, "admin")
		}
	} else {
		if err := json.Unmarshal(row.Scopes, &a.Scopes); err != nil {
			return a, err
		}
	}
	return a, nil
}
func (s *Service) Authorize(a Actor, projectID, role string) error {
	scope := "read"
	if role != "viewer" {
		scope = "write"
	}
	if !a.User.Enabled || !a.Can(scope) {
		return ErrForbidden
	}
	if a.User.Administrator && a.Can("admin") {
		return nil
	}
	var p storage.Project
	if err := s.DB.First(&p, "id = ?", projectID).Error; err != nil {
		return err
	}
	if p.OwnerID == a.User.ID {
		return nil
	}
	var m struct{ Role string }
	if err := s.DB.Table("memberships").Where("project_id = ? AND user_id = ?", projectID, a.User.ID).Take(&m).Error; err != nil {
		return ErrForbidden
	}
	rank := map[string]int{"viewer": 1, "operator": 2, "owner": 3}
	if rank[m.Role] < rank[role] {
		return ErrForbidden
	}
	return nil
}
func Admin(a Actor) error {
	if !a.User.Enabled || !a.User.Administrator || !a.Can("admin") {
		return ErrForbidden
	}
	return nil
}
func (s *Service) Audit(a Actor, action, target string) error {
	return s.DB.Exec("INSERT INTO audit_events(id,actor_id,action,target) VALUES (?,?,?,?)", storage.ID(), a.User.ID, action, target).Error
}
func (s *Service) Bootstrap(ctx context.Context) (string, error) {
	token := security.Token()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(718606944)").Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&storage.User{}).Where("administrator").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("an administrator already exists")
		}
		if err := tx.Exec("DELETE FROM invitations WHERE administrator").Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO invitations(hash,administrator,expires_at) VALUES (?,true,?)", security.Hash(token), time.Now().Add(time.Hour)).Error
	})
	return token, err
}
func (s *Service) CreateToken(a Actor, scopes []string, expiry time.Time) (string, error) {
	if !a.User.Enabled || !a.Can("write") || len(scopes) == 0 || expiry.Before(time.Now()) || expiry.After(time.Now().Add(366*24*time.Hour)) {
		return "", ErrForbidden
	}
	for _, scope := range scopes {
		if scope != "read" && scope != "write" && scope != "admin" {
			return "", ErrInvalid
		}
		if !a.Can(scope) || (scope == "admin" && !a.User.Administrator) {
			return "", ErrForbidden
		}
	}
	token := security.Token()
	err := s.DB.Exec("INSERT INTO api_tokens(id,user_id,hash,scopes,expires_at) VALUES (?,?,?,?,?)", storage.ID(), a.User.ID, security.Hash(token), storage.Encode(scopes), expiry).Error
	return token, err
}
func (s *Service) ListProjects(a Actor, offset int) ([]storage.Project, error) {
	if !a.Can("read") {
		return nil, ErrForbidden
	}
	var result []storage.Project
	q := s.DB.Model(&storage.Project{})
	if !a.User.Administrator || !a.Can("admin") {
		q = q.Where("owner_id = ? OR id IN (SELECT project_id FROM memberships WHERE user_id = ?)", a.User.ID, a.User.ID)
	}
	err := q.Order("name,id").Limit(100).Offset(offset).Find(&result).Error
	return result, err
}
func (s *Service) CreateProject(a Actor, p storage.Project) (storage.Project, error) {
	if !a.Can("write") {
		return p, ErrForbidden
	}
	source, err := Decode[jobs.Source](p.Source)
	if err != nil || source.Validate() != nil || !jobs.ValidName(p.Name) {
		return p, ErrInvalid
	}
	preferences, err := Decode[jobs.Preferences](p.Preferences)
	if len(p.Preferences) == 0 {
		preferences = jobs.Preferences{}
		err = nil
	}
	if err != nil {
		return p, ErrInvalid
	}
	if err := s.validateDestinations(a.User.ID, preferences); err != nil {
		return p, err
	}
	p.ID = storage.ID()
	p.OwnerID = a.User.ID
	p.Version = 1
	p.Preferences = storage.Encode(preferences)
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return s.With(tx).Audit(a, "project.create", p.ID)
	})
	return p, err
}
func (s *Service) Preferences(a Actor, projectID string, p jobs.Preferences, version int64) error {
	if projectID == "" {
		if err := s.validateDestinations(a.User.ID, p); err != nil {
			return err
		}
		if !a.Can("write") {
			return ErrForbidden
		}
		q := s.DB.Model(&storage.User{}).Where("id=? AND version=?", a.User.ID, version).Updates(map[string]any{"preferences": storage.Encode(p), "version": version + 1})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			return ErrConflict
		}
		return nil
	}
	if err := s.Authorize(a, projectID, "owner"); err != nil {
		return err
	}
	if p.Notifications != nil {
		for _, id := range *p.Notifications {
			var n int64
			if err := s.DB.Table("notification_destinations").Where("id=? AND owner_id=(SELECT owner_id FROM projects WHERE id=?)", id, projectID).Count(&n).Error; err != nil {
				return err
			}
			if n != 1 {
				return ErrForbidden
			}
		}
	}
	q := s.DB.Model(&storage.Project{}).Where("id=? AND version=?", projectID, version).Updates(map[string]any{"preferences": storage.Encode(p), "version": version + 1})
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected == 0 {
		return ErrConflict
	}
	return s.Audit(a, "project.preferences", projectID)
}
func (s *Service) Membership(a Actor, projectID, userID, role string) error {
	if err := s.Authorize(a, projectID, "owner"); err != nil {
		return err
	}
	if role != "" && role != "owner" && role != "operator" && role != "viewer" {
		return ErrInvalid
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var p storage.Project
		if err := tx.First(&p, "id=?", projectID).Error; err != nil {
			return err
		}
		if p.OwnerID == userID {
			return errors.New("project owner membership cannot be removed")
		}
		if role == "" {
			if err := tx.Exec("DELETE FROM memberships WHERE project_id=? AND user_id=?", projectID, userID).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Exec("INSERT INTO memberships(project_id,user_id,role) VALUES (?,?,?) ON CONFLICT(project_id,user_id) DO UPDATE SET role=EXCLUDED.role", projectID, userID, role).Error; err != nil {
				return err
			}
		}
		return s.With(tx).Audit(a, "membership.change", projectID+"/"+userID)
	})
}
func (s *Service) CheckReferences(projectID string, spec jobs.Spec, source jobs.Source) error {
	for _, id := range spec.References(source) {
		var n int64
		if err := s.DB.Table("credential_grants").Where("secret_id=? AND project_id=?", id, projectID).Count(&n).Error; err != nil {
			return err
		}
		if n != 1 {
			return invalid("credential mapping or project grant is missing")
		}
	}
	return nil
}
func (s *Service) CreateSecret(a Actor, name, kind string, value json.RawMessage) (storage.Secret, error) {
	secret := storage.Secret{ID: storage.ID(), OwnerID: a.User.ID, Name: name, Kind: kind, Version: 1}
	if !a.Can("write") || name == "" || !json.Valid(value) {
		return secret, ErrInvalid
	}
	switch kind {
	case "environment", "https", "ssh", "notification", "oauth":
	default:
		return secret, ErrInvalid
	}
	encrypted, err := s.Keys.Encrypt(secret.ID, value)
	if err != nil {
		return secret, err
	}
	secret.Encrypted = storage.Encode(encrypted)
	err = s.DB.Create(&secret).Error
	return secret, err
}
func (s *Service) Grant(a Actor, secretID, projectID string, allow bool) error {
	if !a.Can("write") {
		return ErrForbidden
	}
	var secret storage.Secret
	if err := s.DB.First(&secret, "id=? AND owner_id=?", secretID, a.User.ID).Error; err != nil {
		return ErrForbidden
	}
	if allow {
		if err := s.Authorize(a, projectID, "owner"); err != nil {
			return err
		}
		return s.DB.Exec("INSERT INTO credential_grants VALUES (?,?) ON CONFLICT DO NOTHING", secretID, projectID).Error
	}
	return s.DB.Exec("DELETE FROM credential_grants WHERE secret_id=? AND project_id=?", secretID, projectID).Error
}
func (s *Service) Decrypt(id string) ([]byte, error) {
	var secret storage.Secret
	if err := s.DB.First(&secret, "id=?", id).Error; err != nil {
		return nil, err
	}
	var e security.Encrypted
	if err := json.Unmarshal(secret.Encrypted, &e); err != nil {
		return nil, err
	}
	return s.Keys.Decrypt(id, e)
}
func (s *Service) RotateKeys(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var secrets []storage.Secret
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&secrets).Error; err != nil {
			return err
		}
		for _, secret := range secrets {
			plain, err := s.With(tx).Decrypt(secret.ID)
			if err != nil {
				return err
			}
			e, err := s.Keys.Encrypt(secret.ID, plain)
			if err != nil {
				return err
			}
			if err := tx.Model(&secret).Update("encrypted", storage.Encode(e)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func Decode[T any](raw storage.JSON) (T, error) {
	var v T
	err := strictConfig(raw, &v)
	return v, err
}
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
func CleanError(err error) string {
	if errors.Is(err, ErrApprovalRequired) {
		return ErrApprovalRequired.Error()
	}
	if errors.Is(err, ErrForbidden) {
		return ErrForbidden.Error()
	}
	if errors.Is(err, ErrConflict) {
		return ErrConflict.Error()
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "record does not exist"
	}
	if errors.Is(err, ErrInvalid) {
		return strings.TrimSpace(err.Error())
	}
	return "operation failed"
}

func (s *Service) validateDestinations(owner string, p jobs.Preferences) error {
	if p.Notifications != nil {
		for _, id := range *p.Notifications {
			var n int64
			if err := s.DB.Table("notification_destinations").Where("id=? AND owner_id=?", id, owner).Count(&n).Error; err != nil {
				return err
			}
			if n != 1 {
				return ErrForbidden
			}
		}
	}
	return nil
}
