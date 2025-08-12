package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type JobService struct {
	db *config.Database
}

func NewJobService(db *config.Database) *JobService {
	return &JobService{
		db: db,
	}
}

// CreateJob
func (s *JobService) CreateJob(ctx context.Context, req *models.CreateJobRequest, userId string) (*models.Jobs, error) {
	if err := s.validateJobRequest(req); err != nil {
		return nil, err
	}

	// Set default value for EnableRecovery if not provided
	enableRecovery := true
	if req.EnableRecovery != nil {
		enableRecovery = *req.EnableRecovery
	}

	job := &models.Jobs{
		ID:            uuid.New(),
		Name:          req.Name,
		UserID:        uuid.MustParse(userId),
		WorkspaceID:   req.WorkspaceID,
		Payload:       req.Payload,
		Status:        models.JobStatusActive,
		Type:          req.Type,
		Schedule:      req.Schedule,
		Interval:      req.Interval,
		IsDeleted:     false,
		NextRunAt:     nil,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Version:       1,
		RiverJobID:    0,
		EnableRecovery: enableRecovery,
	}

	if err := s.calculateNextRunTime(job); err != nil {
		return nil, err
	}

	// Begin transaction
	tx := s.db.GORM.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create job in transaction
	if err := tx.Create(job).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Schedule job in River
	if err := GetRiverClientInstance(s.db).ScheduleJobInRiver(ctx, job); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update job with River job ID
	query := `UPDATE jobs SET river_job_id = $1 WHERE id = $2 AND status = 'active' AND is_deleted = false`
	err := tx.Exec(query, job.RiverJobID, job.ID).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return job, nil
}

func (s *JobService) validateJobRequest(req *models.CreateJobRequest) error {
	switch req.Type {
	case models.JobTypeScheduled:
		if req.Schedule == nil {
			return fmt.Errorf("schedule is required for interval jobs")
		}
		var scheduleData models.ScheduleData
		if err := json.Unmarshal([]byte(*req.Schedule), &scheduleData); err != nil {
			return err
		}
		if scheduleData.ExecuteAt == nil {
			return fmt.Errorf("execute_at is required for scheduled jobs")
		}
		return nil
	case models.JobTypeInterval:
		if req.Interval == nil {
			return fmt.Errorf("schedule is required for interval jobs")
		}
		var intervalData models.IntervalData
		if err := json.Unmarshal([]byte(*req.Interval), &intervalData); err != nil {
			return err
		}
		if intervalData.IntervalType == "" {
			return fmt.Errorf("interval_type is required for interval jobs")
		}
		if intervalData.Value == nil {
			return fmt.Errorf("value is required for interval jobs")
		}
		return nil
	}
	return nil
}

func (s *JobService) calculateNextRunTimeForScheduledJob(job *models.Jobs) error {
	var scheduleData models.ScheduleData
	if err := json.Unmarshal([]byte(*job.Schedule), &scheduleData); err != nil {
		return err
	}

	if scheduleData.ExecuteAt == nil {
		return fmt.Errorf("execute_at is required for scheduled jobs")

	}

	if *scheduleData.ExecuteAt == "now" {
		now := time.Now().Add(2 * time.Second)
		job.NextRunAt = &now
		return nil
	}

	// Parse with timezone support
	parsedTime, err := s.parseDateTime(*scheduleData.ExecuteAt)
	if err != nil {
		return fmt.Errorf("failed to parse execute_at '%s': %w", *scheduleData.ExecuteAt, err)
	}

	// Validate not in the past
	if parsedTime.Before(time.Now()) {
		return fmt.Errorf("scheduled time cannot be in the past: %s", parsedTime.Format(time.RFC3339))
	}

	job.NextRunAt = &parsedTime
	log.Printf("Job %s scheduled for: %s", job.ID, parsedTime.Format(time.RFC3339))
	return nil

}

func (s *JobService) calculateNextRunTimeForIntervalJob(job *models.Jobs) error {
	var intervalData models.IntervalData
	if err := json.Unmarshal([]byte(*job.Interval), &intervalData); err != nil {
		return err
	}
	if intervalData.Value == nil {
		return fmt.Errorf("value is required for interval jobs")
	}
	schedule, err := cron.ParseStandard(*intervalData.Value)
	if err != nil {
		return err
	}
	now := time.Now() // VPS timezone
	nextRun := schedule.Next(now)
	job.NextRunAt = &nextRun
	return nil
}

func (s *JobService) parseDateTime(dateStr string) (time.Time, error) {
	supportedFormats := []string{
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range supportedFormats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", dateStr)
}

func (s *JobService) calculateNextRunTime(job *models.Jobs) error {
	switch job.Type {
	case models.JobTypeInterval:
		return s.calculateNextRunTimeForIntervalJob(job)
	case models.JobTypeScheduled:
		return s.calculateNextRunTimeForScheduledJob(job)
	default:
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

}

// GetJobs

type GetJobsRequest struct {
	UserId      string
	WorkspaceId string
	Page        int
	Limit       int
}

type GetJobsResponse struct {
	Data      []models.Jobs `json:"data"`
	Total     int64         `json:"total"`
	TotalPage int           `json:"totalPage"`
	Page      int           `json:"page"`
	Limit     int           `json:"limit"`
}

func (s *JobService) GetJobs(ctx context.Context, req *GetJobsRequest) (*GetJobsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}

	if req.Limit < 1 {
		req.Limit = 10
	}
	if req.Limit > 20 {
		req.Limit = 20 // Maximum limit
	}

	offset := (req.Page - 1) * req.Limit

	var jobs []models.Jobs
	var totalCount int64

	// Get total count
	countResult := s.db.GORM.Model(&models.Jobs{}).
		Where("user_id = ? AND workspace_id = ? AND is_deleted = false", uuid.MustParse(req.UserId), uuid.MustParse(req.WorkspaceId)).
		Count(&totalCount)
	if countResult.Error != nil {
		return nil, fmt.Errorf("failed to count jobs: %w", countResult.Error)
	}

	// Get paginated jobs
	result := s.db.GORM.Where("user_id = ? AND workspace_id = ? AND is_deleted = false", uuid.MustParse(req.UserId), uuid.MustParse(req.WorkspaceId)).
		Offset(offset).
		Limit(req.Limit).
		Order("created_at DESC").
		Find(&jobs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", result.Error)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(req.Limit)))

	return &GetJobsResponse{
		Data:      jobs,
		Total:     totalCount,
		TotalPage: totalPages,
		Page:      req.Page,
		Limit:     req.Limit,
	}, nil
}

// GetJob

type GetJobRequest struct {
	Id        string
	UserId    string
	TaskPage  int `json:"taskPage"`
	TaskLimit int `json:"taskLimit"`
}

type ListTasks struct {
	Data      []models.Tasks `json:"data"`
	Total     int64          `json:"total"`
	TotalPage int            `json:"totalPage"`
	Page      int            `json:"page"`
	Limit     int            `json:"limit"`
}

type GetJobResponse struct {
	Job   *models.Jobs `json:"job"`
	Tasks *ListTasks   `json:"tasks"`
}

func (s *JobService) GetJob(ctx context.Context, req *GetJobRequest) (*GetJobResponse, error) {
	job := &models.Jobs{}
	if err := s.db.GORM.Where("id = ? AND user_id = ? AND is_deleted = false", req.Id, req.UserId).First(job).Error; err != nil {
		return nil, err
	}

	if req.TaskPage < 1 {
		req.TaskPage = 1
	}

	if req.TaskLimit < 1 {
		req.TaskLimit = 10
	}

	taskOffset := (req.TaskPage - 1) * req.TaskLimit

	var tasks []models.Tasks
	var taskTotalCount int64

	// Get total count of tasks for this job
	taskCountResult := s.db.GORM.Model(&models.Tasks{}).Where("job_id = ? AND is_deleted = false", req.Id).Count(&taskTotalCount)
	if taskCountResult.Error != nil {
		return nil, fmt.Errorf("failed to count tasks: %w", taskCountResult.Error)
	}

	// Get paginated tasks
	taskResult := s.db.GORM.Where("job_id = ? AND is_deleted = false", req.Id).
		Offset(taskOffset).
		Limit(req.TaskLimit).
		Order("created_at DESC").
		Find(&tasks)
	if taskResult.Error != nil {
		return nil, fmt.Errorf("failed to fetch tasks: %w", taskResult.Error)
	}

	taskTotalPages := int(math.Ceil(float64(taskTotalCount) / float64(req.TaskLimit)))

	return &GetJobResponse{
		Job: job,
		Tasks: &ListTasks{
			Data:      tasks,
			Total:     taskTotalCount,
			TotalPage: taskTotalPages,
			Page:      req.TaskPage,
			Limit:     req.TaskLimit,
		},
	}, nil
}

func (s *JobService) IsJobActive(ctx context.Context, id uuid.UUID) (bool, error) {
	job := &models.Jobs{}
	if err := s.db.GORM.Where("id = ? AND status = 'active' AND is_deleted = false", id).First(job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("job not found", id)
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteJob
type DeleteJobRequest struct {
	Id     string `json:"id"`
	UserId string `json:"userId"`
}

func (s *JobService) DeleteJob(ctx context.Context, req *DeleteJobRequest) error {
	// Parse user ID once
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	jobID, err := uuid.Parse(req.Id)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	// Begin transaction
	tx := s.db.GORM.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Find and lock the job record
	job := &models.Jobs{}
	if err := tx.Where("id = ? AND user_id = ? AND is_deleted = false", jobID, userID).First(job).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("job not found or access denied")
		}
		return err
	}

	// Soft delete the job
	if err := tx.Model(job).Update("is_deleted", true).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete job from River queue
	if job.RiverJobID != 0 {
		err := tx.Exec(`DELETE FROM river_job WHERE args ->> 'job_id' = ?`, job.ID.String()).Error
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete job from River queue: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (s *JobService) RescheduleIntervalJob(ctx context.Context, job *models.Jobs) error {
	var intervalData models.IntervalData
	if err := json.Unmarshal([]byte(*job.Interval), &intervalData); err != nil {
		return err
	}

	schedule, err := cron.ParseStandard(*intervalData.Value)
	if err != nil {
		return err
	}

	now := time.Now()
	nextRun := schedule.Next(now)
	job.NextRunAt = &nextRun
	job.UpdatedAt = time.Now()

	// Schedule in River
	err = GetRiverClientInstance(s.db).ScheduleJobInRiver(ctx, job)
	if err != nil {
		return err
	}

	// Update database
	query := `UPDATE jobs SET next_run_at = $1, updated_at = $2, river_job_id = $3 WHERE id = $4 AND status = 'active' AND is_deleted = false`
	err = s.db.GORM.Exec(query, job.NextRunAt, job.UpdatedAt, job.RiverJobID, job.ID).Error
	if err != nil {
		return err
	}
	return nil
}

// Pause/Resume jobs
func (s *JobService) PauseJob(ctx context.Context, id uuid.UUID, userId uuid.UUID) error {
	query := `UPDATE jobs SET status = 'inactive', updated_at = $1 WHERE id = $2 AND user_id = $3`
	err := s.db.GORM.Exec(query, time.Now(), id, userId).Error
	return err
}

func (s *JobService) ResumeJob(ctx context.Context, id uuid.UUID, userId uuid.UUID) error {
	query := `UPDATE jobs SET status = 'active', updated_at = $1 WHERE id = $2 AND user_id = $3`
	err := s.db.GORM.Exec(query, time.Now(), id, userId).Error
	return err
}

func (s *JobService) GetJobsForWorker() ([]models.Jobs, error) {
	var jobs []models.Jobs
	// Get paginated jobs
	result := s.db.GORM.Where("status = 'active' AND is_deleted = false").
		Order("updated_at DESC, created_at DESC").
		Find(&jobs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", result.Error)
	}

	log.Println("number of valid jobs", len(jobs))
	return jobs, nil
}

// RescheduleIntervalJob reschedules an interval job for the next run
func (s *JobService) RescheduleIntervalJob(ctx context.Context, job *models.Jobs) error {
	var intervalData models.IntervalData
	if err := json.Unmarshal([]byte(*job.Interval), &intervalData); err != nil {
		return err
	}

	schedule, err := cron.ParseStandard(*intervalData.Value)
	if err != nil {
		return err
	}

	now := time.Now()
	nextRun := schedule.Next(now)
	job.NextRunAt = &nextRun
	job.UpdatedAt = time.Now()

	// Schedule in River
	err = GetRiverClientInstance(s.db).ScheduleJobInRiver(ctx, job)
	if err != nil {
		return err
	}

	// Update database
	query := `UPDATE jobs SET next_run_at = $1, updated_at = $2, river_job_id = $3 WHERE id = $4 AND status = 'active' AND is_deleted = false`
	err = s.db.GORM.Exec(query, job.NextRunAt, job.UpdatedAt, job.RiverJobID, job.ID).Error
	if err != nil {
		return err
	}
	return nil
}

// GetJobIntervalProgress retrieves the current progress of a job interval
func (s *JobService) GetJobIntervalProgress(jobID uuid.UUID) (*models.IntervalProgress, error) {
	job := &models.Jobs{}
	if err := s.db.GORM.Where("id = ? AND type = 'interval' AND status = 'active' AND is_deleted = false", jobID).First(job).Error; err != nil {
		return nil, err
	}

	if job.IntervalProgress == nil {
		return nil, nil
	}

	var progress models.IntervalProgress
	if err := json.Unmarshal([]byte(*job.IntervalProgress), &progress); err != nil {
		return nil, err
	}

	return &progress, nil
}

// UpdateJobIntervalProgress updates the progress of a job interval
func (s *JobService) UpdateJobIntervalProgress(jobID uuid.UUID, progress *models.IntervalProgress) error {
	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return err
	}

	progressStr := string(progressJSON)
	now := time.Now()

	query := `UPDATE jobs SET 
		interval_progress = $1, 
		current_interval_id = $2, 
		interval_started_at = $3,
		updated_at = $4 
		WHERE id = $5 AND type = 'interval' AND status = 'active' AND is_deleted = false`

	return s.db.GORM.Exec(query, progressStr, progress.IntervalID, progress.StartedAt, now, jobID).Error
}

// StartNewInterval starts a new interval execution for a job
func (s *JobService) StartNewInterval(jobID uuid.UUID, totalTasks int) (*models.IntervalProgress, error) {
	intervalID := uuid.New().String()
	now := time.Now()

	progress := &models.IntervalProgress{
		IntervalID:     intervalID,
		TotalTasks:     totalTasks,
		CompletedTasks: 0,
		FailedTasks:    0,
		TaskResults:    make(map[string]models.TaskResult),
		Status:         models.IntervalStatusRunning,
		StartedAt:      now,
		LastUpdatedAt:  now,
	}

	if err := s.UpdateJobIntervalProgress(jobID, progress); err != nil {
		return nil, err
	}

	return progress, nil
}

// CompleteInterval marks an interval as completed
func (s *JobService) CompleteInterval(jobID uuid.UUID, intervalID string) error {
	progress, err := s.GetJobIntervalProgress(jobID)
	if err != nil {
		return err
	}

	if progress == nil || progress.IntervalID != intervalID {
		return fmt.Errorf("interval %s not found for job %s", intervalID, jobID)
	}

	progress.Status = models.IntervalStatusCompleted
	progress.LastUpdatedAt = time.Now()

	return s.UpdateJobIntervalProgress(jobID, progress)
}

// GetIncompleteIntervals retrieves all jobs with incomplete intervals for recovery
func (s *JobService) GetIncompleteIntervals() ([]models.Jobs, error) {
	var jobs []models.Jobs
	query := `SELECT * FROM jobs 
		WHERE type = 'interval' 
		AND status = 'active' 
		AND is_deleted = false 
		AND current_interval_id IS NOT NULL 
		AND interval_progress IS NOT NULL`

	if err := s.db.GORM.Raw(query).Scan(&jobs).Error; err != nil {
		return nil, err
	}

	// Filter jobs with running intervals
	var incompleteJobs []models.Jobs
	for _, job := range jobs {
		progress, err := s.GetJobIntervalProgress(job.ID)
		if err != nil {
			continue
		}
		if progress != nil && progress.Status == models.IntervalStatusRunning {
			incompleteJobs = append(incompleteJobs, job)
		}
	}

	return incompleteJobs, nil
}
