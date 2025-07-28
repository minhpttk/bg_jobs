# Database Migrations

This directory contains database migrations for the Background Job Service.

## Files Overview

- `001_create_jobs_and_tasks_tables.sql` - Initial migration to create jobs and tasks tables
- `001_create_jobs_and_tasks_tables_down.sql` - Rollback migration to drop tables
- `migrate.go` - GORM-based migration functions
- `README.md` - This documentation file

## Migration Methods

You can run migrations using two different approaches:

### 1. SQL-based Migrations (Recommended for Production)

Use the shell script to run raw SQL migrations:

```bash
# Apply migration (create tables)
./scripts/run-migration.sh up

# Rollback migration (drop tables)
./scripts/run-migration.sh down
```

### 2. GORM-based Migrations (Good for Development)

Use the Go command to run GORM auto-migrations:

```bash
# Build and run migration command
go build -o bin/migrate ./cmd/migrate
./bin/migrate -action=setup

# Or run directly
go run ./cmd/migrate -action=setup
```

## Environment Variables

Set your database connection string:

```bash
export DATABASE_URL="postgres://username:password@localhost:5432/dbname?sslmode=disable"
```

Default: `postgres://postgres:postgres@localhost:5432/bg_jobs`

## Table Structure

### Jobs Table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| user_id | UUID | User who created the job |
| status | VARCHAR(20) | Job status (created, processing, completed, failed, cancelled) |
| payload | JSONB | Job payload with prompt, resource_type, resource_id |
| job_type | VARCHAR(100) | Type of job |
| schedule_interval | JSONB | Schedule configuration (cron, timedelta, manual) |
| max_retries | INTEGER | Maximum retry attempts |
| priority | INTEGER | Job priority (0-100) |
| is_deleted | BOOLEAN | Soft delete flag |
| created_at | TIMESTAMPTZ | Creation timestamp |
| updated_at | TIMESTAMPTZ | Last update timestamp |
| version | BIGINT | Version for optimistic locking |

### Tasks Table

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| job_id | UUID | Foreign key to jobs table |
| status | VARCHAR(20) | Task status |
| payload | JSONB | Task-specific payload |
| result | TEXT | Task execution result |
| is_deleted | BOOLEAN | Soft delete flag |
| created_at | TIMESTAMPTZ | Creation timestamp |
| updated_at | TIMESTAMPTZ | Last update timestamp |
| version | BIGINT | Version for optimistic locking |

## Indexes Created

### Jobs Table Indexes
- `idx_jobs_user_id` - User ID lookup
- `idx_jobs_status` - Status filtering
- `idx_jobs_created_at` - Time-based queries
- `idx_jobs_priority` - Priority-based ordering
- `idx_jobs_is_deleted` - Soft delete filtering
- `idx_jobs_user_status` - Composite index for user + status queries
- `idx_jobs_status_priority` - Composite index for status + priority queries
- `idx_jobs_created_status` - Composite index for time + status queries

### Tasks Table Indexes
- `idx_tasks_job_id` - Job relationship lookup
- `idx_tasks_status` - Status filtering
- `idx_tasks_created_at` - Time-based queries
- `idx_tasks_is_deleted` - Soft delete filtering
- `idx_tasks_job_status` - Composite index for job + status queries

## Features

- **UUID Primary Keys**: Using UUID v4 for better distribution
- **JSONB Storage**: Flexible payload storage with indexing support
- **Automatic Timestamps**: Triggers to update `updated_at` automatically
- **Soft Deletes**: `is_deleted` flag instead of hard deletes
- **Optimistic Locking**: Version field for concurrent update safety
- **Check Constraints**: Status field validation at database level
- **Foreign Key Constraints**: Referential integrity between jobs and tasks
- **Comprehensive Indexing**: Optimized for common query patterns

## Usage Examples

### Running Initial Setup

```bash
# Method 1: SQL-based
./scripts/run-migration.sh up

# Method 2: GORM-based
go run ./cmd/migrate -action=setup
```

### Rolling Back (SQL only)

```bash
./scripts/run-migration.sh down
```

## Notes

- The GORM migration method doesn't support rollbacks - use SQL method for production
- River queue tables are handled separately by River CLI (`river migrate-up`)
- Always backup your database before running migrations in production
- The migration scripts are idempotent and safe to run multiple times 