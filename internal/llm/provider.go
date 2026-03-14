// Package llm defines the Provider interface and shared types for LLM access.
package llm

import "context"

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

// ProviderConfig holds configuration for a named LLM provider.
type ProviderConfig struct {
	// Type is the provider kind: "anthropic", "openai-compat", "gemini".
	Type string `yaml:"type"`
	// APIKey for this provider. Falls back to environment variable if empty.
	APIKey string `yaml:"api_key,omitempty"`
	// BaseURL for OpenAI-compatible providers (e.g. Ollama, DeepSeek, z.ai).
	BaseURL string `yaml:"base_url,omitempty"`
}
