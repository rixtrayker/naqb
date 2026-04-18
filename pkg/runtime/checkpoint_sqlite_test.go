package runtime

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDBCheckpointer_PutAndGet(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	cp := &DBCheckpointer[map[string]string]{DB: db}

	_, err = cp.Get(ctx, "thread-1")
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("expected ErrCheckpointNotFound, got %v", err)
	}

	state := map[string]string{"key": "value", "chapter": "one"}
	if err := cp.Put(ctx, "thread-1", state); err != nil {
		t.Fatalf("put: %v", err)
	}

	loaded, err := cp.Get(ctx, "thread-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded["key"] != "value" || loaded["chapter"] != "one" {
		t.Fatalf("unexpected state: %v", loaded)
	}
}

func TestDBCheckpointer_Overwrite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	cp := &DBCheckpointer[int]{DB: db}

	if err := cp.Put(ctx, "thread-2", 42); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := cp.Put(ctx, "thread-2", 99); err != nil {
		t.Fatalf("put overwrite: %v", err)
	}

	loaded, err := cp.Get(ctx, "thread-2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded != 99 {
		t.Fatalf("expected 99, got %d", loaded)
	}
}

func TestDBCheckpointer_Delete(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	cp := &DBCheckpointer[string]{DB: db}

	if err := cp.Put(ctx, "thread-3", "hello"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := cp.Delete(ctx, "thread-3"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = cp.Get(ctx, "thread-3")
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Fatalf("expected ErrCheckpointNotFound after delete, got %v", err)
	}
}
