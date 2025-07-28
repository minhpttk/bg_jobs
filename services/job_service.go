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

func (s *JobService) calculateNextRunTime(job *models.Jobs) error {

	if job.Type == models.JobTypeInterval {
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
		now := time.Now()
		nextRun := schedule.Next(now)
		job.NextRunAt = &nextRun
		return nil
	}

	if job.Type == models.JobTypeScheduled {
		var scheduleData models.ScheduleData
		if err := json.Unmarshal([]byte(*job.Schedule), &scheduleData); err != nil {
			return err
		}
		if scheduleData.ExecuteAt != nil {
			if *scheduleData.ExecuteAt == "now" {
				now := time.Now().Add(2 * time.Second)
				job.NextRunAt = &now
				return nil
			} else {
				executeAt, err := time.ParseInLocation("2006-01-02T15:04", *scheduleData.ExecuteAt, time.Local)
				if err != nil {
					return err
				}
				job.NextRunAt = &executeAt
				return nil
			}
		}

		return fmt.Errorf("execute_at is required for scheduled jobs")
	}

	return nil

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
	countResult := s.db.GORM.Model(&models.Jobs{}).Where("user_id = ? AND workspace_id = ? AND is_deleted = false", uuid.MustParse(req.UserId), uuid.MustParse(req.WorkspaceId)).Count(&totalCount)
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
	if err := s.db.GORM.Where("id = ? AND is_deleted = false", req.Id).First(job).Error; err != nil {
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
	if err := s.db.GORM.Model(&models.Jobs{}).Where("id = ? AND user_id = ? AND is_deleted = false", req.Id, uuid.MustParse(req.UserId)).
		Update("is_deleted", true).Error; err != nil {
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
func (s *JobService) PauseJob(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE jobs SET status = 'inactive', updated_at = $1 WHERE id = $2`
	err := s.db.GORM.Exec(query, time.Now(), id).Error
	return err
}

func (s *JobService) ResumeJob(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE jobs SET status = 'active', updated_at = $1 WHERE id = $2`
	err := s.db.GORM.Exec(query, time.Now(), id).Error
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

func (s *JobService) UpdateJobLastRun(ctx context.Context, jobID uuid.UUID) error {
	now := time.Now()
	query := `UPDATE jobs SET last_run_at = $1, updated_at = $2 WHERE id = $3 AND status = 'active' AND is_deleted = false`
	err := s.db.GORM.Exec(query, now, now, jobID).Error
	return err
}
