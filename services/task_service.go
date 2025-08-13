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
	for _, task := range runningTasks {
		jobIDs[task.JobID] = true
	}

	// ✅ OPTIMIZED: Batch update all running tasks in a single query
	taskIDs := make([]uuid.UUID, len(runningTasks))
	for i, task := range runningTasks {
		taskIDs[i] = task.ID
	}

	// Batch update all tasks to created status
	batchUpdateResult := s.db.GORM.Model(&models.Tasks{}).
		Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{
			"status":     models.TaskStatusCreated,
			"result":     "", // Clear any partial results
			"updated_at": time.Now(),
		})

	if batchUpdateResult.Error != nil {
		log.Printf("Failed to batch update tasks: %v", batchUpdateResult.Error)
		return fmt.Errorf("failed to batch update tasks: %w", batchUpdateResult.Error)
	}

	log.Printf("Batch updated %d tasks from running to created status", batchUpdateResult.RowsAffected)

	// ✅ OPTIMIZED: Batch clear current_task_id for all affected jobs
	jobIDList := make([]uuid.UUID, 0, len(jobIDs))
	for jobID := range jobIDs {
		jobIDList = append(jobIDList, jobID)
	}

	batchClearResult := s.db.GORM.Model(&models.Jobs{}).
		Where("id IN ? AND status = 'active' AND is_deleted = false", jobIDList).
		Updates(map[string]interface{}{
			"current_task_id": nil,
			"updated_at":      time.Now(),
		})

	if batchClearResult.Error != nil {
		log.Printf("Failed to batch clear current_task_id: %v", batchClearResult.Error)
	} else {
		log.Printf("Batch cleared current_task_id for %d jobs", batchClearResult.RowsAffected)
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

// ✅ ADD: Bulk update task statuses for better performance
func (s *TasksService) BulkUpdateTaskStatuses(taskIDs []uuid.UUID, status models.TaskStatus) error {
	if len(taskIDs) == 0 {
		return nil
	}

	result := s.db.GORM.Model(&models.Tasks{}).
		Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to bulk update task statuses: %w", result.Error)
	}

	log.Printf("Bulk updated %d tasks to status %s", result.RowsAffected, status)
	return nil
}

// ✅ ADD: Get tasks with pagination and count in single query
func (s *TasksService) GetTasksWithPagination(jobID uuid.UUID, page, limit int) ([]models.Tasks, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	var taskResults []struct {
		models.Tasks
		TotalCount int64 `json:"total_count"`
	}

	result := s.db.GORM.Raw(`
		WITH task_data AS (
			SELECT *,
				   COUNT(*) OVER() as total_count
			FROM tasks 
			WHERE job_id = ? AND is_deleted = false
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		)
		SELECT * FROM task_data
	`, jobID, limit, offset).Scan(&taskResults)

	if result.Error != nil {
		return nil, 0, fmt.Errorf("failed to fetch tasks with pagination: %w", result.Error)
	}

	// Extract tasks and total count
	tasks := make([]models.Tasks, len(taskResults))
	var totalCount int64
	for i, taskResult := range taskResults {
		tasks[i] = taskResult.Tasks
		if i == 0 {
			totalCount = taskResult.TotalCount
		}
	}

	return tasks, totalCount, nil
}
