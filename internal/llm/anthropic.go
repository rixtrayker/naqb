package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/amr/naqb/internal/log"
)

// AnthropicProvider implements Provider using the Anthropic SDK.
type AnthropicProvider struct {
	inner    *anthropic.Client
	lastIn   int
	lastOut  int
}

// NewAnthropic creates an AnthropicProvider with the provided API key.
func NewAnthropic(apiKey string) *AnthropicProvider {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{inner: &c}
}

// Complete sends a single non-streaming request and returns the full response.
func (p *AnthropicProvider) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	log.Debug("LLM complete", "provider", "anthropic", "model", model, "max_tokens", maxTokens, "messages", len(messages))

	msgs := buildAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	resp, err := p.inner.Messages.New(ctx, params)
	if err != nil {
		log.Error("LLM complete failed", "model", model, "err", err)
		return "", fmt.Errorf("anthropic API error: %w", err)
	}
	if len(resp.Content) == 0 {
		log.Error("LLM complete returned empty content", "model", model)
		return "", fmt.Errorf("empty response from API")
	}
	p.lastIn = int(resp.Usage.InputTokens)
	p.lastOut = int(resp.Usage.OutputTokens)
	log.Debug("LLM complete done", "model", model, "stop_reason", resp.StopReason, "input_tokens", p.lastIn, "output_tokens", p.lastOut)
	return resp.Content[0].Text, nil
}

// LastTokens implements TokenReporter — returns usage from the last Complete call.
func (p *AnthropicProvider) LastTokens() (int, int) {
	return p.lastIn, p.lastOut
}

// Stream sends a request with streaming, calling onDelta for each text chunk.
// Returns the full assembled response.
func (p *AnthropicProvider) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	log.Debug("LLM stream start", "provider", "anthropic", "model", model, "max_tokens", maxTokens, "messages", len(messages))

	msgs := buildAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	stream := p.inner.Messages.NewStreaming(ctx, params)

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

func buildAnthropicMessages(messages []Message) []anthropic.MessageParam {
	msgs := make([]anthropic.MessageParam, len(messages))
	for i, m := range messages {
		if m.Role == "user" {
			msgs[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		} else {
			msgs[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}
	return msgs
}
