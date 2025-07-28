# Background Job Service with River

A robust background job processing service built with Go, River, GORM, and PostgreSQL. This service supports AI Agent and Client Agent job processing with proper error handling, retries, and graceful shutdown.

## Features

- **River Queue System**: Reliable job processing with PostgreSQL
- **Multiple Job Types**: Support for AI Agent and Client Agent jobs
- **Database Integration**: GORM for database operations with job status tracking
- **Graceful Shutdown**: Proper handling of shutdown signals
- **Error Handling**: Comprehensive error handling with job status updates
- **Retry Logic**: Automatic job retries on failure
- **RESTful API**: HTTP endpoints for job creation

## Architecture

### Components

1. **API Server** (`cmd/api`): REST API for job creation and management
2. **Worker Process** (`cmd/worker`): Background job processor using River
3. **Job Handler** (`handlers/job_handler.go`): HTTP handlers for job operations
4. **Job Worker** (`jobs/worker.go`): River worker implementation
5. **River Client** (`jobs/river_client.go`): River client configuration
6. **Database Models** (`models/jobs.go`): Job data structures

### Job Flow

1. Client creates job via REST API
2. Job is stored in PostgreSQL with status "created"
3. River worker picks up job and updates status to "processing"
4. Worker processes job based on resource type (AI Agent or Client Agent)
5. Job status updated to "completed" or "failed"

## Setup

### Prerequisites

- Go 1.21+
- PostgreSQL 12+
- River CLI for migrations

### Installation

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Install River CLI:
   ```bash
   go install github.com/riverqueue/river/cmd/river@latest
   ```

4. Set up environment variables:
   ```bash
   export DATABASE_URL="postgres://username:password@localhost:5432/dbname?sslmode=disable"
   ```

5. Run database migrations:
   ```bash
   river migrate-up --database-url "$DATABASE_URL"
   ```

### Building

Build both API server and worker:

```bash
# Build API server
go build -o bin/api ./cmd/api

# Build worker
go build -o bin/worker ./cmd/worker
```

## Usage

### Starting the Services

1. **Start the API Server**:
   ```bash
   ./bin/api
   ```
   The API server will start on port 8082.

2. **Start the Worker Process**:
   ```bash
   ./bin/worker
   ```
   The worker will connect to the database and start processing jobs.

### Creating Jobs

Create a job via the REST API:

```bash
curl -X POST http://localhost:8082/api/jobs/create \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123e4567-e89b-12d3-a456-426614174000",
    "resource_type": "ai_agent",
    "resource_id": "456e7890-e89b-12d3-a456-426614174001",
    "payload": "{\"task\": \"process_data\", \"data\": \"example\"}",
    "priority": 1,
    "scheduled_at": "2024-01-01T12:00:00Z",
    "max_retries": 3
  }'
```

### Job Types

The system supports two resource types:

1. **AI Agent** (`ai_agent`): For AI-related processing tasks
2. **Client Agent** (`client_agent`): For client-specific operations

### Job Status Flow

- `created` → `processing` → `completed` (success)
- `created` → `processing` → `failed` (error)

## Configuration

### Database Configuration

The database configuration is handled in `config/database.go`:

- **GORM**: For ORM operations and job status tracking
- **River**: For job queue management with pgx driver
- **Connection Pooling**: Optimized connection management

### River Configuration

River is configured with:
- **Default Queue**: 100 max workers
- **Job Timeout**: Configurable per job type
- **Retry Policy**: Automatic retries with exponential backoff
- **Graceful Shutdown**: 30-second timeout for graceful shutdown

### Worker Configuration

The worker supports:
- **AI Agent Jobs**: 2-second simulated processing time
- **Client Agent Jobs**: 1-second simulated processing time
- **Payload Parsing**: JSON payload validation and processing
- **Status Updates**: Real-time job status tracking

## API Endpoints

### Health Check
```
GET /api/health
```

### Create Job
```
POST /api/jobs/create
```

**Request Body:**
```json
{
  "user_id": "uuid",
  "resource_type": "ai_agent|client_agent",
  "resource_id": "uuid",
  "payload": "json_string",
  "priority": 1,
  "scheduled_at": "2024-01-01T12:00:00Z",
  "max_retries": 3
}
```

**Response:**
```json
{
  "job_id": "uuid"
}
```

## Development

### Project Structure

```
background-job-service/
├── cmd/
│   ├── api/main.go          # API server entry point
│   └── worker/main.go       # Worker process entry point
├── config/
│   └── database.go          # Database configuration
├── handlers/
│   └── job_handler.go       # HTTP handlers
├── jobs/
│   ├── river_client.go      # River client setup
│   └── worker.go            # Job worker implementation
├── models/
│   └── jobs.go              # Data models
└── scripts/
    └── migrate-up.sh        # Migration script
```

### Adding New Job Types

1. Add new resource type to `models/jobs.go`:
   ```go
   const (
       AIAgent     ResourceType = "ai_agent"
       ClientAgent ResourceType = "client_agent"
       NewType     ResourceType = "new_type"  // Add here
   )
   ```

2. Add processing logic to `jobs/worker.go`:
   ```go
   case string(models.NewType):
       err = w.processNewTypeJob(ctx, job.Args)
   ```

3. Implement the processing function:
   ```go
   func (w *JobWorker) processNewTypeJob(ctx context.Context, args handlers.JobArgs) error {
       // Implementation here
       return nil
   }
   ```

### Testing

Run tests:
```bash
go test ./...
```

### Monitoring

The worker logs provide detailed information about:
- Job processing start/completion
- Error handling and retries
- Graceful shutdown process
- Database connection status

## Deployment

### Environment Variables

Required environment variables:
- `DATABASE_URL`: PostgreSQL connection string

### Docker Deployment

Example Dockerfile for the worker:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/worker .
CMD ["./worker"]
```

### Production Considerations

1. **Database Connection Pooling**: Configure appropriate pool sizes
2. **Worker Scaling**: Run multiple worker instances for high throughput
3. **Monitoring**: Set up logging and metrics collection
4. **Error Alerting**: Monitor failed jobs and system health
5. **Resource Limits**: Configure appropriate memory and CPU limits

## Troubleshooting

### Common Issues

1. **Database Connection Errors**: Check `DATABASE_URL` and PostgreSQL availability
2. **Migration Issues**: Ensure River migrations are up to date
3. **Worker Not Processing Jobs**: Verify worker is connected to correct database
4. **Job Failures**: Check logs for detailed error messages

### Logs

Worker logs include:
- Job processing status
- Error details with stack traces
- Database operation results
- Shutdown process information

## License

This project is licensed under the MIT License. 