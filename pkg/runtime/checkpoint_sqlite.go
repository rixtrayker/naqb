// Package runtime provides the core LangGraph-style abstractions for the nqb
// agent and pipeline systems: Runnable, StateGraph, Tool, Checkpointer, and
// CallbackHandler.
package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DBCheckpointer is a SQLite-backed Checkpointer implementation.
// It serializes state as JSON and stores it in a dedicated table.
// Custom Serialize/Deserialize functions can be provided for states that
// are not directly JSON-serializable (e.g. those containing interface fields).
type DBCheckpointer[State any] struct {
	DB          *sql.DB
	Serialize   func(State) ([]byte, error)
	Deserialize func([]byte) (State, error)

	initOnce sync.Once
	initErr  error
}

// Get loads the latest checkpoint for a thread.
// Returns ErrCheckpointNotFound if the thread has no persisted state.
func (c *DBCheckpointer[State]) Get(ctx context.Context, threadID string) (State, error) {
	var zero State
	if err := c.ensureSchema(ctx); err != nil {
		return zero, err
	}

	var raw []byte
	err := c.DB.QueryRowContext(ctx,
		`SELECT state_json FROM runtime_checkpoints WHERE thread_id = ?`,
		threadID,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return zero, ErrCheckpointNotFound
	}
	if err != nil {
		return zero, fmt.Errorf("checkpoint get: %w", err)
	}

	deserialize := c.Deserialize
	if deserialize == nil {
		deserialize = func(data []byte) (State, error) {
			var state State
			if err := json.Unmarshal(data, &state); err != nil {
				return state, err
			}
			return state, nil
		}
	}
	state, err := deserialize(raw)
	if err != nil {
		return zero, fmt.Errorf("checkpoint unmarshal: %w", err)
	}
	return state, nil
}

// Put persists a checkpoint for a thread, overwriting any existing one.
func (c *DBCheckpointer[State]) Put(ctx context.Context, threadID string, state State) error {
	if err := c.ensureSchema(ctx); err != nil {
		return err
	}

	serialize := c.Serialize
	if serialize == nil {
		serialize = func(s State) ([]byte, error) {
			return json.Marshal(s)
		}
	}
	raw, err := serialize(state)
	if err != nil {
		return fmt.Errorf("checkpoint marshal: %w", err)
	}

	_, err = c.DB.ExecContext(ctx,
		`INSERT INTO runtime_checkpoints (thread_id, state_json, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(thread_id) DO UPDATE SET
		   state_json = excluded.state_json,
		   updated_at = excluded.updated_at`,
		threadID, string(raw), time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("checkpoint put: %w", err)
	}
	return nil
}

// Delete removes a checkpoint for a thread.
func (c *DBCheckpointer[State]) Delete(ctx context.Context, threadID string) error {
	if err := c.ensureSchema(ctx); err != nil {
		return err
	}
	_, err := c.DB.ExecContext(ctx,
		`DELETE FROM runtime_checkpoints WHERE thread_id = ?`, threadID)
	return err
}

func (c *DBCheckpointer[State]) ensureSchema(ctx context.Context) error {
	c.initOnce.Do(func() {
		_, c.initErr = c.DB.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS runtime_checkpoints (
				thread_id  TEXT PRIMARY KEY,
				state_json TEXT NOT NULL,
				updated_at DATETIME NOT NULL
			)
		`)
	})
	return c.initErr
}
