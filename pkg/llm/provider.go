// Package llm defines the Provider interface and shared types for LLM access.
package llm

import (
	"context"

	"github.com/amr/naqb/pkg/config"
)

// Message is a simple chat message.
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// StreamFunc is called with each streamed text delta.
type StreamFunc func(delta string) error

// Provider is the common interface for all LLM backends.
// Each provider implementation must be safe for concurrent use.
type Provider interface {
	// Complete sends a non-streaming request and returns the full response.
	Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error)

	// Stream sends a streaming request, calling onDelta for each text chunk.
	// Returns the full assembled response.
	Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error)
}

// TokenReporter is optionally implemented by providers that track token usage.
// After each Complete call the provider stores the last token counts.
type TokenReporter interface {
	LastTokens() (inputTokens, outputTokens int)
}

// ProviderConfig is an alias for config.ProviderConfig — single source of truth.
// All provider configuration lives in the config package; llm refers to it here.
type ProviderConfig = config.ProviderConfig
