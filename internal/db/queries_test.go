package db

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
)

// ── Sessions ──────────────────────────────────────────────────────────────────

func TestCreateAndGetSession(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	if err := CreateSession(db, id, "/books/mybook", 3); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sess, err := GetSession(db, id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.ID != id {
		t.Errorf("ID: got %q, want %q", sess.ID, id)
	}
	if sess.BookDir != "/books/mybook" {
		t.Errorf("BookDir: got %q, want /books/mybook", sess.BookDir)
	}
	if sess.ChapterNum != 3 {
		t.Errorf("ChapterNum: got %d, want 3", sess.ChapterNum)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	_, err := GetSession(db, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent session ID")
	}
}

func TestListSessions(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	bookDir := "/books/testbook"
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for i, id := range ids {
		if err := CreateSession(db, id, bookDir, i+1); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
	}

	sessions, err := ListSessions(db, bookDir, 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("ListSessions: got %d, want 3", len(sessions))
	}
}

func TestListSessions_OtherBookIsolated(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id1 := uuid.NewString()
	id2 := uuid.NewString()
	_ = CreateSession(db, id1, "/books/book-a", 1)
	_ = CreateSession(db, id2, "/books/book-b", 1)

	sessions, err := ListSessions(db, "/books/book-a", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != id1 {
		t.Errorf("expected only book-a session, got %+v", sessions)
	}
}

func TestDeleteSession(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = CreateSession(db, id, "/books/x", 0)
	// Append a message so we can verify cascade delete
	msgID := uuid.NewString()
	_ = AppendMessage(db, msgID, id, "user", "hello", "model", 10, 5)

	if err := DeleteSession(db, id); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Session should be gone
	if _, err := GetSession(db, id); err == nil {
		t.Error("expected error after deleting session")
	}

	// Message should cascade-delete
	msgs, err := GetSessionMessages(db, id)
	if err != nil {
		t.Fatalf("GetSessionMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cascade delete, got %d", len(msgs))
	}
}

func TestTouchSession(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = CreateSession(db, id, "/books/x", 0)

	// Just verify no error
	if err := TouchSession(db, id); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}
}

// ── Messages ──────────────────────────────────────────────────────────────────

func TestAppendAndGetMessages(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	sessID := uuid.NewString()
	_ = CreateSession(db, sessID, "/books/x", 1)

	msgs := []struct {
		role    string
		content string
	}{
		{"user", "Hello agent"},
		{"assistant", "Hello! I can help you write."},
		{"tool", `{"tool":"read_file","result":"..."}`},
	}
	for _, m := range msgs {
		msgID := uuid.NewString()
		if err := AppendMessage(db, msgID, sessID, m.role, m.content, "model-x", 10, 20); err != nil {
			t.Fatalf("AppendMessage role=%s: %v", m.role, err)
		}
	}

	got, err := GetSessionMessages(db, sessID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(got) != len(msgs) {
		t.Fatalf("message count: got %d, want %d", len(got), len(msgs))
	}
	for i, m := range got {
		if m.Role != msgs[i].role {
			t.Errorf("msg[%d].Role = %q, want %q", i, m.Role, msgs[i].role)
		}
		if m.Content != msgs[i].content {
			t.Errorf("msg[%d].Content mismatch", i)
		}
		if m.TokensIn != 10 || m.TokensOut != 20 {
			t.Errorf("msg[%d] tokens: got %d/%d, want 10/20", i, m.TokensIn, m.TokensOut)
		}
	}
}

func TestGetSessionMessages_Empty(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	sessID := uuid.NewString()
	_ = CreateSession(db, sessID, "/books/x", 0)

	msgs, err := GetSessionMessages(db, sessID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

func TestEnqueueAndGetJob(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	payload := map[string]any{"chapter_num": 3}
	if err := EnqueueJob(db, id, "write", "/books/x", 3, payload, false, 0); err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	job, err := GetJob(db, id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.ID != id {
		t.Errorf("ID: got %q, want %q", job.ID, id)
	}
	if job.Type != "write" {
		t.Errorf("Type: got %q, want write", job.Type)
	}
	if job.Status != "pending" {
		t.Errorf("Status: got %q, want pending", job.Status)
	}
	if job.Batch {
		t.Error("Batch should be false")
	}
}

func TestEnqueueJob_BatchFlag(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = EnqueueJob(db, id, "pipeline", "/books/x", 1, map[string]any{}, true, 5)

	job, err := GetJob(db, id)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if !job.Batch {
		t.Error("Batch should be true")
	}
	if job.Priority != 5 {
		t.Errorf("Priority: got %d, want 5", job.Priority)
	}
}

func TestDequeueJob_ClaimsPendingJob(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = EnqueueJob(db, id, "write", "/books/x", 1, map[string]any{}, false, 0)

	job, err := DequeueJob(db)
	if err != nil {
		t.Fatalf("DequeueJob: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job, got nil")
	}
	if job.ID != id {
		t.Errorf("ID: got %q, want %q", job.ID, id)
	}
	if job.Status != "running" {
		t.Errorf("Status after dequeue: got %q, want running", job.Status)
	}
	if job.Attempt != 1 {
		t.Errorf("Attempt: got %d, want 1", job.Attempt)
	}
}

func TestDequeueJob_EmptyQueue(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	job, err := DequeueJob(db)
	if err != nil {
		t.Fatalf("DequeueJob on empty queue: %v", err)
	}
	if job != nil {
		t.Errorf("expected nil job from empty queue, got %+v", job)
	}
}

func TestDequeueJob_PriorityOrder(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	low := uuid.NewString()
	high := uuid.NewString()
	_ = EnqueueJob(db, low, "write", "/books/x", 1, map[string]any{}, false, 0)
	_ = EnqueueJob(db, high, "write", "/books/x", 2, map[string]any{}, false, 10)

	job, err := DequeueJob(db)
	if err != nil {
		t.Fatalf("DequeueJob: %v", err)
	}
	if job.ID != high {
		t.Errorf("expected high-priority job first, got %q", job.ID)
	}
}

func TestUpdateJobStatus_Done(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = EnqueueJob(db, id, "qa", "/books/x", 1, map[string]any{}, false, 0)

	if err := UpdateJobStatus(db, id, "done", `{"ok":true}`, ""); err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}

	job, _ := GetJob(db, id)
	if job.Status != "done" {
		t.Errorf("Status: got %q, want done", job.Status)
	}
	if job.Result != `{"ok":true}` {
		t.Errorf("Result: got %q", job.Result)
	}
	if job.FinishedAt == nil {
		t.Error("FinishedAt should be set for done jobs")
	}
}

func TestUpdateJobStatus_Failed(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = EnqueueJob(db, id, "write", "/books/x", 1, map[string]any{}, false, 0)
	_ = UpdateJobStatus(db, id, "failed", "", "context deadline exceeded")

	job, _ := GetJob(db, id)
	if job.Status != "failed" {
		t.Errorf("Status: got %q, want failed", job.Status)
	}
	if job.Error != "context deadline exceeded" {
		t.Errorf("Error: got %q", job.Error)
	}
}

func TestCancelJob(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	id := uuid.NewString()
	_ = EnqueueJob(db, id, "research", "/books/x", 1, map[string]any{}, false, 0)
	if err := CancelJob(db, id); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}

	job, _ := GetJob(db, id)
	if job.Status != "cancelled" {
		t.Errorf("Status: got %q, want cancelled", job.Status)
	}
}

func TestListJobs_AllStatuses(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	for i, jt := range []string{"write", "qa", "pipeline"} {
		id := uuid.NewString()
		_ = EnqueueJob(db, id, jt, "/books/x", i+1, map[string]any{}, false, 0)
	}

	all, err := ListJobs(db, "", 50)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListJobs all: got %d, want 3", len(all))
	}

	pending, err := ListJobs(db, "pending", 50)
	if err != nil {
		t.Fatalf("ListJobs pending: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("ListJobs pending: got %d, want 3", len(pending))
	}
}

func TestListJobs_LimitRespected(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	for i := range 5 {
		_ = EnqueueJob(db, uuid.NewString(), "write", "/books/x", i+1, map[string]any{}, false, 0)
	}

	jobs, err := ListJobs(db, "pending", 3)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("limit=3: got %d jobs", len(jobs))
	}
}

// ── Stage progress ─────────────────────────────────────────────────────────────

func TestMarkStageComplete_RoundTrip(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	jobID := enqueueTestJob(t, db)

	if err := MarkStageComplete(db, jobID, "context"); err != nil {
		t.Fatalf("MarkStageComplete: %v", err)
	}

	stages, err := CompletedStages(db, jobID)
	if err != nil {
		t.Fatalf("CompletedStages: %v", err)
	}
	if !stages["context"] {
		t.Error("expected 'context' in completed stages")
	}
	if stages["write"] {
		t.Error("'write' should not be in completed stages")
	}
}

func TestMarkStageComplete_Idempotent(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	jobID := enqueueTestJob(t, db)

	for i := range 3 {
		if err := MarkStageComplete(db, jobID, "write"); err != nil {
			t.Fatalf("MarkStageComplete attempt %d: %v", i+1, err)
		}
	}

	stages, err := CompletedStages(db, jobID)
	if err != nil {
		t.Fatalf("CompletedStages: %v", err)
	}
	if !stages["write"] {
		t.Error("expected 'write' to be recorded")
	}
}

func TestCompletedStages_EmptyForNewJob(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	jobID := enqueueTestJob(t, db)

	stages, err := CompletedStages(db, jobID)
	if err != nil {
		t.Fatalf("CompletedStages: %v", err)
	}
	if len(stages) != 0 {
		t.Errorf("expected 0 completed stages for new job, got %d", len(stages))
	}
}

func TestCompletedStages_MultipleStages(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	jobID := enqueueTestJob(t, db)

	for _, stage := range []string{"context", "write", "qa"} {
		if err := MarkStageComplete(db, jobID, stage); err != nil {
			t.Fatalf("MarkStageComplete(%s): %v", stage, err)
		}
	}

	stages, err := CompletedStages(db, jobID)
	if err != nil {
		t.Fatalf("CompletedStages: %v", err)
	}
	for _, name := range []string{"context", "write", "qa"} {
		if !stages[name] {
			t.Errorf("expected stage %q in completed set", name)
		}
	}
}

// enqueueTestJob is a helper that inserts a minimal job and returns its ID.
func enqueueTestJob(t *testing.T, sqlDB *sql.DB) string {
	t.Helper()
	id := uuid.NewString()
	if err := EnqueueJob(sqlDB, id, "pipeline", "/books/test", 1, map[string]any{}, false, 0); err != nil {
		t.Fatalf("enqueueTestJob: %v", err)
	}
	return id
}
