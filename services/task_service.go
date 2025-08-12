package services

import (
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
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

	// Group tasks by job ID to efficiently clear current_task_id
	jobIDs := make(map[uuid.UUID]bool)
	
	// Reset all running tasks to created status so they can be re-executed
	for _, task := range runningTasks {
		updateResult := s.db.GORM.Model(&models.Tasks{}).
			Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"status":     models.TaskStatusCreated,
				"result":     "", // Clear any partial results
				"updated_at": time.Now(),
			})

		if updateResult.Error != nil {
			log.Printf("Failed to recover task %s: %v", task.ID, updateResult.Error)
			continue
		}

		jobIDs[task.JobID] = true
		log.Printf("Recovered task %s (job: %s) from running to created status", task.ID, task.JobID)
	}

	// ✅ ADD: Clear current_task_id for all jobs that had running tasks
	for jobID := range jobIDs {
		clearResult := s.db.GORM.Model(&models.Jobs{}).
			Where("id = ? AND status = 'active' AND is_deleted = false", jobID).
			Updates(map[string]interface{}{
				"current_task_id": nil,
				"updated_at":      time.Now(),
			})

		if clearResult.Error != nil {
			log.Printf("Failed to clear current_task_id for job %s: %v", jobID, clearResult.Error)
		} else {
			log.Printf("Cleared current_task_id for job %s", jobID)
		}
	}

	log.Printf("Task recovery completed. Recovered %d tasks from %d jobs", len(runningTasks), len(jobIDs))
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
