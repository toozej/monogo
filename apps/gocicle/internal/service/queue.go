package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Tick claims schedules and expires leases. Separate servers can call Tick concurrently.
func (s *Service) Tick(ctx context.Context, now time.Time) error {
	if err := s.expireInspections(ctx); err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			JobID, RevisionID, Expression, Timezone string
			NextAt                                  time.Time
		}
		if err := tx.Table("schedules").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("next_at<=?", now).Order("next_at").Limit(100).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var job storage.Job
			// Match UpdateJob's job lock without waiting for its schedule lock.
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).First(&job, "id=?", row.JobID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			} else if err != nil {
				return err
			}
			schedule, err := jobs.Schedule(row.Expression, row.Timezone)
			if err != nil {
				return err
			}
			status := "queued"
			if row.NextAt.Before(now.Truncate(time.Minute)) {
				status = "missed"
			}
			if _, err := s.With(tx).enqueue(job, &row.NextAt, status); err != nil {
				return err
			}
			if err := tx.Exec("UPDATE schedules SET next_at=? WHERE job_id=?", schedule.Next(now), job.ID).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runs []storage.Run
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status='running' AND id IN (SELECT run_id FROM runner_leases WHERE expires_at < ? AND (expires_at < deadline OR deadline + interval '60 seconds' < ?))", now, now).Find(&runs).Error; err != nil {
			return err
		}
		for _, run := range runs {
			var deadline time.Time
			if err := tx.Table("runner_leases").Where("run_id=?", run.ID).Select("deadline").Scan(&deadline).Error; err != nil {
				return err
			}
			status := "lost"
			if !now.Before(deadline) {
				status = "timeout"
			}
			if err := s.With(tx).finish(&run, protocol.Completion{Status: status, Error: "runner lease expired"}); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) EnrollToken(a Actor, name string, labels, roots []string) (string, error) {
	if err := Admin(a); err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrInvalid
	}
	for _, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
			return "", invalid("approved roots must be absolute directories below /")
		}
	}
	token := security.Token()
	err := s.DB.Exec("INSERT INTO enrollments(hash,name,labels,approved_roots,expires_at) VALUES (?,?,?,?,?)", security.Hash(token), name, storage.Encode(labels), storage.Encode(roots), time.Now().Add(time.Hour)).Error
	return token, err
}
func (s *Service) Enroll(token string) (storage.Runner, string, error) {
	var runner storage.Runner
	credential := security.Token()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var enrollment struct {
			Name                  string
			Labels, ApprovedRoots storage.JSON
		}
		if err := tx.Table("enrollments").Clauses(clause.Locking{Strength: "UPDATE"}).Where("hash=? AND expires_at>now()", security.Hash(token)).Take(&enrollment).Error; err != nil {
			return ErrForbidden
		}
		runner = storage.Runner{ID: storage.ID(), Name: enrollment.Name, Labels: enrollment.Labels, ApprovedRoots: enrollment.ApprovedRoots, TokenHash: security.Hash(credential), Enabled: true, Version: 1}
		if err := tx.Create(&runner).Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM enrollments WHERE hash=?", security.Hash(token)).Error
	})
	return runner, credential, err
}
func (s *Service) Runner(token string) (storage.Runner, error) {
	var r storage.Runner
	err := s.DB.First(&r, "token_hash=? AND enabled", security.Hash(token)).Error
	if err != nil {
		return r, ErrForbidden
	}
	return r, nil
}
func (s *Service) Poll(r storage.Runner) (*protocol.Assignment, error) {
	if assignment, err := s.PollInspection(r); assignment != nil || err != nil {
		return assignment, err
	}
	var assignment *protocol.Assignment
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE runners SET last_seen=now() WHERE id=?", r.ID).Error; err != nil {
			return err
		}
		var runs []storage.Run
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status='queued'").Order("created_at,id").Limit(100).Find(&runs).Error; err != nil {
			return err
		}
		labels, err := Decode[[]string](r.Labels)
		if err != nil {
			return err
		}
		roots, err := Decode[[]string](r.ApprovedRoots)
		if err != nil {
			return err
		}
		for _, run := range runs {
			var rev storage.Revision
			if err := tx.First(&rev, "id=?", run.RevisionID).Error; err != nil {
				return err
			}
			spec, err := Decode[jobs.Spec](rev.Spec)
			if err != nil {
				return err
			}
			source, err := Decode[jobs.Source](rev.Source)
			if err != nil {
				return err
			}
			if !containsAll(labels, spec.Runner.Labels) || (source.Local != nil && source.Local.RunnerID != r.ID) {
				continue
			}
			var allowed int64
			if err := tx.Table("runner_grants").Where("runner_id=? AND user_id=(SELECT owner_id FROM projects WHERE id=?)", r.ID, run.ProjectID).Count(&allowed).Error; err != nil {
				return err
			}
			if allowed == 0 {
				continue
			}
			svc := s.With(tx)
			if err := svc.CheckReferences(run.ProjectID, spec, source); err != nil {
				if err := svc.finish(&run, protocol.Completion{Status: "failure", Error: "credential grant is unavailable"}); err != nil {
					return err
				}
				continue
			}
			if source.Local != nil {
				valid := false
				for _, root := range roots {
					if within(root, source.Local.HostPath) {
						valid = true
					}
				}
				if !valid {
					continue
				}
				// Serialize all local access on a runner. This also protects overlapping paths and aliases.
				var n int64
				if err := tx.Table("run_locks").Where("key=?", "local/"+r.ID).Count(&n).Error; err != nil {
					return err
				}
				if n != 0 {
					continue
				}
			}
			if err := tx.Exec("SELECT pg_advisory_xact_lock(718606945)").Error; err != nil {
				return err
			}
			lock := ""
			if source.Local != nil {
				lock = "local/" + r.ID
			} else if spec.PushChanges {
				lock = "git/" + security.Hash(source.Git.URL+"/"+source.Git.Branch)
			}
			if lock != "" {
				q := tx.Exec("INSERT INTO run_locks(run_id,key) VALUES (?,?) ON CONFLICT(key) DO NOTHING", run.ID, lock)
				if q.Error != nil {
					return q.Error
				}
				if q.RowsAffected == 0 {
					continue
				}
			}
			prefs, err := Decode[jobs.Preferences](run.Preferences)
			if err != nil {
				return err
			}
			token := security.Token()
			duration, _ := time.ParseDuration(spec.Timeout)
			now := time.Now()
			deadline := now.Add(duration)
			assignment = &protocol.Assignment{RunID: run.ID, Spec: spec, Source: source, Preferences: prefs, LeaseToken: token, ExpiresAt: now.Add(protocol.LeaseDuration), Deadline: deadline, ApprovedRoots: roots, Credentials: map[string]protocol.Credential{}}
			for _, id := range spec.References(source) {
				plain, err := svc.Decrypt(id)
				if err != nil {
					return err
				}
				var credential protocol.Credential
				if err := json.Unmarshal(plain, &credential); err != nil {
					return errors.New("credential format is invalid")
				}
				assignment.Credentials[id] = credential
			}
			if err := tx.Exec("INSERT INTO runner_leases(run_id,runner_id,token_hash,expires_at,deadline) VALUES (?,?,?,?,?)", run.ID, r.ID, security.Hash(token), assignment.ExpiresAt, deadline).Error; err != nil {
				return err
			}
			return tx.Model(&run).Updates(map[string]any{"status": "running", "runner_id": r.ID, "started_at": now}).Error
		}
		return nil
	})
	return assignment, err
}
func (s *Service) lease(r storage.Runner, runID, token string, report bool) (storage.Run, error) {
	var run storage.Run
	if err := s.DB.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, "id=? AND runner_id=?", runID, r.ID).Error; err != nil {
		return run, ErrForbidden
	}
	var lease struct{ ExpiresAt, Deadline time.Time }
	if err := s.DB.Table("runner_leases").Where("run_id=? AND runner_id=? AND token_hash=?", runID, r.ID, security.Hash(token)).Take(&lease).Error; err != nil {
		return run, ErrForbidden
	}
	now := time.Now()
	valid := now.Before(lease.ExpiresAt) && now.Before(lease.Deadline)
	if report && !lease.ExpiresAt.Before(lease.Deadline) && now.Before(lease.Deadline.Add(time.Minute)) {
		valid = true
	}
	if !valid {
		return run, ErrForbidden
	}
	return run, nil
}
func (s *Service) Heartbeat(r storage.Runner, runID, token string) (time.Time, bool, error) {
	expiry := time.Now().Add(protocol.LeaseDuration)
	cancel := false
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		run, err := svc.lease(r, runID, token, false)
		if err != nil {
			return err
		}
		cancel = run.CancelRequested || run.Status != "running"
		var rev storage.Revision
		if err := tx.First(&rev, "id=?", run.RevisionID).Error; err != nil {
			return err
		}
		spec, err := Decode[jobs.Spec](rev.Spec)
		if err != nil {
			return err
		}
		source, err := Decode[jobs.Source](rev.Source)
		if err != nil {
			return err
		}
		if svc.CheckReferences(run.ProjectID, spec, source) != nil {
			cancel = true
		}
		return tx.Exec("UPDATE runner_leases SET expires_at=LEAST(?,deadline) WHERE run_id=?", expiry, runID).Error
	})
	return expiry, cancel, err
}
func (s *Service) Report(r storage.Runner, runID, token string, e protocol.Event) error {
	if e.Sequence < 1 || len(e.Data) > 64<<10 || !json.Valid(e.Data) {
		return ErrInvalid
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		run, err := svc.lease(r, runID, token, true)
		if err != nil {
			return err
		}
		var previous struct{ Kind, PayloadHash string }
		requestHash := security.Hash(string(e.Data))
		q := tx.Table("run_events").Where("run_id=? AND sequence=?", runID, e.Sequence).Take(&previous)
		if q.Error == nil {
			if previous.Kind != e.Kind || previous.PayloadHash != requestHash {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return q.Error
		}
		if run.Status != "running" {
			return ErrConflict
		}
		var max int64
		if err := tx.Table("run_events").Where("run_id=?", runID).Select("COALESCE(MAX(sequence),0)").Scan(&max).Error; err != nil {
			return err
		}
		if e.Sequence != max+1 {
			return ErrConflict
		}
		if err := svc.redactReport(run, &e); err != nil {
			return err
		}
		switch e.Kind {
		case "log":
			var content string
			if json.Unmarshal(e.Data, &content) != nil {
				return ErrInvalid
			}
			remaining := int64(protocol.MaxLogBytes) - run.LogBytes
			truncated := int64(len(content)) > remaining
			if truncated {
				content = content[:remaining]
			}
			e.Data = json.RawMessage(storage.Encode(content))
			if err := tx.Exec("INSERT INTO log_chunks VALUES (?,?,?)", runID, e.Sequence, content).Error; err != nil {
				return err
			}
			if err := tx.Model(&run).Updates(map[string]any{"log_bytes": run.LogBytes + int64(len(content)), "log_truncated": run.LogTruncated || truncated}).Error; err != nil {
				return err
			}
		case "truncated":
			if string(e.Data) != "true" {
				return ErrInvalid
			}
			if err := tx.Model(&run).Update("log_truncated", true).Error; err != nil {
				return err
			}
		case "metrics":
			var metrics protocol.Metrics
			if json.Unmarshal(e.Data, &metrics) != nil {
				return ErrInvalid
			}
			if err := tx.Exec("INSERT INTO metric_samples VALUES (?,?,?)", runID, e.Sequence, storage.JSON(e.Data)).Error; err != nil {
				return err
			}
		case "complete":
			var completion protocol.Completion
			if json.Unmarshal(e.Data, &completion) != nil {
				return ErrInvalid
			}
			switch completion.Status {
			case "success", "failure", "timeout", "cancelled", "push_failure":
			default:
				return ErrInvalid
			}
			var deadline time.Time
			if err := tx.Table("runner_leases").Where("run_id=?", runID).Select("deadline").Scan(&deadline).Error; err != nil {
				return err
			}
			if time.Now().After(deadline) {
				completion.Status = "timeout"
			}
			if run.CancelRequested {
				completion.Status = "cancelled"
			}
			if err := svc.finish(&run, completion); err != nil {
				return err
			}
		default:
			return ErrInvalid
		}
		// Store only bounded, redacted data. The hash identifies retries before redaction.
		return tx.Exec("INSERT INTO run_events(run_id,sequence,kind,payload_hash,data) VALUES (?,?,?,?,?)", runID, e.Sequence, e.Kind, requestHash, storage.JSON(e.Data)).Error
	})
}
func (s *Service) finish(run *storage.Run, c protocol.Completion) error {
	if err := s.DB.Model(run).Updates(map[string]any{"status": c.Status, "finished_at": time.Now(), "result": storage.Encode(c), "source_commit": c.SourceCommit, "image_identity": c.ImageIdentity, "push_result": c.PushResult}).Error; err != nil {
		return err
	}
	if err := s.DB.Exec("DELETE FROM run_locks WHERE run_id=?", run.ID).Error; err != nil {
		return err
	}
	prefs, err := Decode[jobs.Preferences](run.Preferences)
	if err != nil {
		return err
	}
	if prefs.Notifications == nil {
		return nil
	}
	event := c.Status
	send := c.Status != "success" && prefs.NotifyFailure != nil && *prefs.NotifyFailure
	if c.Status == "success" {
		send = prefs.NotifySuccess != nil && *prefs.NotifySuccess
		var previous storage.Run
		if err := s.DB.Where("job_id=? AND id<>? AND status IN ('success','failure','timeout','lost','push_failure')", run.JobID, run.ID).Order("created_at DESC").First(&previous).Error; err == nil && previous.Status != "success" && prefs.NotifyRecovery != nil && *prefs.NotifyRecovery {
			event = "recovery"
			send = true
		}
	}
	if send {
		for _, id := range *prefs.Notifications {
			if err := s.DB.Exec("INSERT INTO notification_deliveries(id,run_id,destination_id,event) SELECT ?,?,?,? WHERE EXISTS (SELECT 1 FROM notification_destinations WHERE id=?) ON CONFLICT DO NOTHING", storage.ID(), run.ID, id, event, id).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) Retain(ctx context.Context, age time.Duration) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM runs WHERE finished_at < ?", time.Now().Add(-age)).Error; err != nil {
			return err
		}
		for _, table := range []string{"sessions", "api_tokens", "oauth_states", "enrollments", "invitations"} {
			if err := tx.Exec("DELETE FROM " + table + " WHERE expires_at<now()").Error; err != nil {
				return err
			}
		}
		return tx.Exec("DELETE FROM idempotency_keys WHERE created_at<?", time.Now().Add(-age)).Error
	})
}
func containsAll(have, need []string) bool {
	m := map[string]bool{}
	for _, v := range have {
		m[v] = true
	}
	for _, v := range need {
		if !m[v] {
			return false
		}
	}
	return true
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Service) redactReport(run storage.Run, e *protocol.Event) error {
	var revision storage.Revision
	if err := s.DB.First(&revision, "id=?", run.RevisionID).Error; err != nil {
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
	var secrets []string
	for _, id := range spec.References(source) {
		plain, err := s.Decrypt(id)
		if err != nil {
			continue
		}
		var credential protocol.Credential
		if json.Unmarshal(plain, &credential) == nil {
			secrets = append(secrets, credential.Value, credential.Password, credential.PrivateKey)
		}
	}
	var value any
	if err := json.Unmarshal(e.Data, &value); err != nil {
		return err
	}
	var redact func(any) any
	redact = func(v any) any {
		switch x := v.(type) {
		case string:
			return security.Redact(x, secrets)
		case map[string]any:
			for k, y := range x {
				x[k] = redact(y)
			}
		case []any:
			for i, y := range x {
				x[i] = redact(y)
			}
		}
		return v
	}
	e.Data = json.RawMessage(storage.Encode(redact(value)))
	return nil
}
