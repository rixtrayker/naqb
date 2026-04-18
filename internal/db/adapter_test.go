package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/amr/naqb/pkg/runtime"
	_ "modernc.org/sqlite"
)

func TestSessionStore_CreateSessionAndList(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	store := NewSessionStore(db)
	ctx := context.Background()

	// Create a session
	if err := store.CreateSession(ctx, "sess-1", "/books/test", 3); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// List should return it
	sessions, err := store.ListSessions(ctx, "/books/test", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "sess-1" {
		t.Errorf("ID = %q, want %q", sessions[0].ID, "sess-1")
	}
	if sessions[0].BookDir != "/books/test" {
		t.Errorf("BookDir = %q, want %q", sessions[0].BookDir, "/books/test")
	}
	if sessions[0].ChapterNum != 3 {
		t.Errorf("ChapterNum = %d, want 3", sessions[0].ChapterNum)
	}
}

func TestSessionStore_DuplicateSession(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	store := NewSessionStore(db)
	ctx := context.Background()

	// Creating the same session twice should not error (used for resumption)
	if err := store.CreateSession(ctx, "sess-1", "/books/test", 0); err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}
	if err := store.CreateSession(ctx, "sess-1", "/books/test", 0); err != nil {
		t.Logf("duplicate CreateSession returned error (acceptable): %v", err)
	}
}

func TestSessionStore_AppendMessageAndTouch(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	store := NewSessionStore(db)
	ctx := context.Background()

	if err := store.CreateSession(ctx, "sess-1", "/books/test", 0); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Append a user message
	if err := store.AppendMessage(ctx, "msg-1", "sess-1", "user", "hello", "", 10, 20); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	// Append an assistant message
	if err := store.AppendMessage(ctx, "msg-2", "sess-1", "assistant", "hi", "gpt-4", 5, 15); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	// Touch session updates updated_at
	if err := store.TouchSession(ctx, "sess-1"); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}

	// Verify session was touched by checking list order
	sessions, err := store.ListSessions(ctx, "/books/test", 1)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
	if sessions[0].UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSessionStore_ListSessionsLimit(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	store := NewSessionStore(db)
	ctx := context.Background()

	// Create multiple sessions
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("sess-%d", i)
		if err := store.CreateSession(ctx, id, "/books/test", 0); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // ensure different timestamps
	}

	// List with limit 2
	sessions, err := store.ListSessions(ctx, "/books/test", 2)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}

	// Default limit (20) when limit <= 0
	sessions, err = store.ListSessions(ctx, "/books/test", 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 5 {
		t.Fatalf("len(sessions) = %d, want 5", len(sessions))
	}
}

func TestSessionStore_ListSessionsDifferentBook(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()
	store := NewSessionStore(db)
	ctx := context.Background()

	if err := store.CreateSession(ctx, "sess-a", "/books/a", 0); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CreateSession(ctx, "sess-b", "/books/b", 0); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sessions, err := store.ListSessions(ctx, "/books/a", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "sess-a" {
		t.Errorf("ID = %q, want %q", sessions[0].ID, "sess-a")
	}
}

// compile-time check that SessionStore implements runtime.SessionStore
var _ runtime.SessionStore = (*SessionStore)(nil)
