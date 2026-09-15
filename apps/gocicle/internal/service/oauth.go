package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type oauthState struct {
	State providers.State `json:"state"`
}

func (s *Service) Provider(id, publicURL string) (*providers.Adapter, error) {
	var row storage.Provider
	if err := s.DB.First(&row, "id=?", id).Error; err != nil {
		return nil, err
	}
	cfg, err := Decode[providers.Config](row.Config)
	if err != nil {
		return nil, err
	}
	cfg.RedirectURL = publicURL + "/auth/" + id + "/callback"
	secret := ""
	if cfg.SecretID != "" {
		plain, err := s.Decrypt(cfg.SecretID)
		if err != nil {
			return nil, err
		}
		var v struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal(plain, &v); err != nil {
			return nil, err
		}
		secret = v.Value
	}
	if row.Kind == "tangled" {
		cfg.ClientID = publicURL + "/oauth-client-metadata.json"
	}
	return providers.New(row.Kind, cfg, secret)
}
func (s *Service) BeginLogin(ctx context.Context, providerID, publicURL, hint, invitation string, actor *Actor) (string, string, error) {
	adapter, err := s.Provider(providerID, publicURL)
	if err != nil {
		return "", "", err
	}
	state := security.Token()
	location, pending, err := adapter.Begin(ctx, state, hint)
	if err != nil {
		return "", "", err
	}
	encrypted, err := s.Keys.Encrypt(security.Hash(state), storage.Encode(oauthState{State: pending}))
	if err != nil {
		return "", "", err
	}
	var userID, invite any
	if actor != nil {
		if !actor.Session || !actor.Can("write") {
			return "", "", ErrForbidden
		}
		userID = actor.User.ID
	}
	if invitation != "" {
		invite = security.Hash(invitation)
	}
	err = s.DB.Exec("INSERT INTO oauth_states(hash,provider_id,user_id,invitation_hash,encrypted,expires_at) VALUES (?,?,?,?,?,?)", security.Hash(state), providerID, userID, invite, storage.Encode(encrypted), time.Now().Add(10*time.Minute)).Error
	return location, state, err
}
func (s *Service) CompleteLogin(ctx context.Context, providerID, publicURL, state, code, issuer string) (string, string, error) {
	var row loginStateRow
	if err := s.DB.Table("oauth_states").Where("hash=? AND provider_id=? AND expires_at>now()", security.Hash(state), providerID).Take(&row).Error; err != nil {
		return "", "", ErrForbidden
	}
	encrypted, err := Decode[security.Encrypted](row.Encrypted)
	if err != nil {
		return "", "", err
	}
	plain, err := s.Keys.Decrypt(security.Hash(state), encrypted)
	if err != nil {
		return "", "", err
	}
	var pending oauthState
	if err := json.Unmarshal(plain, &pending); err != nil {
		return "", "", err
	}
	adapter, err := s.Provider(providerID, publicURL)
	if err != nil {
		return "", "", err
	}
	profile, accessToken, err := adapter.Exchange(ctx, code, issuer, pending.State)
	if err != nil {
		return "", "", err
	}
	return s.completeIdentity(ctx, providerID, state, profile, accessToken, row)
}

type loginStateRow struct {
	UserID, InvitationHash *string
	Encrypted              storage.JSON
}

func (s *Service) completeIdentity(ctx context.Context, providerID, state string, profile providers.Profile, accessToken *oauth2.Token, row loginStateRow) (string, string, error) {
	if profile.Subject == "" {
		return "", "", ErrForbidden
	}
	token, csrf := security.Token(), security.Token()
	enabled := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deleted := tx.Exec("DELETE FROM oauth_states WHERE hash=? AND expires_at>now()", security.Hash(state))
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ErrForbidden
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", providerID+"/"+profile.Subject).Error; err != nil {
			return err
		}
		var identity struct{ UserID string }
		err := tx.Table("identities").Where("provider_id=? AND subject=?", providerID, profile.Subject).Take(&identity).Error
		if row.UserID != nil && err == nil && identity.UserID != *row.UserID {
			return ErrConflict
		}
		var user storage.User
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if row.UserID != nil {
				if err := tx.First(&user, "id=? AND enabled", *row.UserID).Error; err != nil {
					return ErrForbidden
				}
			} else {
				user = storage.User{ID: storage.ID(), Name: profile.Name, Preferences: storage.Encode(map[string]any{}), Version: 1}

				if err := tx.Create(&user).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec("INSERT INTO identities(id,user_id,provider_id,subject,profile) VALUES (?,?,?,?,?)", storage.ID(), user.ID, providerID, profile.Subject, storage.Encode(profile)).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if err := tx.First(&user, "id=?", identity.UserID).Error; err != nil {
				return err
			}
		}
		if row.InvitationHash != nil && row.UserID == nil {
			var invitation struct{ Administrator bool }
			if err := tx.Table("invitations").Clauses(clause.Locking{Strength: "UPDATE"}).Where("hash=? AND used_at IS NULL AND expires_at>now()", *row.InvitationHash).Take(&invitation).Error; err != nil {
				return ErrForbidden
			}
			user.Enabled = true
			user.Administrator = user.Administrator || invitation.Administrator
			if err := tx.Model(&user).Updates(map[string]any{"enabled": true, "administrator": user.Administrator}).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE invitations SET used_at=now() WHERE hash=?", *row.InvitationHash).Error; err != nil {
				return err
			}
		}
		if accessToken != nil && accessToken.AccessToken != "" {
			var connection storage.Resource
			err := tx.Table("connections").Where("owner_id=? AND provider_id=? AND name=?", user.ID, providerID, "Login metadata").Take(&connection).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			secretID := connection.SecretID
			if secretID == "" {
				secretID = storage.ID()
			}
			tokenData := storage.Encode(map[string]any{"value": accessToken.AccessToken, "subject": profile.Subject, "pds": accessToken.Extra("pds")})
			encrypted, err := s.Keys.Encrypt(secretID, tokenData)
			if err != nil {
				return err
			}
			if connection.ID == "" {
				secret := storage.Secret{ID: secretID, OwnerID: user.ID, Name: "Login metadata", Kind: "oauth", Encrypted: storage.Encode(encrypted), Version: 1}
				if err := tx.Create(&secret).Error; err != nil {
					return err
				}
				if err := tx.Table("connections").Create(map[string]any{"id": storage.ID(), "owner_id": user.ID, "name": "Login metadata", "provider_id": providerID, "secret_id": secretID, "config": storage.Encode(map[string]string{"providerID": providerID}), "version": 1}).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&storage.Secret{}).Where("id=?", secretID).Updates(map[string]any{"encrypted": storage.Encode(encrypted), "version": gorm.Expr("version+1")}).Error; err != nil {
					return err
				}
			}
		}
		enabled = user.Enabled
		if !enabled {
			return nil
		}
		if err := tx.Exec("INSERT INTO sessions(hash,user_id,csrf_hash,expires_at) VALUES (?,?,?,?)", security.Hash(token), user.ID, security.Hash(csrf), time.Now().Add(24*time.Hour)).Error; err != nil {
			return err
		}
		return s.With(tx).Audit(Actor{User: user}, "identity.login", providerID+"/"+profile.Subject)
	})
	if err == nil && !enabled {
		err = ErrApprovalRequired
	}
	return token, csrf, err
}
