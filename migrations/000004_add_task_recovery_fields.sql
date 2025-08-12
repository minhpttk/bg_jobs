-- Add task recovery fields to jobs table
ALTER TABLE jobs 
ADD COLUMN current_interval_id VARCHAR(255),
ADD COLUMN interval_progress TEXT,
ADD COLUMN interval_started_at TIMESTAMP,
ADD COLUMN enable_recovery BOOLEAN NOT NULL DEFAULT TRUE;

-- Add index for faster recovery queries
CREATE INDEX idx_jobs_incomplete_intervals 
ON jobs (type, status, is_deleted, current_interval_id) 
WHERE type = 'interval' AND status = 'active' AND is_deleted = false AND current_interval_id IS NOT NULL;