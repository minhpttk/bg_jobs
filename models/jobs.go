package models

import (
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the status of a job
//
//	@Description	Job status enumeration
type JobStatus string

const (
	JobStatusActive   JobStatus = "active"
	JobStatusInactive JobStatus = "inactive"
	JobStatusDeleted  JobStatus = "deleted"
)

type TaskStatus string

const (
	TaskStatusCreated   TaskStatus = "created"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type ResourceName string

const (
	AIAgent     ResourceName = "ai_agent"
	ClientAgent ResourceName = "client_agent"
)

type ScheduleData struct {
	Value     *string `json:"value"`
	ExecuteAt *string `json:"execute_at"`
}

type IntervalData struct {
	IntervalType string  `json:"interval_type" validate:"required,oneof=minutes, hours, days, months"`
	Value        *string `json:"value"`
}

type Payload struct {
	Prompt       string       `json:"prompt" validate:"required"`
	ResourceName ResourceName `json:"resource_name" validate:"required,oneof=ai_agent client_agent"`
	ResourceData string       `json:"resource_data" validate:"required"`
}

type JobType string

const (
	JobTypeScheduled JobType = "scheduled"
	JobTypeInterval  JobType = "interval"
)

type Jobs struct {
	ID          uuid.UUID  `gorm:"primaryKey" db:"id" json:"id"`
	Name        string     `gorm:"not null" db:"name" json:"name" default:"Job"`
	UserID      uuid.UUID  `gorm:"not null" db:"user_id" json:"user_id"`
	WorkspaceID uuid.UUID  `gorm:"not null" db:"workspace_id" json:"workspace_id"`
	Payload     string     `gorm:"not null" db:"payload" json:"payload"`
	Status      JobStatus  `gorm:"not null;default:active" db:"status" json:"status"`
	Type        JobType    `gorm:"not null" db:"type" json:"type"`
	Schedule    *string    `db:"schedule" json:"schedule"`
	Interval    *string    `db:"interval" json:"interval"`
	IsDeleted   bool       `gorm:"not null;default:false" db:"is_deleted" json:"is_deleted"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty" db:"next_run_at"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty" db:"last_run_at"`
	CreatedAt   time.Time  `gorm:"not null" db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null" db:"updated_at" json:"updated_at"`
	Version     int64      `gorm:"not null" db:"version" json:"version"`
	RiverJobID  int64      `gorm:"not null" db:"river_job_id" json:"river_job_id"`
}

type Tasks struct {
	ID        uuid.UUID  `gorm:"primaryKey" db:"id" json:"id"`
	JobID     uuid.UUID  `gorm:"not null" db:"job_id" json:"job_id"`
	Status    TaskStatus `gorm:"not null;default:created" db:"status" json:"status"`
	Payload   string     `gorm:"not null" db:"payload" json:"payload"`
	Result    string     `db:"result" json:"result"`
	IsDeleted bool       `gorm:"not null;default:false" db:"is_deleted" json:"is_deleted"`
	CreatedAt time.Time  `gorm:"not null" db:"created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null" db:"updated_at" json:"updated_at"`
	Version   int64      `gorm:"not null" db:"version" json:"version"`
}

// Create Job Request DTO
type CreateJobRequest struct {
	Name        string    `json:"name" validate:"required"`
	WorkspaceID uuid.UUID `json:"workspace_id" validate:"required"`
	Payload     string    `json:"payload" validate:"required"`
	Type        JobType   `json:"type" binding:"required,oneof=scheduled interval"`
	Schedule    *string   `json:"schedule,omitempty"`
	Interval    *string   `json:"interval,omitempty"`
}

// Create Job Response DTO
type CreateJobResponse struct {
	JobID uuid.UUID `json:"job_id"`
}
