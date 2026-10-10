package models

import (
	"encoding/json"
	"time"
)

const (
	JobAnalyze       = "analyze"
	JobIndex         = "index"
	JobLegacyAnalyze = "legacy_analyze"
	JobQueued        = "queued"
	JobRunning       = "running"
	JobCompleted     = "completed"
	JobFailed        = "failed"
	JobCanceling     = "canceling"
	JobCanceled      = "canceled"
)

// Job is additive to the legacy analyses table. All source references are relative.
type Job struct {
	ID               string          `json:"job_id" gorm:"primaryKey;size:36"`
	OwnerID          uint            `json:"-" gorm:"not null;index"`
	ProjectID        uint            `json:"project_id" gorm:"not null;index"`
	Kind             string          `json:"kind" gorm:"not null;size:32"`
	Target           string          `json:"target" gorm:"not null"`
	Status           string          `json:"status" gorm:"not null;size:32;index"`
	Result           json.RawMessage `json:"result" gorm:"type:jsonb;serializer:json"`
	FailureCode      string          `json:"failure_code,omitempty"`
	FailureReason    string          `json:"failure_reason,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	StartedAt        *time.Time      `json:"started_at"`
	FinishedAt       *time.Time      `json:"finished_at"`
	LegacyAnalysisID uint            `json:"analysis_id,omitempty" gorm:"index"`
	Payload          string          `json:"-" gorm:"type:text"`
}
