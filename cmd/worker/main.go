package main

import (
	"context"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/services"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {

	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	// Initialize database connection
	db, err := config.NewDatabase()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	defer db.Pool.Close()

	// Initialize River client for job handling
	riverClient := services.GetRiverClientInstance(db).Client
	if err != nil {
		log.Fatal("Failed to create River client: ", err)
	}

	// Start River client
	log.Println("Starting River client...")
	if err := riverClient.Start(context.Background()); err != nil {
		log.Fatalf("River client failed to start: %v", err)
	}

	log.Println("Worker started successfully")

	// Start interval job scheduler
	jobService := services.NewJobService(db)
	go func() {
		// Wait a bit for River to fully initialize
		time.Sleep(5 * time.Second)
		if err := recoverMissedJobs(context.Background(), jobService); err != nil {
			log.Printf("Error recovering missed jobs: %v", err)
		}
	}()
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := riverClient.Stop(ctx); err != nil {
		log.Printf("Error stopping River client: %v", err)
	}

	log.Println("Worker exited")
}

// ✅ ADD: Recover missed jobs on startup
func recoverMissedJobs(ctx context.Context, jobService *services.JobService) error {
	log.Println("Starting job recovery process...")

	jobs, err := jobService.GetJobsForWorker()
	if err != nil {
		return err
	}

	now := time.Now()
	recoveredCount := 0

	for _, job := range jobs {
		// Skip non-interval jobs or inactive jobs
		if job.Type != "interval" || job.Status != "active" || job.IsDeleted {
			continue
		}

		// Check if job should have run but missed
		if job.NextRunAt != nil && job.NextRunAt.Before(now) {
			log.Printf("Recovering missed job: %s (should have run at %v)", job.ID, job.NextRunAt)

			// Reschedule immediately or for next scheduled time
			if err := jobService.RescheduleIntervalJob(ctx, &job); err != nil {
				log.Printf("Failed to recover job %s: %v", job.ID, err)
			} else {
				recoveredCount++
			}
		}
	}

	log.Printf("Job recovery completed. Recovered %d jobs.", recoveredCount)
	return nil
}
