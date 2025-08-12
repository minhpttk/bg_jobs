-- Add current_task_id column to jobs table for task recovery
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS current_task_id UUID;

-- Add index for better performance when querying by current_task_id
CREATE INDEX IF NOT EXISTS idx_jobs_current_task_id ON jobs (current_task_id) WHERE current_task_id IS NOT NULL;