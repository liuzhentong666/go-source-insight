package models

import "time"

type Analysis struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ProjectID uint      `json:"project_id" gorm:"not null"`
	Status    string    `json:"status" gorm:"not null"` // pending, running, completed, failed
	Result    string    `json:"result" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AnalysisRequest struct {
	ProjectID uint   `json:"project_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
}

type AnalysisResponse struct {
	ID        uint      `json:"id"`
	ProjectID uint      `json:"project_id"`
	Status    string    `json:"status"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}