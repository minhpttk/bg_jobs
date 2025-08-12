package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gin-gorm-river-app/models"
	"gin-gorm-river-app/shared"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type IntervalJobWorker struct {
	jobService   *JobService
	tasksService *TasksService
	river.WorkerDefaults[shared.IntervalJobArgs]
}

func NewIntervalJobWorker(jobService *JobService, tasksService *TasksService) *IntervalJobWorker {
	return &IntervalJobWorker{
		jobService:   jobService,
		tasksService: tasksService,
	}
}

func (w *IntervalJobWorker) Work(ctx context.Context, job *river.Job[shared.IntervalJobArgs]) error {
	log.Printf("Executing scheduled job: (ID: %s)", job.Args.JobID)

	isExistedJob, err := w.jobService.IsJobActive(ctx, job.Args.JobID)
	if err != nil {
		log.Printf("Failed to check if job is active: %v", err)
		return err
	}
	if !isExistedJob {
		_ = river.JobCancel(fmt.Errorf("Job %s is no longer active", job.Args.JobID))
		return nil
	}

	payload := models.Payload{}
	if err := json.Unmarshal([]byte(job.Args.Payload), &payload); err != nil {
		log.Printf("Failed to unmarshal payload: %v", err)
		return err
	}

	// Check if there's an incomplete interval for this job
	progress, err := w.jobService.GetJobIntervalProgress(job.Args.JobID)
	if err != nil {
		log.Printf("Failed to get job progress: %v", err)
		return err
	}

	// If there's a running interval, schedule recovery instead of starting new one
	if progress != nil && progress.Status == models.IntervalStatusRunning {
		log.Printf("Found incomplete interval %s for job %s, scheduling recovery", progress.IntervalID, job.Args.JobID)
		
		recoveryArgs := shared.TaskRecoveryArgs{
			JobID:       job.Args.JobID,
			IntervalID:  progress.IntervalID,
			UserID:      job.Args.UserID,
			WorkspaceID: job.Args.WorkspaceID,
			Payload:     job.Args.Payload,
		}

		if err := GetRiverClientInstance(w.jobService.db).ScheduleTaskRecovery(ctx, recoveryArgs); err != nil {
			log.Printf("Failed to schedule recovery: %v", err)
			return err
		}

		// Reschedule the job for next run
		w.rescheduleJobIfNeeded(ctx, job.Args.JobID)
		return nil
	}

	// Start new interval execution
	totalTasks := w.calculateTotalTasks(payload)
	progress, err = w.jobService.StartNewInterval(job.Args.JobID, totalTasks)
	if err != nil {
		log.Printf("Failed to start new interval: %v", err)
		return err
	}

	log.Printf("Started new interval %s for job %s with %d tasks", progress.IntervalID, job.Args.JobID, totalTasks)

	// Execute tasks for this interval
	if err := w.executeIntervalTasks(ctx, job.Args, progress); err != nil {
		log.Printf("Failed to execute interval tasks: %v", err)
		return err
	}

	// Mark interval as completed
	if err := w.jobService.CompleteInterval(job.Args.JobID, progress.IntervalID); err != nil {
		log.Printf("Failed to complete interval: %v", err)
		return err
	}

	log.Printf("Job %s completed successfully", job.Args.JobID)
	
	// Reschedule job for next run
	w.rescheduleJobIfNeeded(ctx, job.Args.JobID)
	return nil
}

// calculateTotalTasks determines how many tasks need to be executed for this job
func (w *IntervalJobWorker) calculateTotalTasks(payload models.Payload) int {
	// For now, we'll use a simple approach based on resource type
	// In a real implementation, this could be more sophisticated
	switch payload.ResourceName {
	case models.AIAgent:
		return 1 // AI agent jobs typically have 1 task
	case models.ClientAgent:
		// Client agent jobs might have multiple tasks based on the plan
		// For now, we'll assume 1 task, but this could be parsed from the payload
		return 1
	default:
		return 1
	}
}

// executeIntervalTasks executes all tasks for an interval
func (w *IntervalJobWorker) executeIntervalTasks(ctx context.Context, jobArgs shared.IntervalJobArgs, progress *models.IntervalProgress) error {
	payload := models.Payload{}
	if err := json.Unmarshal([]byte(jobArgs.Payload), &payload); err != nil {
		return err
	}

	// For now, we'll execute a single task per interval
	// In a more sophisticated implementation, this could handle multiple tasks
	taskID := uuid.New().String()
	
	// Initialize task result
	taskResult := models.TaskResult{
		TaskID:    taskID,
		Status:    models.TaskStatusCreated,
		StartedAt: time.Now(),
	}
	progress.TaskResults[taskID] = taskResult
	progress.LastUpdatedAt = time.Now()

	if err := w.jobService.UpdateJobIntervalProgress(jobArgs.JobID, progress); err != nil {
		return err
	}

	// Execute the task
	var processErr error
	var result interface{}

	switch payload.ResourceName {
	case models.AIAgent:
		result, processErr = processAIAgentJob(shared.ProcessJobArgs{
			JobID:       jobArgs.JobID,
			TaskID:      uuid.MustParse(taskID),
			UserID:      jobArgs.UserID,
			WorkspaceID: jobArgs.WorkspaceID,
			Payload:     jobArgs.Payload,
		}, w.tasksService)
	case models.ClientAgent:
		result, processErr = processClientAgentJob(shared.ProcessJobArgs{
			JobID:       jobArgs.JobID,
			TaskID:      uuid.MustParse(taskID),
			UserID:      jobArgs.UserID,
			WorkspaceID: jobArgs.WorkspaceID,
			Payload:     jobArgs.Payload,
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

	return w.jobService.UpdateJobIntervalProgress(jobArgs.JobID, progress)
}

// ✅ ADD: Helper function to reschedule interval jobs
func (w *IntervalJobWorker) rescheduleJobIfNeeded(ctx context.Context, jobID uuid.UUID) {
	// Get the job from database
	dbJob := &models.Jobs{}
	if err := w.jobService.db.GORM.Where("id = ? AND type = 'interval' AND status = 'active' AND is_deleted = false", jobID).First(dbJob).Error; err != nil {
		return
	}

	// Only reschedule interval jobs
	if dbJob.Type != models.JobTypeInterval {
		log.Printf("Job %s is not an interval job, skipping reschedule", jobID)
		return
	}

	// Check if schedule is for recurring jobs (not one_time)
	if dbJob.Interval != nil {
		var intervalData models.IntervalData
		if err := json.Unmarshal([]byte(*dbJob.Interval), &intervalData); err != nil {
			log.Printf("Failed to unmarshal schedule for job %s: %v", jobID, err)
			return
		}
	}

	// Reschedule the job
	if err := w.jobService.RescheduleIntervalJob(ctx, dbJob); err != nil {
		log.Printf("Failed to reschedule interval job %s: %v", jobID, err)
	} else {
		log.Printf("Successfully rescheduled interval job %s for next run at %v", jobID, dbJob.NextRunAt)
	}
}

func processClientAgentJob(jobArgs shared.ProcessJobArgs, tasksService *TasksService) (interface{}, error) {
	payload := models.Payload{}
	if err := json.Unmarshal([]byte(jobArgs.Payload), &payload); err != nil {
		return nil, err
	}

	err := tasksService.UpdateTaskById(jobArgs.TaskID, models.TaskStatusRunning)
	if err != nil {
		log.Printf("Failed to update task status to running: %v", err)
		return nil, err
	}

	clientAgentData := models.ClientAgentData{}
	if err := json.Unmarshal([]byte(payload.ResourceData), &clientAgentData); err != nil {
		return nil, err
	}

	// Prepare the request payload
	requestBody := shared.ClientAgentRequest{
		Message: payload.Prompt,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// Make HTTP POST request to client agent
	agentURL := clientAgentData.URL + "/messages"
	resp, err := http.Post(agentURL, "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var errorResponse map[string]interface{}
		json.Unmarshal(bodyBytes, &errorResponse)

		return nil, errors.New(errorResponse["error"].(string))
	}

	// Parse response
	var responseData struct {
		Content   []shared.IAgentTask `json:"content"`
		ReplyType string              `json:"replyType"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return nil, err
	}

	if responseData.ReplyType == "agent_plan" {
		// Handle agent_plan reply type
		// Parse the plans from the content
		plans := responseData.Content

		// Sort plans by step number
		sort.Slice(plans, func(i, j int) bool {
			stepI := plans[i].Step
			stepJ := plans[j].Step
			return stepI < stepJ
		})
		var tasksWithDependencies, tasksWithoutDependencies []shared.IAgentTask

		for index, plan := range plans {
			if index == 0 || len(plan.Dependencies) > 0 {
				tasksWithDependencies = append(tasksWithDependencies, plan)
			} else {
				tasksWithoutDependencies = append(tasksWithoutDependencies, plan)
			}
		}
		// log tasksWithDependencies json
		tasksWithDependenciesJSON, _ := json.Marshal(tasksWithDependencies)
		log.Printf("Tasks with dependencies: %s", string(tasksWithDependenciesJSON))

		// log tasksWithoutDependencies json
		tasksWithoutDependenciesJSON, _ := json.Marshal(tasksWithoutDependencies)
		log.Printf("Tasks without dependencies: %s", string(tasksWithoutDependenciesJSON))

		var snapshotStepResults []map[string]interface{}

		// Execute tasks with dependencies sequentially
		if len(tasksWithDependencies) > 0 {
			for _, step := range tasksWithDependencies {
				stepInt := step.Step

				// Filter previous results for dependencies
				var filteredPrevResults []map[string]interface{}
				if len(step.Dependencies) > 0 {
					for _, result := range snapshotStepResults {
						for _, dep := range step.Dependencies {
							if resultTaskID, ok := result["taskId"].(string); ok && dep == resultTaskID {
								filteredPrevResults = append(filteredPrevResults, result)
								break
							}
						}
					}
				}

				// Prepare task with previous results
				taskStr := step.Task
				if len(filteredPrevResults) > 0 {
					prevResultsJSON, _ := json.Marshal(filteredPrevResults)
					taskStr += fmt.Sprintf("\nPrevious results: %s", string(prevResultsJSON))
				}

				response, err := executeAIAgent(step.AgentAddress+"/messages", taskStr)
				if err != nil {
					log.Printf("Failed to process AI agent job: %v", err)
					continue
				}

				result := map[string]interface{}{
					"agentName": step.AgentName,
					"taskId":    step.TaskID,
					"content":   response,
				}

				snapshotStepResults = append(snapshotStepResults, result)
				log.Printf("Completed task with dependencies: %s (step %d)", step.TaskID, stepInt)
			}
		}

		// Execute tasks without dependencies in parallel
		if len(tasksWithoutDependencies) > 0 {
			// Use channels to collect results from parallel execution
			resultsChan := make(chan map[string]interface{}, len(tasksWithoutDependencies))
			errorsChan := make(chan error, len(tasksWithoutDependencies))

			// Execute tasks in parallel using goroutines
			for _, step := range tasksWithoutDependencies {
				go func(s shared.IAgentTask) {
					response, err := executeAIAgent(s.AgentAddress+"/messages", s.Task)
					if err != nil {
						log.Printf("Failed to process AI agent job: %v", err)
						errorsChan <- err
						return
					}

					result := map[string]interface{}{
						"agentName": s.AgentName,
						"taskId":    s.TaskID,
						"content":   response,
					}

					resultsChan <- result
				}(step)
			}

			// Collect results from parallel execution
			completedCount := 0
			for completedCount < len(tasksWithoutDependencies) {
				select {
				case result := <-resultsChan:
					snapshotStepResults = append(snapshotStepResults, result)
					if stepNum, ok := result["step"].(int); ok {
						log.Printf("Completed parallel task: %s (step %d)", result["taskId"], stepNum)
					}
					completedCount++
				case err := <-errorsChan:
					log.Printf("Error in parallel task execution: %v", err)
					completedCount++
				}
			}
		}

		// // Return final result
		if len(snapshotStepResults) > 0 {
			return snapshotStepResults, nil
		}

		return nil, fmt.Errorf("no final result found")
	}
	return "No Result Found", nil
}

// ===== AIAgentJob =====

func executeAIAgent(agentURL string, message string) (interface{}, error) {
	client := shared.NewAIAgentClient()

	taskID := uuid.New().String()
	completedTask, err := client.SendMessageAndWaitForCompletion(agentURL, taskID, message)
	if err != nil {
		return nil, err
	}
	finalResponse := shared.ExtractFinalResponse(completedTask)
	return finalResponse, nil
}

func processAIAgentJob(jobArgs shared.ProcessJobArgs, tasksService *TasksService) (interface{}, error) {
	payload := models.Payload{}
	if err := json.Unmarshal([]byte(jobArgs.Payload), &payload); err != nil {
		return nil, err
	}

	agentData := models.AIAgentData{}
	if err := json.Unmarshal([]byte(payload.ResourceData), &agentData); err != nil {
		return nil, err
	}

	err := tasksService.UpdateTaskById(jobArgs.TaskID, models.TaskStatusRunning)
	if err != nil {
		log.Printf("Failed to update task status to running: %v", err)
		return nil, err
	}

	response, err := executeAIAgent(agentData.URL+"/messages", payload.Prompt)
	if err != nil {
		return nil, err
	}

	result := shared.AIAgentResponse{
		AgentName: agentData.Name,
		TaskID:    uuid.New().String(),
		Content:   response.(string),
	}

	return result, nil
}
