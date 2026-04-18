package runtime

import "context"

// Tool is a first-class callable capability.
// Tools are registered in a ToolRegistry and can be bound to agent runtimes
// (e.g. charm.land/fantasy) via an adapter.
type Tool interface {
	Name() string
	Description() string
	Schema() any
	Invoke(ctx context.Context, input string, opts ...Option) (string, error)
}

// TaskSpawner is an interface for launching background tasks from tools.
// Implemented by the TUI task tracker and job queue.
type TaskSpawner interface {
	Spawn(label, taskType string, chapterNum int, fn func(ctx context.Context) (string, error)) string
}
