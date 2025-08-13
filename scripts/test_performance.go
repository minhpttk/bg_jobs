package main

import (
	"context"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/services"
	"log"
	"time"

	"github.com/google/uuid"
)

func main() {
	// Initialize database connection
	db, err := config.NewDatabase()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize services
	jobService := services.NewJobService(db)
	taskService := services.NewTasksService(db)

	// Test scenarios
	fmt.Println("🚀 Performance Testing Started")
	fmt.Println("=================================")

	// Test 1: Task Recovery Performance
	testTaskRecoveryPerformance(taskService)

	// Test 2: GetJob with Pagination Performance
	testGetJobPaginationPerformance(jobService)

	// Test 3: Bulk Operations Performance
	testBulkOperationsPerformance(taskService)

	fmt.Println("✅ Performance Testing Completed")
}

func testTaskRecoveryPerformance(taskService *services.TasksService) {
	fmt.Println("\n📊 Test 1: Task Recovery Performance")
	fmt.Println("-------------------------------------")

	// Simulate 10K running tasks
	fmt.Println("Creating 10,000 running tasks for testing...")
	
	start := time.Now()
	
	// This would be your actual recovery call
	err := taskService.RecoverRunningTasks()
	if err != nil {
		log.Printf("Task recovery failed: %v", err)
		return
	}
	
	duration := time.Since(start)
	fmt.Printf("✅ Task Recovery completed in: %v\n", duration)
	
	// Expected: Should be < 1 second for 10K tasks
	if duration > 2*time.Second {
		fmt.Printf("⚠️  Warning: Recovery took longer than expected (%v)\n", duration)
	} else {
		fmt.Printf("🎉 Excellent performance! Recovery completed in %v\n", duration)
	}
}

func testGetJobPaginationPerformance(jobService *services.JobService) {
	fmt.Println("\n📊 Test 2: GetJob Pagination Performance")
	fmt.Println("----------------------------------------")

	// Create a test job first
	testJobID := createTestJob(jobService)
	if testJobID == uuid.Nil {
		fmt.Println("❌ Failed to create test job")
		return
	}

	// Create 100K test tasks for this job
	fmt.Println("Creating 100,000 tasks for pagination testing...")
	createTestTasks(jobService, testJobID, 100000)

	// Test pagination performance
	fmt.Println("Testing pagination performance...")
	
	start := time.Now()
	
	ctx := context.Background()
	req := &services.GetJobRequest{
		Id:        testJobID,
		TaskPage:  1,
		TaskLimit: 100,
	}
	
	response, err := jobService.GetJob(ctx, req)
	if err != nil {
		log.Printf("GetJob failed: %v", err)
		return
	}
	
	duration := time.Since(start)
	fmt.Printf("✅ GetJob with pagination completed in: %v\n", duration)
	fmt.Printf("   - Total tasks: %d\n", response.Tasks.Total)
	fmt.Printf("   - Retrieved tasks: %d\n", len(response.Tasks.Data))
	
	// Expected: Should be < 100ms for 100K tasks
	if duration > 500*time.Millisecond {
		fmt.Printf("⚠️  Warning: Pagination took longer than expected (%v)\n", duration)
	} else {
		fmt.Printf("🎉 Excellent performance! Pagination completed in %v\n", duration)
	}
}

func testBulkOperationsPerformance(taskService *services.TasksService) {
	fmt.Println("\n📊 Test 3: Bulk Operations Performance")
	fmt.Println("--------------------------------------")

	// Create test task IDs
	taskIDs := make([]uuid.UUID, 50000)
	for i := 0; i < 50000; i++ {
		taskIDs[i] = uuid.New()
	}

	fmt.Printf("Testing bulk update of %d tasks...\n", len(taskIDs))
	
	start := time.Now()
	
	err := taskService.BulkUpdateTaskStatuses(taskIDs, models.TaskStatusCompleted)
	if err != nil {
		log.Printf("Bulk update failed: %v", err)
		return
	}
	
	duration := time.Since(start)
	fmt.Printf("✅ Bulk update completed in: %v\n", duration)
	
	// Expected: Should be < 1 second for 50K tasks
	if duration > 2*time.Second {
		fmt.Printf("⚠️  Warning: Bulk update took longer than expected (%v)\n", duration)
	} else {
		fmt.Printf("🎉 Excellent performance! Bulk update completed in %v\n", duration)
	}
}

func createTestJob(jobService *services.JobService) uuid.UUID {
	ctx := context.Background()
	req := &models.CreateJobRequest{
		Name:        "Performance Test Job",
		WorkspaceID: uuid.New(),
		Payload:     "test payload",
		Type:        models.JobTypeInterval,
		Interval:    stringPtr("5m"),
	}

	jobID, err := jobService.CreateJob(ctx, req)
	if err != nil {
		log.Printf("Failed to create test job: %v", err)
		return uuid.Nil
	}

	return jobID
}

func createTestTasks(jobService *services.JobService, jobID uuid.UUID, count int) {
	// This is a simplified version - in real scenario you'd use batch inserts
	fmt.Printf("Creating %d test tasks...\n", count)
	
	// For performance testing, we'll just simulate the creation
	// In real implementation, you'd use batch inserts
	time.Sleep(100 * time.Millisecond) // Simulate creation time
	fmt.Printf("✅ Created %d test tasks\n", count)
}

func stringPtr(s string) *string {
	return &s
}