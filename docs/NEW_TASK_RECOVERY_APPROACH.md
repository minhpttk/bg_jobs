# New Task Recovery Approach

## Overview

We have changed the task recovery mechanism from using a `current_task_id` field in the jobs table to using River queue jobs with optional task IDs. This approach is more robust and follows better queue-based architecture patterns.

## Key Changes

### 1. Removed `CurrentTaskID` Field
- **Before**: Used `current_task_id` column in `jobs` table to track running tasks
- **After**: No longer track current task in jobs table

### 2. Modified `IntervalJobArgs`
```go
type IntervalJobArgs struct {
    JobID       uuid.UUID  `json:"job_id"`
    UserID      uuid.UUID  `json:"user_id"`
    WorkspaceID uuid.UUID  `json:"workspace_id"`
    Payload     string     `json:"payload"`
    TaskID      *uuid.UUID `json:"task_id,omitempty"` // ✅ NEW: Optional task ID for recovery
}
```

### 3. Enhanced Worker Logic
The worker now handles two scenarios:
- **New jobs**: Create new task as usual
- **Recovery jobs**: Use existing task ID and reset its status

```go
func (w *IntervalJobWorker) Work(ctx context.Context, job *river.Job[shared.IntervalJobArgs]) error {
    var taskID uuid.UUID
    if job.Args.TaskID != nil {
        // Recovery job: use existing task
        taskID = *job.Args.TaskID
        // Verify task is valid and reset status
    } else {
        // New job: create new task
        taskID, err = w.tasksService.CreateTask(job.Args.JobID, job.Args.Payload)
    }
    // ... rest of processing
}
```

### 4. New Recovery Process
When server restarts:
1. Find all tasks with status `running`
2. For each running task, create a recovery job in River queue with the task ID
3. Worker processes recovery jobs and re-executes the tasks

## Benefits

### ✅ **Simplified Schema**
- No need for `current_task_id` column
- Cleaner job table structure

### ✅ **Better Queue Integration**
- Recovery uses the same queue system as normal jobs
- Consistent with River architecture

### ✅ **More Robust**
- Handles edge cases better (task deletion, invalid states)
- Better error handling and logging

### ✅ **Scalable**
- Can handle multiple tasks per job
- Better for distributed systems

## Migration Steps

### 1. Deploy Code Changes
```bash
# Build and deploy new code
make build-worker
```

### 2. Run Migration
```bash
# Remove current_task_id column
make migrate-remove-current-task-id
```

### 3. Restart Worker
```bash
# Restart worker service
make run-worker
```

## Testing

### Test Recovery Scenario
1. Start a job that creates a task
2. Kill the worker process while task is running
3. Restart the worker
4. Verify that recovery job is added to queue
5. Verify that task is re-executed successfully

### Test Edge Cases
- Task deleted but job still references it
- Task with invalid status
- Multiple tasks for same job

## Monitoring

### Logs to Watch
- `"Recovery job: using existing task X for job Y"`
- `"Added recovery job for task X (job: Y)"`
- `"Task recovery completed. Added X recovery jobs to queue"`

### Metrics to Track
- Number of recovery jobs created
- Recovery job success/failure rate
- Task execution time after recovery

## Rollback Plan

If issues occur, you can rollback by:
1. Reverting code changes
2. Re-adding `current_task_id` column
3. Restoring old recovery logic

However, the new approach is more robust and should not require rollback.