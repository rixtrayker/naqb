package runtime

import (
	"context"
	"errors"
)

// ErrCheckpointNotFound is returned by a Checkpointer when no snapshot exists.
var ErrCheckpointNotFound = errors.New("checkpoint not found")

// Checkpointer persists and resumes state snapshots (LangGraph-style).
type Checkpointer[State any] interface {
	Get(ctx context.Context, threadID string) (State, error)
	Put(ctx context.Context, threadID string, state State) error
}
