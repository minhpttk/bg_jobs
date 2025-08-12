package services

import (
	"context"
	"encoding/json"
	"fmt"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/shared"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type TaskRecoveryWorker struct {
	jobService   *JobService
	tasksService *TasksService
	river.WorkerDefaults[shared.TaskRecoveryArgs]
}

func NewTaskRecoveryWorker(jobService *JobService, tasksService *TasksService) *TaskRecoveryWorker {
	return &TaskRecoveryWorker{
		jobService:   jobService,
		tasksService: tasksService,
	}
}

func (w *TaskRecoveryWorker) Work(ctx context.Context, job *river.Job[shared.TaskRecoveryArgs]) error {
	log.Printf("Starting task recovery for job: %s, interval: %s", job.Args.JobID, job.Args.IntervalID)

	// Get current progress
	progress, err := w.jobService.GetJobIntervalProgress(job.Args.JobID)
	if err != nil {
		log.Printf("Failed to get progress for job %s: %v", job.Args.JobID, err)
		return err
	}

	if progress == nil || progress.IntervalID != job.Args.IntervalID {
		log.Printf("Progress not found or interval ID mismatch for job %s", job.Args.JobID)
		return nil
	}

	// Check if interval is still running
	if progress.Status != models.IntervalStatusRunning {
		log.Printf("Interval %s for job %s is not running (status: %s)", job.Args.IntervalID, job.Args.JobID, progress.Status)
		return nil
	}

	// Find incomplete tasks
	incompleteTasks := w.findIncompleteTasks(progress)
	if len(incompleteTasks) == 0 {
		log.Printf("No incomplete tasks found for interval %s", job.Args.IntervalID)
		// Mark interval as completed
		return w.jobService.CompleteInterval(job.Args.JobID, job.Args.IntervalID)
	}

	log.Printf("Found %d incomplete tasks for recovery", len(incompleteTasks))

	// Process incomplete tasks
	for _, taskID := range incompleteTasks {
		if err := w.processRecoveryTask(ctx, job.Args, taskID); err != nil {
			log.Printf("Failed to process recovery task %s: %v", taskID, err)
			// Continue with other tasks
			continue
		}
	}

	// Check if all tasks are now completed
	updatedProgress, err := w.jobService.GetJobIntervalProgress(job.Args.JobID)
	if err != nil {
		return err
	}

	if updatedProgress != nil && 
	   updatedProgress.IntervalID == job.Args.IntervalID &&
	   (updatedProgress.CompletedTasks+updatedProgress.FailedTasks) >= updatedProgress.TotalTasks {
		log.Printf("All tasks completed for interval %s", job.Args.IntervalID)
		return w.jobService.CompleteInterval(job.Args.JobID, job.Args.IntervalID)
	}

	return nil
}

func (w *TaskRecoveryWorker) findIncompleteTasks(progress *models.IntervalProgress) []string {
	var incompleteTasks []string

	for taskID, taskResult := range progress.TaskResults {
		if taskResult.Status != models.TaskStatusCompleted && taskResult.Status != models.TaskStatusFailed {
			incompleteTasks = append(incompleteTasks, taskID)
		}
	}

	return incompleteTasks
}

func (w *TaskRecoveryWorker) processRecoveryTask(ctx context.Context, args shared.TaskRecoveryArgs, taskID string) error {
	log.Printf("Processing recovery task: %s", taskID)

	// Get task result from progress
	progress, err := w.jobService.GetJobIntervalProgress(args.JobID)
	if err != nil {
		return err
	}

	taskResult, exists := progress.TaskResults[taskID]
	if !exists {
		return fmt.Errorf("task %s not found in progress", taskID)
	}

	// Skip if task is already completed or failed
	if taskResult.Status == models.TaskStatusCompleted || taskResult.Status == models.TaskStatusFailed {
		return nil
	}

	// Update task status to running
	taskResult.Status = models.TaskStatusRunning
	taskResult.StartedAt = time.Now()
	progress.TaskResults[taskID] = taskResult
	progress.LastUpdatedAt = time.Now()

	if err := w.jobService.UpdateJobIntervalProgress(args.JobID, progress); err != nil {
		return err
	}

	// Process the task
	payload := models.Payload{}
	if err := json.Unmarshal([]byte(args.Payload), &payload); err != nil {
		return err
	}

	var processErr error
	var result interface{}

	switch payload.ResourceName {
	case models.AIAgent:
		result, processErr = processAIAgentJob(shared.ProcessJobArgs{
			JobID:       args.JobID,
			TaskID:      uuid.MustParse(taskID),
			UserID:      args.UserID,
			WorkspaceID: args.WorkspaceID,
			Payload:     args.Payload,
		}, w.tasksService)
	case models.ClientAgent:
		result, processErr = processClientAgentJob(shared.ProcessJobArgs{
			JobID:       args.JobID,
			TaskID:      uuid.MustParse(taskID),
			UserID:      args.UserID,
			WorkspaceID: args.WorkspaceID,
			Payload:     args.Payload,
		}, w.tasksService)
	default:
		processErr = fmt.Errorf("unknown resource type: %s", payload.ResourceName)
	}

	// Update task result
	endedAt := time.Now()
	taskResult.EndedAt = &endedAt

	if processErr != nil {
		taskResult.Status = models.TaskStatusFailed
		taskResult.Error = processErr.Error()
		progress.FailedTasks++
	} else {
		taskResult.Status = models.TaskStatusCompleted
		if str, ok := result.(string); ok {
			taskResult.Result = str
		} else {
			resultJSON, _ := json.Marshal(result)
			taskResult.Result = string(resultJSON)
		}
		progress.CompletedTasks++
	}

	progress.TaskResults[taskID] = taskResult
	progress.LastUpdatedAt = time.Now()

	return w.jobService.UpdateJobIntervalProgress(args.JobID, progress)
}