CREATE INDEX idx_jobs_user_workspace ON jobs (
    user_id,
    workspace_id,
    is_deleted
);

CREATE INDEX idx_jobs_status_type ON jobs(status, type, is_deleted);

CREATE INDEX idx_tasks_job_id ON tasks (job_id, is_deleted);