-- Migration 003: Optimize indexes for performance and fix N+1 query issues
-- This migration addresses performance issues with large datasets (1M+ records)

-- Drop existing inefficient indexes
DROP INDEX IF EXISTS idx_jobs_user_id_workspace_id;
DROP INDEX IF EXISTS idx_jobs_status_type;
DROP INDEX IF EXISTS idx_jobs_river_job_id;
DROP INDEX IF EXISTS idx_tasks_job_id;
DROP INDEX IF EXISTS idx_tasks_river_job_id;

-- Create optimized composite indexes for jobs table
-- Covering index for user/workspace queries with status filtering
CREATE INDEX idx_jobs_user_workspace_status ON jobs (
    user_id,
    workspace_id,
    status,
    is_deleted
) WHERE is_deleted = false;

-- Covering index for status/type queries with pagination support
CREATE INDEX idx_jobs_status_type_pagination ON jobs (
    status,
    type,
    created_at DESC,
    is_deleted
) WHERE is_deleted = false;

-- Index for river job lookups
CREATE INDEX idx_jobs_river_job_id ON jobs (river_job_id) WHERE is_deleted = false;

-- Index for current task tracking
CREATE INDEX idx_jobs_current_task_id ON jobs (current_task_id) WHERE current_task_id IS NOT NULL;

-- Optimized indexes for tasks table
-- Covering index for job_id queries with status filtering and pagination
CREATE INDEX idx_tasks_job_id_status_pagination ON tasks (
    job_id,
    status,
    created_at DESC,
    is_deleted
) WHERE is_deleted = false;

-- Index for recovery queries (running tasks)
CREATE INDEX idx_tasks_status_recovery ON tasks (
    status,
    is_deleted,
    created_at
) WHERE status = 'running' AND is_deleted = false;

-- Index for incomplete tasks queries
CREATE INDEX idx_tasks_job_incomplete ON tasks (
    job_id,
    status,
    created_at ASC,
    is_deleted
) WHERE status IN ('created', 'running') AND is_deleted = false;

-- Index for task updates by ID
CREATE INDEX idx_tasks_id_update ON tasks (id, is_deleted) WHERE is_deleted = false;

-- Index for river job lookups in tasks
CREATE INDEX idx_tasks_river_job_id ON tasks (river_job_id) WHERE is_deleted = false;