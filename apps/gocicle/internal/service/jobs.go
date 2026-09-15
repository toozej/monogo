package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Import struct {
	Document    jobs.Document     `json:"document"`
	Versions    map[string]int64  `json:"versions"`
	Credentials map[string]string `json:"credentials"`
}

func (s *Service) Import(a Actor, projectID, key string, input Import) ([]storage.Job, error) {
	if key == "" {
		return nil, invalid("Idempotency-Key is required")
	}
	original := storage.Encode(input)
	input = Import{}
	if err := json.Unmarshal(original, &input); err != nil {
		return nil, err
	}
	if err := input.Document.Validate(); err != nil {
		return nil, invalid("%s", err)
	}
	request := storage.Encode(map[string]any{"projectID": projectID, "import": input})
	var result []storage.Job
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		if err := svc.Authorize(a, projectID, "operator"); err != nil {
			return err
		}
		cached, err := svc.idempotency(a, key, request, &result)
		if err != nil || cached {
			return err
		}
		var project storage.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id=?", projectID).Error; err != nil {
			return err
		}
		source := input.Document.Project.Source
		if source.Git != nil && source.Git.Credential != "" {
			mapped, ok := input.Credentials[source.Git.Credential]
			if !ok {
				return invalid("map git credential explicitly")
			}
			source.Git.Credential = mapped
		}
		previous, err := Decode[jobs.Source](project.Source)
		if err != nil {
			return err
		}
		if !bytes.Equal(storage.Encode(previous), storage.Encode(source)) {
			var names []string
			if err := tx.Model(&storage.Job{}).Where("project_id=?", projectID).Pluck("name", &names).Error; err != nil {
				return err
			}
			for _, name := range names {
				if _, exists := input.Document.Jobs[name]; !exists {
					return invalid("include every existing job when changing the project source")
				}
			}
		}
		for _, name := range input.Document.Names() {
			spec := input.Document.Jobs[name]
			for env, ref := range spec.Secrets {
				mapped, ok := input.Credentials[ref]
				if !ok {
					return invalid("map credential %s explicitly", ref)
				}
				spec.Secrets[env] = mapped
			}
			if err := svc.CheckReferences(projectID, spec, source); err != nil {
				return err
			}
			var job storage.Job
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("project_id=? AND name=?", projectID, name).Take(&job).Error
			switch {
			case err == nil:
				version, ok := input.Versions[name]
				if !ok || version != job.Version {
					return ErrConflict
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				job = storage.Job{ID: storage.ID(), ProjectID: projectID, Name: name}
				if err := tx.Omit("revision_id").Create(&job).Error; err != nil {
					return err
				}
			default:
				return err
			}
			if err := svc.revise(a, &job, spec, source, false); err != nil {
				return err
			}
			result = append(result, job)
		}
		if err := tx.Model(&project).Updates(map[string]any{"source": storage.Encode(source), "version": project.Version + 1}).Error; err != nil {
			return err
		}
		if err := svc.Audit(a, "jobs.import", projectID); err != nil {
			return err
		}
		return svc.remember(a, key, request, result)
	})
	return result, err
}
func (s *Service) revise(a Actor, job *storage.Job, spec jobs.Spec, source jobs.Source, enabled bool) error {
	revision := storage.Revision{ID: storage.ID(), JobID: job.ID, Spec: storage.Encode(spec), Source: storage.Encode(source), CreatedBy: a.User.ID}
	if err := s.DB.Create(&revision).Error; err != nil {
		return err
	}
	job.RevisionID = revision.ID
	job.Version++
	job.Enabled = enabled
	if err := s.DB.Model(job).Updates(map[string]any{"revision_id": job.RevisionID, "version": job.Version, "enabled": enabled}).Error; err != nil {
		return err
	}
	if err := s.DB.Exec("DELETE FROM schedules WHERE job_id=?", job.ID).Error; err != nil {
		return err
	}
	if enabled && !spec.Paused && spec.Schedule != "" {
		schedule, err := jobs.Schedule(spec.Schedule, spec.Timezone)
		if err != nil {
			return err
		}
		return s.DB.Exec("INSERT INTO schedules(job_id,revision_id,expression,timezone,next_at) VALUES (?,?,?,?,?)", job.ID, revision.ID, spec.Schedule, spec.Timezone, schedule.Next(time.Now())).Error
	}
	return nil
}
func (s *Service) UpdateJob(a Actor, id string, version int64, spec jobs.Spec, enabled bool) (storage.Job, error) {
	var job storage.Job
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id=?", id).Error; err != nil {
			return err
		}
		if err := svc.Authorize(a, job.ProjectID, "operator"); err != nil {
			return err
		}
		if version != job.Version {
			return ErrConflict
		}
		var old storage.Revision
		if err := tx.First(&old, "id=?", job.RevisionID).Error; err != nil {
			return err
		}
		source, err := Decode[jobs.Source](old.Source)
		if err != nil {
			return err
		}
		spec.Defaults()
		if err := spec.Validate(source); err != nil {
			return invalid("%s", err)
		}
		if err := svc.CheckReferences(job.ProjectID, spec, source); err != nil {
			return err
		}
		if err := svc.revise(a, &job, spec, source, enabled); err != nil {
			return err
		}
		return svc.Audit(a, "job.update", id)
	})
	return job, err
}
func (s *Service) Export(a Actor, projectID string) (jobs.Document, error) {
	var d jobs.Document
	if err := s.Authorize(a, projectID, "viewer"); err != nil {
		return d, err
	}
	var project storage.Project
	if err := s.DB.First(&project, "id=?", projectID).Error; err != nil {
		return d, err
	}
	source, err := Decode[jobs.Source](project.Source)
	if err != nil {
		return d, err
	}
	d = jobs.Document{APIVersion: jobs.APIVersion, Project: jobs.Project{Name: project.Name, Source: source}, Jobs: map[string]jobs.Spec{}}
	var rows []struct {
		Name string
		Spec storage.JSON
	}
	if err := s.DB.Table("jobs").Select("jobs.name, job_revisions.spec").Joins("JOIN job_revisions ON job_revisions.id=jobs.revision_id").Where("jobs.project_id=?", projectID).Scan(&rows).Error; err != nil {
		return d, err
	}
	for _, row := range rows {
		spec, err := Decode[jobs.Spec](row.Spec)
		if err != nil {
			return d, err
		}
		d.Jobs[row.Name] = spec
	}
	return d, nil
}
func (s *Service) Start(a Actor, jobID, key string) (storage.Run, error) {
	var run storage.Run
	if key == "" {
		return run, invalid("Idempotency-Key is required")
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		var job storage.Job
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id=?", jobID).Error; err != nil {
			return err
		}
		if err := svc.Authorize(a, job.ProjectID, "operator"); err != nil {
			return err
		}
		cached, err := svc.idempotency(a, key, storage.Encode(jobID), &run)
		if cached || err != nil {
			return err
		}
		var rev storage.Revision
		if err := tx.First(&rev, "id=?", job.RevisionID).Error; err != nil {
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
		if err := svc.CheckReferences(job.ProjectID, spec, source); err != nil {
			return err
		}
		if !job.Enabled {
			return invalid("enable the validated job before execution")
		}
		run, err = svc.enqueue(job, nil, "queued")
		if err != nil {
			return err
		}
		if err := svc.Audit(a, "run.start", run.ID); err != nil {
			return err
		}
		return svc.remember(a, key, storage.Encode(jobID), run)
	})
	return run, err
}
func (s *Service) enqueue(job storage.Job, scheduled *time.Time, status string) (storage.Run, error) {
	var p storage.Project
	var owner storage.User
	if err := s.DB.First(&p, "id=?", job.ProjectID).Error; err != nil {
		return storage.Run{}, err
	}
	if err := s.DB.First(&owner, "id=?", p.OwnerID).Error; err != nil {
		return storage.Run{}, err
	}
	op, err := Decode[jobs.Preferences](owner.Preferences)
	if err != nil {
		return storage.Run{}, err
	}
	pp, err := Decode[jobs.Preferences](p.Preferences)
	if err != nil {
		return storage.Run{}, err
	}
	var n int64
	if err := s.DB.Model(&storage.Run{}).Where("job_id=? AND status IN ('queued','running')", job.ID).Count(&n).Error; err != nil {
		return storage.Run{}, err
	}
	if n > 0 && status == "queued" {
		status = "skipped"
	}
	run := storage.Run{ID: storage.ID(), JobID: job.ID, RevisionID: job.RevisionID, ProjectID: job.ProjectID, Preferences: storage.Encode(jobs.Resolve(op, pp)), Status: status, ScheduledAt: scheduled, CreatedAt: time.Now(), Result: storage.Encode(map[string]any{})}
	if status != "queued" {
		now := time.Now()
		run.FinishedAt = &now
	}
	return run, s.DB.Create(&run).Error
}
func (s *Service) Cancel(a Actor, id string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		svc := s.With(tx)
		var run storage.Run
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&run, "id=?", id).Error; err != nil {
			return err
		}
		if err := svc.Authorize(a, run.ProjectID, "operator"); err != nil {
			return err
		}
		if run.Status == "queued" {
			return tx.Model(&run).Updates(map[string]any{"status": "cancelled", "finished_at": time.Now(), "cancel_requested": true}).Error
		}
		if run.Status != "running" {
			return nil
		}
		if err := tx.Model(&run).Update("cancel_requested", true).Error; err != nil {
			return err
		}
		return svc.Audit(a, "run.cancel", id)
	})
}
func (s *Service) idempotency(a Actor, key string, request storage.JSON, result any) (bool, error) {
	if len(key) > 200 {
		return false, ErrInvalid
	}
	if err := s.DB.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", a.User.ID+"/"+key).Error; err != nil {
		return false, err
	}
	var entry struct {
		RequestHash string
		Response    storage.JSON
	}
	err := s.DB.Table("idempotency_keys").Where("actor_id=? AND key=?", a.User.ID, key).Take(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if entry.RequestHash != security.Hash(string(request)) {
		return false, ErrConflict
	}
	return true, json.Unmarshal(entry.Response, result)
}
func (s *Service) remember(a Actor, key string, request storage.JSON, response any) error {
	return s.DB.Exec("INSERT INTO idempotency_keys(actor_id,key,request_hash,response) VALUES (?,?,?,?)", a.User.ID, key, security.Hash(string(request)), storage.Encode(response)).Error
}
