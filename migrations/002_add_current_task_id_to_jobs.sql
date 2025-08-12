-- Migration 002: Add current_task_id column to jobs table for task recovery
-- This migration is handled automatically by GORM auto-migration
-- when the CurrentTaskID field is added to the Jobs struct

-- Manual SQL equivalent (for reference only):
-- ALTER TABLE jobs ADD COLUMN IF NOT EXISTS current_task_id UUID;
-- CREATE INDEX IF NOT EXISTS idx_jobs_current_task_id ON jobs (current_task_id) WHERE current_task_id IS NOT NULL;

-- Note: This file is for reference only. The actual migration is handled by GORM
-- when you run: go run cmd/migrate/main.go -action=setup