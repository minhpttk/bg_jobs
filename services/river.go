package services

import (
	"context"
	"fmt"
	"gin-gorm-river-app/config"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/shared"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

var lock = &sync.Mutex{}

type RiverClient struct {
	Client *river.Client[pgx.Tx]
}

var riverClientInstance *RiverClient

func GetRiverClientInstance(db *config.Database) *RiverClient {
	if riverClientInstance != nil {
		return riverClientInstance
	}

	// Initialize services
	jobService := NewJobService(db)
	tasksService := NewTasksService(db)

	// Create new workers
	newWorkers := river.NewWorkers()

	// Add interval job worker
	river.AddWorker(newWorkers, NewIntervalJobWorker(jobService, tasksService))

	// Add task recovery worker
	river.AddWorker(newWorkers, NewTaskRecoveryWorker(jobService, tasksService))

	maxWorkersInt := 10 // default value
	if maxWorkers := os.Getenv("MAX_WORKERS"); maxWorkers != "" {
		if parsed, err := strconv.Atoi(maxWorkers); err != nil {
			log.Fatal("Failed to convert MAX_WORKERS to int: ", err)
		} else {
			maxWorkersInt = parsed
		}
	}

	newClient, err := river.NewClient(
		riverpgxv5.New(db.Pool),
		&river.Config{
			Queues: map[string]river.QueueConfig{
				river.QueueDefault: {MaxWorkers: maxWorkersInt},
			},
			Workers: newWorkers,
		},
	)

	if err != nil {
		log.Fatal("Failed to create River client: ", err)
		return nil
	}

	riverClientInstance = &RiverClient{
		Client: newClient,
	}
	return riverClientInstance
}

func (s *RiverClient) ScheduleJobInRiver(ctx context.Context, job *models.Jobs) error {
	if job.NextRunAt == nil {
		return fmt.Errorf("next run time not calculated")
	}

	scheduledAt := *job.NextRunAt

	args := shared.IntervalJobArgs{
		JobID:       job.ID,
		UserID:      job.UserID,
		WorkspaceID: job.WorkspaceID,
		Payload:     job.Payload,
	}
	createdJob, err := s.Client.Insert(ctx, args, &river.InsertOpts{
		ScheduledAt: scheduledAt,
		MaxAttempts: 1,
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: 4 * time.Minute, // min interval is 5 minutes => 4 minutes
		},
	})
	if err != nil {
		return err
	}
	job.RiverJobID = createdJob.Job.ID
	job.UpdatedAt = time.Now()
	return nil
}

// ScheduleTaskRecovery schedules a task recovery job
func (s *RiverClient) ScheduleTaskRecovery(ctx context.Context, args shared.TaskRecoveryArgs) error {
	createdJob, err := s.Client.Insert(ctx, args, &river.InsertOpts{
		Queue: river.QueueDefault,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	})
	if err != nil {
		return err
	}

	log.Printf("Scheduled task recovery job: %d for interval %s", createdJob.Job.ID, args.IntervalID)
	return nil
}

// RecoveryService handles automatic recovery of incomplete intervals
type RecoveryService struct {
	jobService *JobService
	riverClient *RiverClient
}

func NewRecoveryService(jobService *JobService, riverClient *RiverClient) *RecoveryService {
	return &RecoveryService{
		jobService:   jobService,
		riverClient:  riverClient,
	}
}

// RecoverIncompleteIntervals finds and schedules recovery for all incomplete intervals
func (rs *RecoveryService) RecoverIncompleteIntervals(ctx context.Context) error {
	log.Println("Starting recovery of incomplete intervals...")

	incompleteJobs, err := rs.jobService.GetIncompleteIntervals()
	if err != nil {
		return fmt.Errorf("failed to get incomplete intervals: %w", err)
	}

	log.Printf("Found %d jobs with incomplete intervals", len(incompleteJobs))

	for _, job := range incompleteJobs {
		progress, err := rs.jobService.GetJobIntervalProgress(job.ID)
		if err != nil {
			log.Printf("Failed to get progress for job %s: %v", job.ID, err)
			continue
		}

		if progress == nil || progress.Status != models.IntervalStatusRunning {
			continue
		}

		// Schedule recovery job
		recoveryArgs := shared.TaskRecoveryArgs{
			JobID:       job.ID,
			IntervalID:  progress.IntervalID,
			UserID:      job.UserID,
			WorkspaceID: job.WorkspaceID,
			Payload:     job.Payload,
		}

		if err := rs.riverClient.ScheduleTaskRecovery(ctx, recoveryArgs); err != nil {
			log.Printf("Failed to schedule recovery for job %s: %v", job.ID, err)
			continue
		}

		log.Printf("Scheduled recovery for job %s, interval %s", job.ID, progress.IntervalID)
	}

	log.Println("Recovery scheduling completed")
	return nil
}
