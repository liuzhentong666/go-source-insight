// Package storage provides the production PostgreSQL repositories.
package storage

import (
	"context"
	"errors"
	"time"

	"go-ai-study/api/fault"
	"go-ai-study/api/models"
	"gorm.io/gorm"
)

type Postgres struct {
	db       *gorm.DB
	instance *Instance
}

func (s *Postgres) Ready(ctx context.Context) error {
	if s.instance != nil {
		if err := s.instance.Check(ctx); err != nil {
			return fault.ErrStorage
		}
	}
	db, err := s.db.DB()
	if err != nil {
		return fault.ErrStorage
	}
	if err = db.PingContext(ctx); err != nil {
		return fault.ErrStorage
	}
	return nil
}

func NewPostgres(db *gorm.DB) *Postgres { return &Postgres{db: db} }
func NewPostgresWithInstance(db *gorm.DB, instance *Instance) *Postgres {
	return &Postgres{db: db, instance: instance}
}

func (s *Postgres) OwnedProject(ctx context.Context, id, ownerID uint) (models.Project, error) {
	var project models.Project
	err := s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&project).Error
	return project, storageError(err)
}

func (s *Postgres) OwnedJob(ctx context.Context, id string, ownerID uint) (models.Job, error) {
	var job models.Job
	err := s.db.WithContext(ctx).Model(&models.Job{}).Select("jobs.*").
		Joins("JOIN projects ON projects.id = jobs.project_id").
		Where("jobs.id = ? AND jobs.owner_id = ? AND projects.owner_id = ?", id, ownerID, ownerID).First(&job).Error
	return job, storageError(err)
}

func (s *Postgres) CreateJob(ctx context.Context, job *models.Job) error {
	if job.Kind == models.JobLegacyAnalyze {
		return storageError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			a := models.Analysis{ProjectID: job.ProjectID, Status: models.JobQueued}
			if err := tx.Create(&a).Error; err != nil {
				return err
			}
			job.LegacyAnalysisID = a.ID
			return tx.Create(job).Error
		}))
	}
	return storageError(s.db.WithContext(ctx).Create(job).Error)
}

func (s *Postgres) UpdateJob(ctx context.Context, id, expected string, changes map[string]any) error {
	if status, ok := changes["status"].(string); ok && (status == models.JobCompleted || status == models.JobFailed || status == models.JobCanceled) {
		changes["payload"] = ""
	}
	return storageError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Job{}).Where("id = ? AND status = ?", id, expected).Updates(changes)
		if result.Error != nil {
			return storageError(result.Error)
		}
		if result.RowsAffected != 1 {
			return fault.ErrConflict
		}
		legacy := map[string]any{"status": changes["status"]}
		if v, ok := changes["result"]; ok {
			if v == nil {
				legacy["result"] = changes["failure_reason"]
			} else {
				legacy["result"] = v
			}
		}
		if err := tx.Model(&models.Analysis{}).Where("id = (SELECT legacy_analysis_id FROM jobs WHERE id = ? AND legacy_analysis_id <> 0)", id).Updates(legacy).Error; err != nil {
			return err
		}
		return nil
	}))
}

// RecoverInterrupted is only invoked after exclusive instance ownership. Never
// replay an index: a crash may have happened after alias publication.
func (s *Postgres) RecoverInterrupted(ctx context.Context) error {
	return storageError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		states := []string{models.JobQueued, models.JobRunning, models.JobCanceling}
		if err := tx.Model(&models.Job{}).Where("status IN ?", states).Updates(map[string]any{"status": models.JobFailed, "result": nil, "payload": "", "failure_code": "interrupted", "failure_reason": "execution interrupted; any index publication must be checked before retry", "finished_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Analysis{}).Where("status IN ?", append(states, "pending")).Updates(map[string]any{"status": models.JobFailed, "result": "execution interrupted"}).Error
	}))
}

func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fault.ErrNotFound
	}
	if errors.Is(err, fault.ErrConflict) {
		return fault.ErrConflict
	}
	return fault.ErrStorage
}
