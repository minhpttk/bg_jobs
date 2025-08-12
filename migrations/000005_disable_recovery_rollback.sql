-- Migration to disable recovery mechanism for existing jobs
-- This can be used if you want to rollback to the original behavior

-- Option 1: Disable recovery for all existing jobs
UPDATE jobs SET enable_recovery = false WHERE type = 'interval';

-- Option 2: Remove recovery fields entirely (uncomment if needed)
-- ALTER TABLE jobs DROP COLUMN IF EXISTS current_interval_id;
-- ALTER TABLE jobs DROP COLUMN IF EXISTS interval_progress;
-- ALTER TABLE jobs DROP COLUMN IF EXISTS interval_started_at;
-- ALTER TABLE jobs DROP COLUMN IF EXISTS enable_recovery;
-- DROP INDEX IF EXISTS idx_jobs_incomplete_intervals;