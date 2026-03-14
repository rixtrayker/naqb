package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/amr/naqb/internal/log"
)

// Client wraps the Anthropic SDK.
type Client struct {
	inner *anthropic.Client
}

// Message is a simple chat message.
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// New creates a Client with the provided API key.
func New(apiKey string) *Client {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Client{inner: &c}
}

// Complete sends a single non-streaming request and returns the full response.
func (c *Client) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	log.Debug("LLM complete", "model", model, "max_tokens", maxTokens, "messages", len(messages))

	msgs := make([]anthropic.MessageParam, len(messages))
	for i, m := range messages {
		if m.Role == "user" {
			msgs[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		} else {
			msgs[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	resp, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		log.Error("LLM complete failed", "model", model, "err", err)
		return "", fmt.Errorf("anthropic API error: %w", err)
	}
	if len(resp.Content) == 0 {
		log.Error("LLM complete returned empty content", "model", model)
		return "", fmt.Errorf("empty response from API")
	}
	log.Debug("LLM complete done", "model", model, "stop_reason", resp.StopReason, "output_tokens", resp.Usage.OutputTokens)
	return resp.Content[0].Text, nil
}

// StreamFunc is called with each streamed text delta.
type StreamFunc func(delta string) error

// Stream sends a request with streaming, calling onDelta for each text chunk.
// Returns the full assembled response.
func (c *Client) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	log.Debug("LLM stream start", "model", model, "max_tokens", maxTokens, "messages", len(messages))

	msgs := make([]anthropic.MessageParam, len(messages))
	for i, m := range messages {
		if m.Role == "user" {
			msgs[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		} else {
			msgs[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	stream := c.inner.Messages.NewStreaming(ctx, params)

	var full string
	for stream.Next() {
		event := stream.Current()
		switch e := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if delta, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
				full += delta.Text
				if onDelta != nil {
					if err := onDelta(delta.Text); err != nil {
						return full, err
					}
				}
			}
		}
	}
	if err := stream.Err(); err != nil && err != io.EOF {
		log.Error("LLM stream error", "model", model, "err", err)
		return full, fmt.Errorf("streaming error: %w", err)
	}
	log.Debug("LLM stream done", "model", model, "total_chars", len(full))
	return full, nil
}
