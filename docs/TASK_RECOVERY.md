# Task Recovery Mechanism

## Tổng quan

Task Recovery Mechanism là một tính năng mới được thêm vào hệ thống bg_jobs để giải quyết vấn đề mất task khi server restart giữa chừng một job interval.

## Vấn đề được giải quyết

**Trước đây:**
- Khi server restart giữa chừng job interval (ví dụ: đang thực hiện task thứ 2/5)
- Các task còn lại sẽ bị miss
- Job sẽ chạy lại từ đầu thay vì tiếp tục từ task đã dừng

**Bây giờ:**
- Progress của từng interval được track chi tiết
- Khi server restart, incomplete intervals được detect và recover
- Tasks resume từ điểm dừng, không chạy lại từ đầu

## Tính năng Backward Compatibility

### ✅ Không ảnh hưởng logic hiện tại

1. **Logic cũ vẫn hoạt động**: Tất cả logic hiện tại được giữ nguyên trong method `executeOriginal()`
2. **Recovery là optional**: Có thể enable/disable cho từng job hoặc globally
3. **Fallback mechanism**: Nếu recovery fail, tự động fallback về logic cũ
4. **Default behavior**: Mặc định recovery được enable nhưng có thể disable

### Cấu trúc code mới:

```go
func (w *IntervalJobWorker) Work(ctx context.Context, job *river.Job[shared.IntervalJobArgs]) error {
    // Check if recovery is enabled for this job
    if w.shouldUseRecovery(job.Args.JobID) {
        return w.executeWithRecovery(ctx, job.Args, payload)
    }
    
    // Original logic (backward compatible)
    return w.executeOriginal(ctx, job.Args, payload)
}
```

## Cấu hình

### Environment Variables

```bash
# Enable/disable recovery globally
ENABLE_RECOVERY=true

# Default value for new jobs
DEFAULT_RECOVERY_ENABLED=true

# How often to check for incomplete intervals (seconds)
RECOVERY_CHECK_INTERVAL=300

# Maximum recovery attempts per job
MAX_RECOVERY_ATTEMPTS=3
```

### Per-Job Configuration

Khi tạo job mới, có thể specify recovery:

```json
{
  "name": "My Job",
  "workspace_id": "123e4567-e89b-12d3-a456-426614174000",
  "payload": "...",
  "type": "interval",
  "interval": "...",
  "enable_recovery": true  // Optional: enable/disable for this job
}
```

## Database Changes

### New Columns

```sql
ALTER TABLE jobs 
ADD COLUMN current_interval_id VARCHAR(255),
ADD COLUMN interval_progress TEXT,
ADD COLUMN interval_started_at TIMESTAMP,
ADD COLUMN enable_recovery BOOLEAN NOT NULL DEFAULT TRUE;
```

### Index

```sql
CREATE INDEX idx_jobs_incomplete_intervals 
ON jobs (type, status, is_deleted, current_interval_id) 
WHERE type = 'interval' AND status = 'active' AND is_deleted = false AND current_interval_id IS NOT NULL;
```

## Cách hoạt động

### 1. Normal Execution (Recovery Enabled)

```go
// Job interval starts
progress := StartNewInterval(jobID, totalTasks)

// Track each task
taskResult := ExecuteTask(taskID)
UpdateProgress(progress, taskResult)

// If server restarts mid-execution
// On startup, detect incomplete intervals
incompleteJobs := GetIncompleteIntervals()

// Schedule recovery for each job
ScheduleTaskRecovery(jobID, intervalID)

// Recovery worker processes
// - Skip completed tasks
// - Resume incomplete tasks
// - Mark interval completed when done
```

### 2. Fallback Execution (Recovery Disabled/Failed)

```go
// Use original logic
taskID := CreateTask(jobID, payload)
result := ProcessJob(taskID)
UpdateTaskResult(taskID, result)
```

## Migration Guide

### 1. Deploy với Recovery Enabled (Recommended)

```bash
# Run migration
go run migrations/migrate.go

# Start services
go run cmd/server/main.go
go run cmd/worker/main.go
```

### 2. Disable Recovery cho Existing Jobs

```sql
-- Disable recovery for all existing jobs
UPDATE jobs SET enable_recovery = false WHERE type = 'interval';
```

### 3. Disable Recovery Globally

```bash
# Set environment variable
export ENABLE_RECOVERY=false

# Restart worker
go run cmd/worker/main.go
```

### 4. Rollback hoàn toàn (nếu cần)

```sql
-- Remove recovery fields entirely
ALTER TABLE jobs DROP COLUMN IF EXISTS current_interval_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS interval_progress;
ALTER TABLE jobs DROP COLUMN IF EXISTS interval_started_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS enable_recovery;
DROP INDEX IF EXISTS idx_jobs_incomplete_intervals;
```

## Monitoring và Debugging

### Logs

Recovery mechanism cung cấp detailed logging:

```
2024/01/01 10:00:00 Starting recovery check on startup...
2024/01/01 10:00:01 Found 2 jobs with incomplete intervals
2024/01/01 10:00:01 Scheduled recovery for job abc-123, interval def-456
2024/01/01 10:00:02 Processing recovery task: task-789
2024/01/01 10:00:03 All tasks completed for interval def-456
```

### Health Check

```bash
# Check recovery status
curl http://localhost:8080/health

# Check job progress
curl http://localhost:8080/jobs/{job_id}
```

## Testing

### Test Script

```bash
# Run test script
./scripts/test_recovery.sh
```

### Manual Testing

1. **Create interval job**
2. **Stop worker mid-execution**
3. **Restart worker**
4. **Verify recovery**

## Troubleshooting

### Common Issues

1. **Recovery not working**
   - Check `ENABLE_RECOVERY` environment variable
   - Check `enable_recovery` field in job record
   - Check logs for error messages

2. **Performance impact**
   - Recovery queries are indexed
   - Progress data stored in database, not memory
   - Minimal overhead for normal execution

3. **Database errors**
   - Ensure migration is run
   - Check database permissions
   - Verify connection string

### Debug Commands

```bash
# Check migration status
go run migrations/migrate.go

# Check recovery configuration
echo $ENABLE_RECOVERY
echo $DEFAULT_RECOVERY_ENABLED

# Check incomplete intervals
psql -d bg_jobs -c "SELECT id, name, current_interval_id FROM jobs WHERE current_interval_id IS NOT NULL;"
```

## Performance Considerations

### Database Impact

- **Minimal overhead**: Progress tracking chỉ thêm 1 query per task
- **Indexed queries**: Recovery queries được optimize với index
- **JSON storage**: Progress data được compress trong JSON field

### Memory Impact

- **No memory storage**: Progress data stored in database
- **Configurable**: Có thể limit recovery attempts
- **Efficient**: Chỉ load progress khi cần

## Future Enhancements

1. **Configurable task granularity**: Cho phép define multiple tasks per interval
2. **Recovery metrics**: Track recovery success/failure rates
3. **Advanced scheduling**: Support for complex task dependencies
4. **Web UI**: Visual progress tracking interface

## Support

Nếu gặp vấn đề với recovery mechanism:

1. Check logs first
2. Verify configuration
3. Test với simple job
4. Disable recovery nếu cần thiết
5. Contact development team