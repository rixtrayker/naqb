-- +goose Up
CREATE TABLE jobs (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    book_dir     TEXT NOT NULL,
    chapter_num  INTEGER,
    payload      TEXT NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'pending'
                     CHECK(status IN ('pending','running','done','failed','cancelled')),
    priority     INTEGER DEFAULT 0,
    batch        BOOLEAN DEFAULT FALSE,
    result       TEXT,
    error        TEXT,
    attempt      INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at   DATETIME,
    finished_at  DATETIME
);

CREATE INDEX idx_jobs_status_priority ON jobs(status, priority DESC, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_status_priority;
DROP TABLE IF EXISTS jobs;
