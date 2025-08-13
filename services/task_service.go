package services

import (
	"context"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/shared"
	"log"
	"time"

	"github.com/google/uuid"
)

type TasksService struct {
	db *config.Database
}

func NewTasksService(db *config.Database) *TasksService {
	return &TasksService{
		db: db,
	}
}

func (s *TasksService) CreateTask(jobID uuid.UUID, payload string) (uuid.UUID, error) {

	taskID := uuid.New()
	task := models.Tasks{
		ID:        taskID,
		JobID:     jobID,
		Payload:   payload,
		Status:    models.TaskStatusCreated,
		Result:    "", // Initialize empty result
		IsDeleted: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	if s.db == nil || s.db.GORM == nil {
		log.Printf("Database connection is nil for job %s", jobID)
		return uuid.Nil, fmt.Errorf("database connection is nil")
	}

	result := s.db.GORM.Create(&task)
	if result.Error != nil {
		log.Printf("Failed to create task for job %s: %v", jobID, result.Error)
		return uuid.Nil, result.Error
	}

	return taskID, nil
}

// UpdateTaskById updates the task status by job ID (fixed method)
func (s *TasksService) UpdateTaskById(taskID uuid.UUID, status models.TaskStatus) error {
	result := s.db.GORM.Model(&models.Tasks{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task with task ID %s not found", taskID)
	}

	return nil
}

// updateTaskResult updates the task result by job ID
func (s *TasksService) UpdateTaskResult(taskID uuid.UUID, result string, status models.TaskStatus) error {
	updateResult := s.db.GORM.Model(&models.Tasks{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":     status,
			"result":     result,
			"updated_at": time.Now(),
		})

	if updateResult.Error != nil {
		return updateResult.Error
	}

	if updateResult.RowsAffected == 0 {
		return fmt.Errorf("task with job ID %s not found", taskID)
	}

	return nil
}

// RecoverRunningTasks recovers tasks that were running when server restarted
func (s *TasksService) RecoverRunningTasks() error {
	// Find all tasks that were running when server restarted
	var runningTasks []models.Tasks
	result := s.db.GORM.Where("status = ? AND is_deleted = false", models.TaskStatusRunning).Find(&runningTasks)
	if result.Error != nil {
		return fmt.Errorf("failed to fetch running tasks: %w", result.Error)
	}

	if len(runningTasks) == 0 {
		log.Println("No running tasks found to recover")
		return nil
	}

	log.Printf("Found %d running tasks to recover", len(runningTasks))

	// Get River client to add recovery jobs
	riverClient := GetRiverClientInstance(s.db).Client
	if riverClient == nil {
		return fmt.Errorf("river client not available for task recovery")
	}

	recoveredCount := 0
	
	// Add recovery jobs to riverqueue for each running task
	for _, task := range runningTasks {
		// Get job information to create recovery args
		var job models.Jobs
		if err := s.db.GORM.Where("id = ? AND is_deleted = false", task.JobID).First(&job).Error; err != nil {
			log.Printf("Failed to get job info for task %s: %v", task.ID, err)
			continue
		}

		// Create recovery job args with task ID
		recoveryArgs := shared.IntervalJobArgs{
			JobID:       task.JobID,
			UserID:      job.UserID,
			WorkspaceID: job.WorkspaceID,
			Payload:     task.Payload,
			TaskID:      &task.ID, // ✅ ADD: Pass task ID for recovery
		}

		// Add recovery job to riverqueue
		_, err := riverClient.Insert(context.Background(), recoveryArgs, nil)
		if err != nil {
			log.Printf("Failed to add recovery job for task %s: %v", task.ID, err)
			continue
		}

		recoveredCount++
		log.Printf("Added recovery job for task %s (job: %s)", task.ID, task.JobID)
	}

	log.Printf("Task recovery completed. Added %d recovery jobs to queue", recoveredCount)
	return nil
}

// ✅ ADD: Get incomplete tasks by job ID
func (s *TasksService) GetIncompleteTasksByJobID(jobID uuid.UUID) ([]models.Tasks, error) {
	var tasks []models.Tasks
	result := s.db.GORM.Where("job_id = ? AND status IN (?, ?) AND is_deleted = false", 
		jobID, models.TaskStatusCreated, models.TaskStatusRunning).
		Order("created_at ASC").
		Find(&tasks)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to fetch incomplete tasks for job %s: %w", jobID, result.Error)
	}
	
	return tasks, nil
}

// ✅ ADD: Check if task is valid for recovery
func (s *TasksService) IsTaskValid(taskID uuid.UUID) bool {
	var task models.Tasks
	result := s.db.GORM.Where("id = ? AND is_deleted = false", taskID).First(&task)
	if result.Error != nil {
		log.Printf("Task %s not found or invalid: %v", taskID, result.Error)
		return false
	}
	
	// Check if task status is valid for recovery
	validStatuses := []models.TaskStatus{
		models.TaskStatusCreated,
		models.TaskStatusRunning,
	}
	
	for _, status := range validStatuses {
		if task.Status == status {
			return true
		}
	}
	
	log.Printf("Task %s has invalid status for recovery: %s", taskID, task.Status)
	return false
}
