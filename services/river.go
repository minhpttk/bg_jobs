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
	if riverClientInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		newWorkers := river.NewWorkers()
		// register workers
		jobService := NewJobService(db)
		tasksService := NewTasksService(db)
		river.AddWorker(newWorkers, NewIntervalJobWorker(jobService, tasksService))

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

		return &RiverClient{
			Client: newClient,
		}
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
		UniqueOpts: river.UniqueOpts{
			ByArgs:   true,
			ByPeriod: 15 * time.Minute,
		},
	})
	if err != nil {
		return err
	}
	job.RiverJobID = createdJob.Job.ID
	job.UpdatedAt = time.Now()
	return nil
}
