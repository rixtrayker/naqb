package agent

import (
	"context"
	"errors"
	"iter"
	"testing"

	"charm.land/fantasy"
	"github.com/amr/naqb/pkg/runtime"
)

// mockFantasyProvider implements fantasy.Provider for testing.
type mockFantasyProvider struct {
	model fantasy.LanguageModel
}

func (m *mockFantasyProvider) Name() string { return "mock" }

func (m *mockFantasyProvider) LanguageModel(ctx context.Context, modelID string) (fantasy.LanguageModel, error) {
	if m.model == nil {
		return nil, errors.New("no model configured")
	}
	return m.model, nil
}

// mockLanguageModel implements fantasy.LanguageModel for testing.
type mockLanguageModel struct {
	streamParts []fantasy.StreamPart
	streamErr   error
}

func (m *mockLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("Generate not implemented")
}

func (m *mockLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return func(yield func(fantasy.StreamPart) bool) {
		for _, part := range m.streamParts {
			if !yield(part) {
				return
			}
		}
	}, nil
}

func (m *mockLanguageModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("GenerateObject not implemented")
}

func (m *mockLanguageModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("StreamObject not implemented")
}

func (m *mockLanguageModel) Provider() string { return "mock" }
func (m *mockLanguageModel) Model() string      { return "mock-model" }

// mockRuntimeTool implements runtime.Tool and can optionally expose a FantasyTool.
type mockRuntimeTool struct {
	name        string
	description string
	invokeFunc  func(ctx context.Context, input string) (string, error)
}

func (m *mockRuntimeTool) Name() string        { return m.name }
func (m *mockRuntimeTool) Description() string { return m.description }
func (m *mockRuntimeTool) Schema() any         { return nil }

func (m *mockRuntimeTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, input)
	}
	return "mock result", nil
}

func makeTextStartStreamPart(id string) fantasy.StreamPart {
	return fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: id}
}

func makeTextStreamPart(id, delta string) fantasy.StreamPart {
	return fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: id, Delta: delta}
}

func makeTextEndStreamPart(id string) fantasy.StreamPart {
	return fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: id}
}

func makeFinishStreamPart() fantasy.StreamPart {
	return fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop}
}

func makeFinishStreamPartWithUsage(inputTokens, outputTokens int64) fantasy.StreamPart {
	return fantasy.StreamPart{
		Type:         fantasy.StreamPartTypeFinish,
		FinishReason: fantasy.FinishReasonStop,
		Usage:        fantasy.Usage{InputTokens: inputTokens, OutputTokens: outputTokens, TotalTokens: inputTokens + outputTokens},
	}
}

func TestAgent_Run_Streaming(t *testing.T) {
	lm := &mockLanguageModel{
		streamParts: []fantasy.StreamPart{
			makeTextStartStreamPart("text-1"),
			makeTextStreamPart("text-1", "Hello"),
			makeTextStreamPart("text-1", " world"),
			makeTextEndStreamPart("text-1"),
			makeFinishStreamPart(),
		},
	}
	provider := &mockFantasyProvider{model: lm}
	agent := New(provider, "mock-model", "/books/test", nil)

	var deltas []string
	result, err := agent.Run(context.Background(), "say hello", "", func(text string) {
		deltas = append(deltas, text)
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Output != "Hello world" {
		t.Errorf("Output = %q, want %q", result.Output, "Hello world")
	}

	if len(deltas) != 2 {
		t.Errorf("deltas = %d, want 2", len(deltas))
	}
	if deltas[0] != "Hello" {
		t.Errorf("delta[0] = %q, want Hello", deltas[0])
	}
	if deltas[1] != " world" {
		t.Errorf("delta[1] = %q, want \" world\"", deltas[1])
	}
}

func TestAgent_Run_SessionPersistence(t *testing.T) {
	lm := &mockLanguageModel{
		streamParts: []fantasy.StreamPart{
			makeTextStartStreamPart("text-1"),
			makeTextStreamPart("text-1", "done"),
			makeTextEndStreamPart("text-1"),
			makeFinishStreamPart(),
		},
	}
	provider := &mockFantasyProvider{model: lm}
	store := &mockSessionStore{}

	agent := New(provider, "mock-model", "/books/test", nil, WithSessionStore(store))

	_, err := agent.Run(context.Background(), "test task", "session-123", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(store.created) != 1 || store.created[0] != "session-123" {
		t.Errorf("created sessions = %v, want [session-123]", store.created)
	}

	// Should have appended user message + at least one assistant message
	if len(store.appended) < 2 {
		t.Fatalf("expected at least 2 appended messages, got %d", len(store.appended))
	}

	if store.appended[0].role != "user" {
		t.Errorf("first message role = %q, want user", store.appended[0].role)
	}
	if store.appended[0].content != "test task" {
		t.Errorf("first message content = %q, want \"test task\"", store.appended[0].content)
	}

	if store.appended[1].role != "assistant" {
		t.Errorf("second message role = %q, want assistant", store.appended[1].role)
	}

	if len(store.touched) < 1 {
		t.Error("expected TouchSession to be called")
	}
}

func TestAgent_Run_NilProvider(t *testing.T) {
	agent := New(nil, "mock-model", "/books/test", nil)
	_, err := agent.Run(context.Background(), "test", "", nil)
	if err == nil {
		t.Fatal("expected error when provider is nil")
	}
	if !errors.Is(err, errors.New("agent: no provider configured")) {
		// Error message comparison
		if err.Error() != "agent: no provider configured" {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestAgent_Run_ProviderError(t *testing.T) {
	provider := &mockFantasyProvider{model: nil}
	agent := New(provider, "mock-model", "/books/test", nil)
	_, err := agent.Run(context.Background(), "test", "", nil)
	if err == nil {
		t.Fatal("expected error when LanguageModel fails")
	}
}

func TestAgent_Run_WithTool(t *testing.T) {
	tool := &mockRuntimeTool{
		name:        "echo",
		description: "echoes input",
		invokeFunc: func(ctx context.Context, input string) (string, error) {
			return "echo: " + input, nil
		},
	}

	lm := &mockLanguageModel{
		streamParts: []fantasy.StreamPart{
			makeTextStartStreamPart("text-1"),
			makeTextStreamPart("text-1", "Using tool..."),
			makeTextEndStreamPart("text-1"),
			makeFinishStreamPart(),
		},
	}
	provider := &mockFantasyProvider{model: lm}
	agent := New(provider, "mock-model", "/books/test", nil, WithTools([]runtime.Tool{tool}))

	// Verify the agent runs successfully when tools are registered.
	// Full tool-call integration (model emits ToolCall → agent invokes tool →
	// model continues) requires complex stream choreography; this test verifies
	// the wiring is correct.
	_, err := agent.Run(context.Background(), "use echo tool", "", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestAgent_Run_TokenCounting(t *testing.T) {
	lm := &mockLanguageModel{
		streamParts: []fantasy.StreamPart{
			makeTextStartStreamPart("text-1"),
			makeTextStreamPart("text-1", "result"),
			makeTextEndStreamPart("text-1"),
			makeFinishStreamPartWithUsage(10, 20),
		},
	}
	provider := &mockFantasyProvider{model: lm}
	agent := New(provider, "mock-model", "/books/test", nil)

	result, err := agent.Run(context.Background(), "test", "", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.TokensIn != 10 {
		t.Errorf("TokensIn = %d, want 10", result.TokensIn)
	}
	if result.TokensOut != 20 {
		t.Errorf("TokensOut = %d, want 20", result.TokensOut)
	}
	if result.Steps != 1 {
		t.Errorf("Steps = %d, want 1", result.Steps)
	}
}

// Helper to collect all values from an iter.Seq.
func collectSeq[T any](seq iter.Seq[T]) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}
