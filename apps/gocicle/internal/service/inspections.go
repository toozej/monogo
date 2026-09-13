package service

import (
	"context"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Inspection struct {
	ID        string       `json:"id"`
	ProjectID string       `json:"projectID"`
	RunnerID  string       `json:"runnerID"`
	Source    storage.JSON `json:"source"`
	Status    string       `json:"status"`
	TokenHash string       `json:"-"`
	ExpiresAt *time.Time   `json:"expiresAt"`
	Deadline  *time.Time   `json:"deadline"`
	Document  storage.JSON `json:"document"`
	Error     string       `json:"error"`
}

func (s *Service) InspectRepository(a Actor, project, runner string) (Inspection, error) {
	row := Inspection{ID: storage.ID(), ProjectID: project, RunnerID: runner, Status: "queued"}
	if err := s.Authorize(a, project, "operator"); err != nil {
		return row, err
	}
	var p storage.Project
	if err := s.DB.First(&p, "id=?", project).Error; err != nil {
		return row, err
	}
	row.Source = p.Source
	var n int64
	if err := s.DB.Table("runner_grants").Where("runner_id=? AND user_id=?", runner, p.OwnerID).Count(&n).Error; err != nil {
		return row, err
	}
	if n != 1 {
		return row, ErrForbidden
	}
	source, err := Decode[jobs.Source](p.Source)
	if err != nil {
		return row, err
	}
	if err := s.CheckReferences(project, jobs.Spec{}, source); err != nil {
		return row, err
	}
	return row, s.DB.Create(&row).Error
}
func (s *Service) PollInspection(r storage.Runner) (*protocol.Assignment, error) {
	var assignment *protocol.Assignment
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var row Inspection
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("runner_id=? AND status='queued'", r.ID).Order("created_at").Take(&row).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		source, err := Decode[jobs.Source](row.Source)
		if err != nil {
			return err
		}
		svc := s.With(tx)
		if err := svc.CheckReferences(row.ProjectID, jobs.Spec{}, source); err != nil {
			return tx.Model(&row).Updates(map[string]any{"status": "failure", "error": "credential grant is unavailable"}).Error
		}
		roots, err := Decode[[]string](r.ApprovedRoots)
		if err != nil {
			return err
		}
		deadline, expiry := time.Now().Add(5*time.Minute), time.Now().Add(protocol.LeaseDuration)
		token := security.Token()
		assignment = &protocol.Assignment{InspectionID: row.ID, RunID: row.ID, Source: source, LeaseToken: token, ExpiresAt: expiry, Deadline: deadline, ApprovedRoots: roots, Credentials: map[string]protocol.Credential{}}
		for _, id := range (jobs.Spec{}).References(source) {
			plain, err := svc.Decrypt(id)
			if err != nil {
				return err
			}
			credential, err := Decode[protocol.Credential](storage.JSON(plain))
			if err != nil {
				return err
			}
			assignment.Credentials[id] = credential
		}
		return tx.Model(&row).Updates(map[string]any{"status": "running", "token_hash": security.Hash(token), "expires_at": expiry, "deadline": deadline}).Error
	})
	return assignment, err
}
func (s *Service) InspectionHeartbeat(r storage.Runner, id, token string) error {
	result := s.DB.Model(&Inspection{}).Where("id=? AND runner_id=? AND token_hash=? AND status='running' AND expires_at>now() AND deadline>now()", id, r.ID, security.Hash(token)).Update("expires_at", gorm.Expr("LEAST(deadline,?)", time.Now().Add(protocol.LeaseDuration)))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrForbidden
	}
	return nil
}
func (s *Service) InspectionReport(r storage.Runner, id, token, yamlText, reportError string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var row Inspection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND runner_id=? AND token_hash=? AND expires_at>now()", id, r.ID, security.Hash(token)).Take(&row).Error; err != nil {
			return ErrForbidden
		}
		if row.Status == "complete" || row.Status == "failure" {
			return nil
		}
		if reportError != "" {
			return tx.Model(&row).Updates(map[string]any{"status": "failure", "error": "repository inspection failed"}).Error
		}
		document, err := jobs.Parse([]byte(yamlText))
		if err != nil {
			return tx.Model(&row).Updates(map[string]any{"status": "failure", "error": "repository job YAML is invalid"}).Error
		}
		return tx.Model(&row).Updates(map[string]any{"status": "complete", "document": storage.Encode(document)}).Error
	})
}
func (s *Service) Inspection(a Actor, id string) (Inspection, error) {
	var row Inspection
	if err := s.DB.First(&row, "id=?", id).Error; err != nil {
		return row, err
	}
	return row, s.Authorize(a, row.ProjectID, "viewer")
}
func (s *Service) expireInspections(ctx context.Context) error {
	return s.DB.WithContext(ctx).Model(&Inspection{}).Where("status='running' AND expires_at<now()").Updates(map[string]any{"status": "lost", "error": "runner lease expired"}).Error
}
