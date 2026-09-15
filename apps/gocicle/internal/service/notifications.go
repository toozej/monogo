package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/pkg/notification"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) Notify(ctx context.Context, hosts []string) error {
	return s.dispatch(ctx, func(svc *Service, ctx context.Context, id, subject, message string) error {
		return svc.sendNotification(ctx, id, subject, message, hosts)
	})
}
func (s *Service) dispatch(ctx context.Context, send func(*Service, context.Context, string, string, string) error) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID, RunID, DestinationID, Event string
			Attempts                        int
		}
		if err := tx.Table("notification_deliveries").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("delivered_at IS NULL AND attempts<8 AND next_at<=now()").Order("next_at").Limit(10).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			subject := "gocicle: " + row.Event
			message := fmt.Sprintf("Run %s ended with event %s.", row.RunID, row.Event)
			sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := send(s.With(tx), sendCtx, row.DestinationID, subject, message)
			cancel()
			values := map[string]any{"attempts": row.Attempts + 1, "next_at": time.Now().Add(time.Duration(1<<row.Attempts) * 30 * time.Second)}
			if err == nil {
				values["delivered_at"] = time.Now()
				values["last_error"] = ""
			} else {
				values["last_error"] = "notification delivery failed"
			}
			if err := tx.Table("notification_deliveries").Where("id=?", row.ID).Updates(values).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) sendNotification(ctx context.Context, id, subject, message string, allowed []string) error {
	var destination storage.Resource
	if err := s.DB.Table("notification_destinations").Where("id=?", id).Take(&destination).Error; err != nil {
		return err
	}
	var cfg notification.Config
	if err := json.Unmarshal(destination.Config, &cfg); err != nil {
		return err
	}
	if cfg.Endpoint != "" {
		u, err := url.Parse(cfg.Endpoint)
		if err != nil || u.Scheme != "https" || u.User != nil {
			return errors.New("notification endpoint must use HTTPS")
		}
		approved := false
		for _, host := range allowed {
			if u.Host == host {
				approved = true
			}
		}
		if !approved {
			return errors.New("notification endpoint is not approved")
		}
	}
	plain, err := s.Decrypt(destination.SecretID)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(plain, &cfg.Credential); err != nil {
		return err
	}
	cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	notifier, err := notification.New(cfg)
	if err != nil {
		return errors.New("notification configuration is invalid")
	}
	if err := notifier.Notify(ctx, subject, message); err != nil {
		return errors.New("notification delivery failed")
	}
	return nil
}
func (s *Service) TestNotification(ctx context.Context, a Actor, id string, hosts []string) error {
	if !a.Can("write") {
		return ErrForbidden
	}
	var n int64
	if err := s.DB.Table("notification_destinations").Where("id=? AND owner_id=?", id, a.User.ID).Count(&n).Error; err != nil {
		return err
	}
	if n != 1 {
		return ErrForbidden
	}
	return s.sendNotification(ctx, id, "gocicle: notification test", "The notification destination accepted a test request.", hosts)
}
