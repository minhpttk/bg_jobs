-- Migration: Remove current_task_id column from jobs table
-- This migration removes the current_task_id column as we're changing the recovery approach
-- to use riverqueue jobs with task IDs instead of tracking current task in the jobs table

-- Remove the current_task_id column
ALTER TABLE jobs DROP COLUMN IF EXISTS current_task_id;

-- Remove the index if it exists (GORM might have created one)
DROP INDEX IF EXISTS idx_jobs_current_task_id;