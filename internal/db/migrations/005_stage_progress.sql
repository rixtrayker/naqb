-- +goose Up
CREATE TABLE stage_progress (
    job_id       TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    stage_name   TEXT NOT NULL,
    completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (job_id, stage_name)
);
CREATE INDEX idx_stage_progress_job ON stage_progress(job_id);

-- +goose Down
DROP INDEX IF EXISTS idx_stage_progress_job;
DROP TABLE IF EXISTS stage_progress;
