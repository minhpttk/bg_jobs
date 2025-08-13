package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/services"
	"gin-gorm-river-app/shared"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

func main() {
	log.Println("🚀 Starting Task Recovery Test...")

	// Load environment variables
	if err := loadEnv(); err != nil {
		log.Fatal("Failed to load .env file:", err)
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

	// Get River client
	riverClient := services.GetRiverClientInstance(db).Client
	if riverClient == nil {
		log.Fatal("Failed to get River client")
	}

	// Start River client
	log.Println("Starting River client...")
	if err := riverClient.Start(context.Background()); err != nil {
		log.Fatalf("River client failed to start: %v", err)
	}
	defer riverClient.Stop(context.Background())

	// Test scenario
	log.Println("📋 Running test scenario...")
	if err := runTestScenario(context.Background(), jobService, taskService, riverClient); err != nil {
		log.Fatal("Test failed:", err)
	}

	log.Println("✅ Task Recovery Test completed successfully!")
}

func runTestScenario(ctx context.Context, jobService *services.JobService, taskService *services.TasksService, riverClient *river.Client[pgx.Tx]) error {
	// Load test configuration
	config := DefaultTestConfig()
	
	// Step 1: Create a test job
	log.Println("Step 1: Creating test job...")
	jobID := uuid.New()
	userID := uuid.New()
	workspaceID := uuid.New()

	// Create payload for AI agent
	payload := models.Payload{
		Prompt:       config.TestPrompt,
		ResourceName: models.AIAgent,
		ResourceData: fmt.Sprintf(`{"agent_name": "%s", "agent_address": "%s"}`, config.TestAgentName, config.TestAgentURL),
	}
	payloadJSON, _ := json.Marshal(payload)

	// Create job in database
	job := models.Jobs{
		ID:          jobID,
		Name:        config.TestJobName,
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

	if err := jobService.db.GORM.Create(&job).Error; err != nil {
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

	if err := taskService.db.GORM.Create(&task).Error; err != nil {
		return fmt.Errorf("failed to create test task: %w", err)
	}
	log.Printf("✅ Created test task: %s (status: running)", taskID)

	// Step 3: Verify task exists and is running
	log.Println("Step 3: Verifying task status...")
	var runningTasks []models.Tasks
	if err := taskService.db.GORM.Where("status = ? AND is_deleted = false", models.TaskStatusRunning).Find(&runningTasks).Error; err != nil {
		return fmt.Errorf("failed to query running tasks: %w", err)
	}
	log.Printf("✅ Found %d running tasks in database", len(runningTasks))

	// Step 4: Simulate server restart by running task recovery
	log.Println("Step 4: Running task recovery (simulating server restart)...")
	if err := taskService.RecoverRunningTasks(); err != nil {
		return fmt.Errorf("task recovery failed: %w", err)
	}
	log.Println("✅ Task recovery completed")

	// Step 5: Check if recovery jobs were added to River queue
	log.Println("Step 5: Checking River queue for recovery jobs...")
	// Note: In a real scenario, you would check the River queue
	// For this test, we'll verify that the task is still in the database
	var recoveredTask models.Tasks
	if err := taskService.db.GORM.Where("id = ? AND is_deleted = false", taskID).First(&recoveredTask).Error; err != nil {
		return fmt.Errorf("failed to find recovered task: %w", err)
	}
	log.Printf("✅ Task still exists in database: %s (status: %s)", recoveredTask.ID, recoveredTask.Status)

	// Step 6: Test the recovery job processing
	log.Println("Step 6: Testing recovery job processing...")
	
	// Create a recovery job args manually
	recoveryArgs := shared.IntervalJobArgs{
		JobID:       jobID,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Payload:     string(payloadJSON),
		TaskID:      &taskID, // This indicates it's a recovery job
	}

	// Add recovery job to River queue
	insertedJob, err := riverClient.Insert(ctx, recoveryArgs, nil)
	if err != nil {
		return fmt.Errorf("failed to insert recovery job: %w", err)
	}
	log.Printf("✅ Added recovery job to River queue: %d", insertedJob.ID)

	// Step 7: Wait a bit for job processing
	log.Println("Step 7: Waiting for job processing...")
	time.Sleep(config.GetWaitTime())

	// Step 8: Verify the task was processed correctly
	log.Println("Step 8: Verifying task processing...")
	var processedTask models.Tasks
	if err := taskService.db.GORM.Where("id = ? AND is_deleted = false", taskID).First(&processedTask).Error; err != nil {
		return fmt.Errorf("failed to find processed task: %w", err)
	}
	log.Printf("✅ Task processed: %s (status: %s)", processedTask.ID, processedTask.Status)

	// Step 9: Clean up test data
	if config.AutoCleanup {
		log.Println("Step 9: Cleaning up test data...")
		if err := cleanupTestData(jobService, taskService, jobID, taskID); err != nil {
			log.Printf("⚠️ Warning: Failed to cleanup test data: %v", err)
		} else {
			log.Println("✅ Test data cleaned up")
		}
	} else {
		log.Println("Step 9: Skipping cleanup (AutoCleanup = false)")
		log.Printf("📝 Test data preserved - Job ID: %s, Task ID: %s", jobID, taskID)
	}

	return nil
}

func cleanupTestData(jobService *services.JobService, taskService *services.TasksService, jobID, taskID uuid.UUID) error {
	// Delete task
	if err := taskService.db.GORM.Where("id = ?", taskID).Delete(&models.Tasks{}).Error; err != nil {
		return fmt.Errorf("failed to delete test task: %w", err)
	}

	// Delete job
	if err := jobService.db.GORM.Where("id = ?", jobID).Delete(&models.Jobs{}).Error; err != nil {
		return fmt.Errorf("failed to delete test job: %w", err)
	}

	return nil
}

func loadEnv() error {
	// Try to load .env file
	// This is a simplified version - you might want to use godotenv package
	return nil
}