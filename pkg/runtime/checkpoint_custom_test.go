package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

// customState has a non-exported field that json.Marshal would normally skip.
// We use custom serialization to persist it.
type customState struct {
	ThreadID string
	Counter  int
	secret   string // not exported, won't json.Marshal by default
}

func TestDBCheckpointer_CustomSerializeDeserialize(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	serialize := func(s customState) ([]byte, error) {
		// Use a map to capture the secret field too
		m := map[string]any{
			"thread_id": s.ThreadID,
			"counter":   s.Counter,
			"secret":    s.secret,
		}
		return json.Marshal(m)
	}

	deserialize := func(data []byte) (customState, error) {
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return customState{}, err
		}
		return customState{
			ThreadID: m["thread_id"].(string),
			Counter:  int(m["counter"].(float64)),
			secret:   m["secret"].(string),
		}, nil
	}

	cp := &DBCheckpointer[customState]{
		DB:          db,
		Serialize:   serialize,
		Deserialize: deserialize,
	}
	ctx := context.Background()

	// Put a state with a secret
	original := customState{ThreadID: "t1", Counter: 42, secret: "shh"}
	if err := cp.Put(ctx, "t1", original); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Get it back
	loaded, err := cp.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.ThreadID != "t1" {
		t.Errorf("ThreadID = %q, want %q", loaded.ThreadID, "t1")
	}
	if loaded.Counter != 42 {
		t.Errorf("Counter = %d, want 42", loaded.Counter)
	}
	if loaded.secret != "shh" {
		t.Errorf("secret = %q, want %q", loaded.secret, "shh")
	}
}

func TestDBCheckpointer_CustomSerializeError(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	cp := &DBCheckpointer[customState]{
		DB: db,
		Serialize: func(s customState) ([]byte, error) {
			return nil, fmt.Errorf("serialize fail")
		},
	}
	ctx := context.Background()

	if err := cp.Put(ctx, "t1", customState{}); err == nil {
		t.Error("expected Put to fail when Serialize errors")
	}
}

func TestDBCheckpointer_CustomDeserializeError(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	cp := &DBCheckpointer[customState]{
		DB: db,
		Serialize: func(s customState) ([]byte, error) {
			return json.Marshal(s)
		},
		Deserialize: func(data []byte) (customState, error) {
			return customState{}, fmt.Errorf("deserialize fail")
		},
	}
	ctx := context.Background()

	if err := cp.Put(ctx, "t1", customState{ThreadID: "t1"}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, err := cp.Get(ctx, "t1")
	if err == nil {
		t.Error("expected Get to fail when Deserialize errors")
	}
}

func TestDBCheckpointer_DefaultSerialization(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	// When Serialize/Deserialize are nil, should fall back to json.Marshal/Unmarshal
	cp := &DBCheckpointer[map[string]int]{DB: db}
	ctx := context.Background()

	state := map[string]int{"a": 1, "b": 2}
	if err := cp.Put(ctx, "map-thread", state); err != nil {
		t.Fatalf("Put: %v", err)
	}

	loaded, err := cp.Get(ctx, "map-thread")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded["a"] != 1 || loaded["b"] != 2 {
		t.Errorf("loaded = %v, want map[a:1 b:2]", loaded)
	}
}

func TestDBCheckpointer_NotFound(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	cp := &DBCheckpointer[customState]{DB: db}
	ctx := context.Background()

	_, err := cp.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrCheckpointNotFound) {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db, func() { db.Close() }
}
