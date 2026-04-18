// Package runtime provides the core LangGraph-style abstractions for the nqb
// agent and pipeline systems: Runnable, StateGraph, Tool, Checkpointer, and
// CallbackHandler.
package runtime

import (
	"context"
	"time"
)

// Runnable is the universal interface for executable units.
// LLMs, agents, pipeline stages, and tools can all implement Runnable.
type Runnable interface {
	Invoke(ctx context.Context, input any, opts ...Option) (any, error)
	Stream(ctx context.Context, input any, opts ...Option) (<-chan StreamEvent, error)
}

// StreamEvent is a single event emitted during streaming execution.
type StreamEvent struct {
	Type    string
	Content string
	Error   error
	Meta    map[string]any
}

// RunConfig carries execution-scoped settings.
type RunConfig struct {
	ThreadID           string
	Callbacks          CallbackHandler
	Checkpointer       Checkpointer[any]
	Metadata           map[string]any
	SkipCheckpointLoad bool
}

// Option configures a RunConfig.
type Option func(*RunConfig)

// WithThreadID sets the thread/checkpoint ID.
func WithThreadID(id string) Option {
	return func(c *RunConfig) { c.ThreadID = id }
}

// WithCallbacks attaches a callback handler.
func WithCallbacks(cb CallbackHandler) Option {
	return func(c *RunConfig) { c.Callbacks = cb }
}

// WithCheckpointer attaches a checkpointer.
func WithCheckpointer(cp Checkpointer[any]) Option {
	return func(c *RunConfig) { c.Checkpointer = cp }
}

// WithSkipCheckpointLoad prevents Invoke from loading a previous checkpoint.
func WithSkipCheckpointLoad() Option {
	return func(c *RunConfig) { c.SkipCheckpointLoad = true }
}

// AgentRunResult is the standard output of an agentic Runnable invocation.
type AgentRunResult struct {
	Output    string
	SessionID string
	TokensIn  int64
	TokensOut int64
	Steps     int
}

// WriterFactory creates agentic writers (Runnables) for a given book project.
type WriterFactory interface {
	NewWriter(bookDir string, cfg any, modelID string) Runnable
}

// SessionInfo is a single persisted chat session.
type SessionInfo struct {
	ID         string
	BookDir    string
	ChapterNum int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SessionStore persists and retrieves agent chat sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, sessionID, bookDir string, chapterNum int) error
	AppendMessage(ctx context.Context, msgID, sessionID, role, content, model string, tokensIn, tokensOut int) error
	TouchSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, bookDir string, limit int) ([]SessionInfo, error)
}

// EpistemicState represents a loaded epistemic / knowledge-graph summary.
type EpistemicState interface {
	Summary() string
}

// EpistemicStore loads the knowledge-graph state for a book.
type EpistemicStore interface {
	Load(ctx context.Context, bookID string) (EpistemicState, error)
}
