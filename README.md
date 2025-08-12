# Background Job Service with River

A robust background job processing service built with Go, River, GORM, and PostgreSQL. This service supports AI Agent and Client Agent job processing with proper error handling, retries, graceful shutdown, and **task recovery mechanism**.

## Key Features

- **River Queue System**: Reliable job processing with PostgreSQL
- **Job Scheduling**: Support for scheduled and interval-based jobs
- **Task Recovery**: Automatic recovery of incomplete tasks when server restarts
- **Progress Tracking**: Detailed tracking of job interval progress
- **Graceful Shutdown**: Proper cleanup and recovery on shutdown
- **Error Handling**: Comprehensive error handling and retry mechanisms

## Architecture

### Components

1. **API Server** (`cmd/server`): REST API for job management
2. **Worker Process** (`cmd/worker`): Background job processor using River
3. **Job Service** (`services/job_service.go`): Business logic for job management
4. **Job Worker** (`services/job_workers.go`): River worker implementation
5. **Task Recovery Worker** (`services/task_recovery_worker.go`): Recovery mechanism for incomplete tasks
6. **River Client** (`services/river.go`): River client configuration

## Task Recovery Mechanism

### Problem Solved
When a server restarts during a job interval execution (e.g., when 5 tasks are planned but only 2 completed), the remaining tasks would be lost and the job would restart from the beginning.

### Solution
The system now implements a comprehensive task recovery mechanism:

1. **Progress Tracking**: Each job interval tracks its execution progress
2. **Task State Management**: Individual task states are stored and tracked
3. **Automatic Recovery**: On server startup, incomplete intervals are automatically recovered
4. **Resume Capability**: Tasks resume from where they left off, not from the beginning

### How It Works

1. **Interval Execution**: When a job interval starts, it creates a progress record
2. **Task Tracking**: Each task execution is tracked with status, result, and timing
3. **Recovery Detection**: On startup, the system identifies incomplete intervals
4. **Task Recovery**: Incomplete tasks are re-executed while completed tasks are skipped
5. **Progress Completion**: Once all tasks are done, the interval is marked as completed

### Database Schema

New fields added to the `jobs` table:
- `current_interval_id`: Tracks the current interval being executed
- `interval_progress`: JSON string storing detailed progress information
- `interval_started_at`: Timestamp when the current interval started

### Recovery Process

```go
// On server startup
1. Scan for jobs with incomplete intervals
2. For each incomplete interval:
   - Schedule a recovery job
   - Recovery worker processes incomplete tasks
   - Skip already completed tasks
   - Resume from the last incomplete task
3. Mark interval as completed when all tasks are done
```

## Job Processing Flow

1. **Job Creation**: Job is created via API with schedule/interval configuration
2. **River Scheduling**: Job is scheduled in River queue system
3. **Worker Pickup**: River worker picks up job and updates status to "processing"
4. **Interval Execution**: For interval jobs, progress tracking begins
5. **Task Execution**: Individual tasks are executed and tracked
6. **Recovery Check**: If server restarts, incomplete tasks are recovered
7. **Completion**: Job is marked as completed and rescheduled for next run

## Setup and Installation

### Prerequisites

- Go 1.21+
- PostgreSQL 12+
- River CLI

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd gin-gorm-river-app
```

2. Install dependencies:
```bash
go mod download
```

3. Install River CLI:
```bash
go install github.com/riverqueue/river/cmd/river@latest
```

4. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your database configuration
```

5. Run database migrations:
```bash
# Run GORM migrations
go run migrations/migrate.go

# Run River migrations
river migrate-up --database-url "$DATABASE_URL"
```

### Running the Application

1. **Start the API server**:
```bash
go run cmd/server/main.go
```

2. **Start the worker**:
```bash
go run cmd/worker/main.go
```

The worker will automatically:
- Start processing background jobs
- Run recovery check for incomplete intervals
- Resume any interrupted task executions

## API Endpoints

### Job Management

- `POST /jobs` - Create a new job
- `GET /jobs` - List all jobs
- `GET /jobs/:id` - Get job details
- `PUT /jobs/:id` - Update job
- `DELETE /jobs/:id` - Delete job

### Job Types

1. **Scheduled Jobs**: One-time execution at a specific time
2. **Interval Jobs**: Recurring execution based on cron schedule

## Configuration

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string
- `MAX_WORKERS`: Maximum number of concurrent workers (default: 10)
- `PORT`: API server port (default: 8080)

### River Configuration

River is configured with:
- PostgreSQL as the backend
- Automatic job recovery
- Configurable worker pools
- Graceful shutdown handling

## Monitoring and Debugging

### Logs

The system provides detailed logging for:
- Job execution progress
- Task recovery operations
- Error handling and retries
- Recovery check results

### River UI

Access River's web UI for job monitoring:
```bash
# Start River UI (if configured)
riverui --database-url "$DATABASE_URL"
```

## Error Handling

- **Job Failures**: Failed jobs are logged and can be retried
- **Task Failures**: Individual task failures don't stop the entire interval
- **Recovery Failures**: Recovery errors are logged but don't prevent other recoveries
- **Database Errors**: Connection issues are handled gracefully

## Performance Considerations

- **Indexed Queries**: Recovery queries are optimized with database indexes
- **Batch Processing**: Multiple tasks can be processed efficiently
- **Memory Management**: Progress data is stored in database, not memory
- **Concurrent Processing**: Multiple workers can process different jobs simultaneously

## Troubleshooting

### Common Issues

1. **Migration Issues**: Ensure River migrations are up to date
2. **Recovery Failures**: Check logs for specific error messages
3. **Database Connection**: Verify database connectivity and permissions
4. **Worker Startup**: Ensure proper environment variable configuration

### Debug Commands

```bash
# Check River migration status
river migrate-status --database-url "$DATABASE_URL"

# View job queue
river list-jobs --database-url "$DATABASE_URL"

# Check incomplete intervals
# (Use the API endpoint or database query)
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under the MIT License. 