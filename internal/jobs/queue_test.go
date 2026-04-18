package jobs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"github.com/amr/naqb/internal/db"
)

// openTestDB opens a temp SQLite DB with migrations applied.
func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	return sqlDB, func() { sqlDB.Close() }
}

// suppress goose output in tests
func init() {
	goose.SetLogger(goose.NopLogger())
}

// ── Enqueue / Next ────────────────────────────────────────────────────────────

func TestQueue_Enqueue(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, JobWrite, "/books/x", 1, WritePayload{ChapterNum: 1}, false, 0)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id == "" {
		t.Error("Enqueue returned empty ID")
	}
	// Verify UUID format
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("ID %q is not a valid UUID: %v", id, err)
	}
}

func TestQueue_Next_EmptyQueue(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)

	job, err := q.Next(context.Background())
	if err != nil {
		t.Fatalf("Next on empty queue: %v", err)
	}
	if job != nil {
		t.Errorf("expected nil job from empty queue, got %+v", job)
	}
}

func TestQueue_Next_ClaimsJob(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, JobQA, "/books/x", 2, QAPayload{ChapterNum: 2}, false, 0)

	job, err := q.Next(ctx)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job, got nil")
	}
	if job.ID != id {
		t.Errorf("job ID: got %q, want %q", job.ID, id)
	}
	if job.Status != "running" {
		t.Errorf("status after Next: got %q, want running", job.Status)
	}
}

func TestQueue_Next_PriorityOrder(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	_, _ = q.Enqueue(ctx, JobWrite, "/books/x", 1, WritePayload{}, false, 0)
	highID, _ := q.Enqueue(ctx, JobWrite, "/books/x", 2, WritePayload{}, false, 99)

	job, _ := q.Next(ctx)
	if job.ID != highID {
		t.Errorf("expected high-priority job first, got %q", job.ID)
	}
}

// ── Complete / Fail / Cancel ──────────────────────────────────────────────────

func TestQueue_Complete(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, JobPipeline, "/books/x", 3, PipelinePayload{ChapterNum: 3}, false, 0)
	job, _ := q.Next(ctx)
	if job == nil {
		t.Fatal("expected a job")
	}

	result := map[string]any{"tokens": 1234}
	if err := q.Complete(ctx, id, result); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// After completion, Next should return nil (no more pending jobs)
	next, _ := q.Next(ctx)
	if next != nil {
		t.Errorf("expected empty queue after completion, got %+v", next)
	}
}

func TestQueue_Fail(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, JobWrite, "/books/x", 1, WritePayload{}, false, 0)
	_, _ = q.Next(ctx) // claim it

	if err := q.Fail(ctx, id, errors.New("LLM timeout")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
}

func TestQueue_Cancel(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, JobResearch, "/books/x", 1, ResearchPayload{}, false, 0)
	if err := q.Cancel(ctx, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Cancelled job should not be returned by Next
	next, _ := q.Next(ctx)
	if next != nil {
		t.Error("cancelled job should not be returned by Next")
	}
}

// ── Requeue ───────────────────────────────────────────────────────────────────

func TestQueue_Requeue(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, JobWrite, "/books/x", 1, WritePayload{}, false, 0)
	_, _ = q.Next(ctx) // claim → running
	_ = q.Fail(ctx, id, errors.New("transient error"))

	// Requeue failed jobs that have remaining attempts
	n, err := q.Requeue(ctx)
	if err != nil {
		t.Fatalf("Requeue: %v", err)
	}
	if n != 1 {
		t.Errorf("Requeue: got %d, want 1", n)
	}

	// Now it should be claimable again
	job, _ := q.Next(ctx)
	if job == nil {
		t.Error("expected requeued job to be claimable")
	}
	if job.ID != id {
		t.Errorf("requeued job ID: got %q, want %q", job.ID, id)
	}
}

// ── Status ────────────────────────────────────────────────────────────────────

func TestQueue_Status(t *testing.T) {
	sqlDB, cleanup := openTestDB(t)
	defer cleanup()
	q := New(sqlDB)
	ctx := context.Background()

	_, _ = q.Enqueue(ctx, JobWrite, "/books/x", 1, WritePayload{}, false, 0)
	_, _ = q.Enqueue(ctx, JobQA, "/books/x", 2, QAPayload{}, false, 0)

	status, err := q.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status["pending"] != 2 {
		t.Errorf("pending count: got %d, want 2", status["pending"])
	}
}
