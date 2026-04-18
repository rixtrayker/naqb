package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/amr/naqb/internal/db"
	"github.com/google/uuid"
)

// Queue wraps the SQLite DB with job-queue operations.
type Queue struct {
	db *sql.DB
}

// New creates a Queue backed by the given *sql.DB (must have migrations applied).
func New(sqlDB *sql.DB) *Queue {
	return &Queue{db: sqlDB}
}

// DB returns the underlying *sql.DB for callers that need direct access
// (e.g. passing to pipeline.StageInput for stage-progress tracking).
func (q *Queue) DB() *sql.DB { return q.db }

// Enqueue inserts a new job and returns its ID.
// payload should be one of the *Payload types in types.go (will be JSON-encoded).
// batch=true marks the job for batch pricing (50% cost reduction where supported).
func (q *Queue) Enqueue(_ context.Context, jt JobType, bookDir string, chapterNum int, payload any, batch bool, priority int) (string, error) {
	id := uuid.NewString()
	if err := db.EnqueueJob(q.db, id, string(jt), bookDir, chapterNum, payload, batch, priority); err != nil {
		return "", fmt.Errorf("queue: enqueue %s: %w", jt, err)
	}
	return id, nil
}

// Next claims the highest-priority pending job atomically.
// Returns (nil, nil) when the queue is empty.
func (q *Queue) Next(_ context.Context) (*db.Job, error) {
	return db.DequeueJob(q.db)
}

// Complete marks a job as done with a JSON-serialisable result.
func (q *Queue) Complete(_ context.Context, jobID string, result any) error {
	var resultJSON string
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("queue: marshal result: %w", err)
		}
		resultJSON = string(b)
	}
	return db.UpdateJobStatus(q.db, jobID, "done", resultJSON, "")
}

// Fail marks a job as failed with an error message.
// If the job has not exhausted its max_attempts, it will be re-queued on the
// next call to Requeue.
func (q *Queue) Fail(_ context.Context, jobID string, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return db.UpdateJobStatus(q.db, jobID, "failed", "", errMsg)
}

// Cancel sets a job to cancelled status.
func (q *Queue) Cancel(_ context.Context, jobID string) error {
	return db.CancelJob(q.db, jobID)
}

// Requeue re-sets failed jobs that have remaining attempts back to "pending".
// Returns the number of jobs re-queued.
func (q *Queue) Requeue(_ context.Context) (int, error) {
	res, err := q.db.Exec(
		`UPDATE jobs SET status = 'pending', started_at = NULL
		 WHERE status = 'failed' AND attempt < max_attempts`,
	)
	if err != nil {
		return 0, fmt.Errorf("queue: requeue: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Status returns a summary map of job counts per status.
func (q *Queue) Status(_ context.Context) (map[string]int, error) {
	rows, err := q.db.Query(
		`SELECT status, COUNT(*) FROM jobs GROUP BY status`,
	)
	if err != nil {
		return nil, fmt.Errorf("queue: status: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, rows.Err()
}
