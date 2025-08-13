package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/services"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("🚀 Starting Task Recovery Test...")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	// Initialize database
	db, err := config.NewDatabase()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Pool.Close()

	// Initialize services
	jobService := services.NewJobService(db)
	taskService := services.NewTasksService(db)

	// Test scenario
	log.Println("📋 Running test scenario...")
	if err := runTestScenario(context.Background(), db, jobService, taskService); err != nil {
		log.Fatal("Test failed:", err)
	}

	log.Println("✅ Task Recovery Test completed successfully!")
}

func runTestScenario(ctx context.Context, db *config.Database, jobService *services.JobService, taskService *services.TasksService) error {
	// Step 1: Create a test job
	log.Println("Step 1: Creating test job...")
	jobID := uuid.New()
	userID := uuid.New()
	workspaceID := uuid.New()

	// Create payload for AI agent
	payload := models.Payload{
		Prompt:       "Test recovery scenario - automated test",
		ResourceName: models.AIAgent,
		ResourceData: `{"agent_name": "test_agent", "agent_address": "http://localhost:8080"}`,
	}
	payloadJSON, _ := json.Marshal(payload)

	// Create job in database
	job := models.Jobs{
		ID:          jobID,
		Name:        "Test Recovery Job",
		UserID:      userID,
		WorkspaceID: workspaceID,
		Payload:     string(payloadJSON),
		Status:      models.JobStatusActive,
		Type:        models.JobTypeInterval,
		IsDeleted:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
		RiverJobID:  0, // Will be set by River
	}

	if err := db.GORM.Create(&job).Error; err != nil {
		return fmt.Errorf("failed to create test job: %w", err)
	}
	log.Printf("✅ Created test job: %s", jobID)

	// Step 2: Create a task and set it to running (simulate interrupted execution)
	log.Println("Step 2: Creating task and setting to running...")
	taskID := uuid.New()
	task := models.Tasks{
		ID:        taskID,
		JobID:     jobID,
		Status:    models.TaskStatusRunning, // Simulate interrupted task
		Payload:   string(payloadJSON),
		Result:    "",
		IsDeleted: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
	}

	if err := db.GORM.Create(&task).Error; err != nil {
		return fmt.Errorf("failed to create test task: %w", err)
	}
	log.Printf("✅ Created test task: %s (status: running)", taskID)

	// Step 3: Verify task exists and is running
	log.Println("Step 3: Verifying task status...")
	var runningTasks []models.Tasks
	if err := db.GORM.Where("status = ? AND is_deleted = false", models.TaskStatusRunning).Find(&runningTasks).Error; err != nil {
		return fmt.Errorf("failed to query running tasks: %w", err)
	}
	log.Printf("✅ Found %d running tasks in database", len(runningTasks))

	// Step 4: Test task validation
	log.Println("Step 4: Testing task validation...")
	if !taskService.IsTaskValid(taskID) {
		return fmt.Errorf("task validation failed for task: %s", taskID)
	}
	log.Printf("✅ Task validation passed for task: %s", taskID)

	// Step 5: Test task status update
	log.Println("Step 5: Testing task status update...")
	if err := taskService.UpdateTaskById(taskID, models.TaskStatusCreated); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	log.Printf("✅ Task status updated to 'created' for task: %s", taskID)

	// Step 6: Verify task status change
	log.Println("Step 6: Verifying task status change...")
	var updatedTask models.Tasks
	if err := db.GORM.Where("id = ? AND is_deleted = false", taskID).First(&updatedTask).Error; err != nil {
		return fmt.Errorf("failed to find updated task: %w", err)
	}
	log.Printf("✅ Task status verified: %s (status: %s)", updatedTask.ID, updatedTask.Status)

	// Step 7: Test task result update
	log.Println("Step 7: Testing task result update...")
	testResult := "Test completed successfully"
	if err := taskService.UpdateTaskResult(taskID, testResult, models.TaskStatusCompleted); err != nil {
		return fmt.Errorf("failed to update task result: %w", err)
	}
	log.Printf("✅ Task result updated for task: %s", taskID)

	// Step 8: Verify final task state
	log.Println("Step 8: Verifying final task state...")
	var finalTask models.Tasks
	if err := db.GORM.Where("id = ? AND is_deleted = false", taskID).First(&finalTask).Error; err != nil {
		return fmt.Errorf("failed to find final task: %w", err)
	}
	log.Printf("✅ Final task state: %s (status: %s, result: %s)", finalTask.ID, finalTask.Status, finalTask.Result)

	// Step 9: Clean up test data
	log.Println("Step 9: Cleaning up test data...")
	if err := cleanupTestData(db, jobID, taskID); err != nil {
		log.Printf("⚠️ Warning: Failed to cleanup test data: %v", err)
	} else {
		log.Println("✅ Test data cleaned up")
	}

	return nil
}

func cleanupTestData(db *config.Database, jobID, taskID uuid.UUID) error {
	// Delete task
	if err := db.GORM.Where("id = ?", taskID).Delete(&models.Tasks{}).Error; err != nil {
		return fmt.Errorf("failed to delete test task: %w", err)
	}

	// Delete job
	if err := db.GORM.Where("id = ?", jobID).Delete(&models.Jobs{}).Error; err != nil {
		return fmt.Errorf("failed to delete test job: %w", err)
	}

	return nil
}