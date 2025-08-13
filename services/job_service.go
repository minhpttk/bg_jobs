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

	job := &models.Jobs{
		ID:          uuid.New(),
		Name:        req.Name,
		UserID:      uuid.MustParse(userId),
		WorkspaceID: req.WorkspaceID,
		Payload:     req.Payload,
		Type:        req.Type,
		Schedule:    req.Schedule,
		Interval:    req.Interval,
		NextRunAt:   nil,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
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

// UpdateJob updates a job based on the request
// If only prompt is updated, it updates the existing job
// If time or target is updated, it creates a new job and deletes the old one
func (s *JobService) UpdateJob(ctx context.Context, jobID string, req *models.UpdateJobRequest, userID string) (*models.UpdateJobResponse, error) {
	// Parse job ID
	jobUUID, err := uuid.Parse(jobID)
	if err != nil {
		return nil, fmt.Errorf("invalid job ID: %w", err)
	}

	// Get existing job
	existingJob := &models.Jobs{}
	if err := s.db.GORM.Where("id = ? AND user_id = ? AND is_deleted = false", jobUUID, userID).First(existingJob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Check if we need to create a new job (time or target changes)
	needsNewJob := false
	
	// Check if type is being changed
	if req.Type != nil && *req.Type != existingJob.Type {
		needsNewJob = true
	}
	
	// Check if schedule is being changed
	if req.Schedule != nil && (existingJob.Schedule == nil || *req.Schedule != *existingJob.Schedule) {
		needsNewJob = true
	}
	
	// Check if interval is being changed
	if req.Interval != nil && (existingJob.Interval == nil || *req.Interval != *existingJob.Interval) {
		needsNewJob = true
	}

	if needsNewJob {
		// Create new job with updated parameters
		newJob := &models.Jobs{
			ID:          uuid.New(),
			Name:        existingJob.Name,
			UserID:      existingJob.UserID,
			WorkspaceID: existingJob.WorkspaceID,
			Payload:     existingJob.Payload,
			Type:        existingJob.Type,
			Schedule:    existingJob.Schedule,
			Interval:    existingJob.Interval,
			NextRunAt:   nil,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Version:     1,
		}

		// Update fields from request
		if req.Name != nil {
			newJob.Name = *req.Name
		}
		if req.Payload != nil {
			newJob.Payload = *req.Payload
		}
		if req.Type != nil {
			newJob.Type = *req.Type
		}
		if req.Schedule != nil {
			newJob.Schedule = req.Schedule
		}
		if req.Interval != nil {
			newJob.Interval = req.Interval
		}

		// Validate the new job configuration
		createReq := &models.CreateJobRequest{
			Name:        newJob.Name,
			WorkspaceID: newJob.WorkspaceID,
			Payload:     newJob.Payload,
			Type:        newJob.Type,
			Schedule:    newJob.Schedule,
			Interval:    newJob.Interval,
		}
		if err := s.validateJobRequest(createReq); err != nil {
			return nil, fmt.Errorf("invalid job configuration: %w", err)
		}

		// Calculate next run time for new job
		if err := s.calculateNextRunTime(newJob); err != nil {
			return nil, fmt.Errorf("failed to calculate next run time: %w", err)
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

		// Delete old job (soft delete)
		if err := tx.Model(&models.Jobs{}).Where("id = ?", existingJob.ID).Update("is_deleted", true).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete old job: %w", err)
		}

		// Create new job
		if err := tx.Create(newJob).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create new job: %w", err)
		}

		// Schedule new job in River
		if err := GetRiverClientInstance(s.db).ScheduleJobInRiver(ctx, newJob); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to schedule new job: %w", err)
		}

		// Update job with River job ID
		query := `UPDATE jobs SET river_job_id = $1 WHERE id = $2 AND status = 'active' AND is_deleted = false`
		if err := tx.Exec(query, newJob.RiverJobID, newJob.ID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update river job ID: %w", err)
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return &models.UpdateJobResponse{
			JobID:    newJob.ID,
			IsNewJob: true,
			Message:  "Job updated successfully. New job created due to time/target changes.",
		}, nil
	} else {
		// Only update prompt and name (no time/target changes)
		updates := make(map[string]interface{})
		updates["updated_at"] = time.Now()

		if req.Name != nil {
			updates["name"] = *req.Name
		}
		if req.Payload != nil {
			updates["payload"] = *req.Payload
		}

		// Update job in database
		if err := s.db.GORM.Model(&models.Jobs{}).Where("id = ? AND user_id = ? AND is_deleted = false", jobUUID, userID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update job: %w", err)
		}

		return &models.UpdateJobResponse{
			JobID:    existingJob.ID,
			IsNewJob: false,
			Message:  "Job updated successfully.",
		}, nil
	}
}

// ✅ ADD: Update current task ID for job
func (s *JobService) UpdateCurrentTaskID(ctx context.Context, jobID uuid.UUID, taskID *uuid.UUID) error {
	query := `UPDATE jobs SET current_task_id = $1, updated_at = $2 WHERE id = $3 AND status = 'active' AND is_deleted = false`
	err := s.db.GORM.Exec(query, taskID, time.Now(), jobID).Error
	if err != nil {
		return fmt.Errorf("failed to update current task ID for job %s: %w", jobID, err)
	}
	return nil
}

// ✅ ADD: Check if job has running tasks
func (s *JobService) HasRunningTasks(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var count int64
	result := s.db.GORM.Model(&models.Tasks{}).
		Where("job_id = ? AND status = ? AND is_deleted = false", jobID, models.TaskStatusRunning).
		Count(&count)
	
	if result.Error != nil {
		return false, fmt.Errorf("failed to check running tasks for job %s: %w", jobID, result.Error)
	}
	
	return count > 0, nil
}

// ✅ ADD: Get current task information for a job
func (s *JobService) GetCurrentTask(ctx context.Context, jobID uuid.UUID) (*models.Tasks, error) {
	// First get the job to check current_task_id
	job := &models.Jobs{}
	if err := s.db.GORM.Where("id = ? AND status = 'active' AND is_deleted = false", jobID).First(job).Error; err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	if job.CurrentTaskID == nil {
		return nil, nil // No current task
	}

	// Get the current task
	task := &models.Tasks{}
	if err := s.db.GORM.Where("id = ? AND is_deleted = false", job.CurrentTaskID).First(task).Error; err != nil {
		return nil, fmt.Errorf("current task not found: %w", err)
	}

	return task, nil
}
