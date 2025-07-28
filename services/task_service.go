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
