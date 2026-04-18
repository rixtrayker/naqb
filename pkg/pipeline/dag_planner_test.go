package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/llm"
)

// mockPlannerProvider implements llm.Provider for DAG planner tests.
type mockPlannerProvider struct {
	response string
	err      error
}

func (m *mockPlannerProvider) Complete(ctx context.Context, model, system string, messages []llm.Message, maxTokens int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockPlannerProvider) Stream(ctx context.Context, model, system string, messages []llm.Message, maxTokens int, onDelta llm.StreamFunc) (string, error) {
	return "", errors.New("Stream not implemented")
}

func TestRunDAGPlanner_Success(t *testing.T) {
	response := `name: research-heavy
stages:
  - id: research
    type: RESEARCH
    depends_on: []
  - id: context
    type: CONTEXT
    depends_on: [research]
  - id: write
    type: WRITE
    depends_on: [context]
`
	client := &mockPlannerProvider{response: response}
	result, err := RunDAGPlanner(context.Background(), client, "write a research chapter")
	if err != nil {
		t.Fatalf("RunDAGPlanner: %v", err)
	}

	if result.Template.Name != "research-heavy" {
		t.Errorf("Name = %q, want research-heavy", result.Template.Name)
	}
	if len(result.Template.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d", len(result.Template.Stages))
	}
	if result.RawYAML == "" {
		t.Error("expected non-empty RawYAML")
	}
}

func TestRunDAGPlanner_WithMarkdownFences(t *testing.T) {
	response := "```yaml\nname: fenced\nstages:\n  - id: a\n    type: CONTEXT\n```"
	client := &mockPlannerProvider{response: response}
	result, err := RunDAGPlanner(context.Background(), client, "test")
	if err != nil {
		t.Fatalf("RunDAGPlanner: %v", err)
	}
	if result.Template.Name != "fenced" {
		t.Errorf("Name = %q, want fenced", result.Template.Name)
	}
}

func TestRunDAGPlanner_LLMError(t *testing.T) {
	client := &mockPlannerProvider{err: errors.New("llm down")}
	_, err := RunDAGPlanner(context.Background(), client, "test")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
	if !strings.Contains(err.Error(), "dag planner LLM failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDAGPlanner_InvalidYAML(t *testing.T) {
	client := &mockPlannerProvider{response: "{[unclosed"}
	_, err := RunDAGPlanner(context.Background(), client, "test")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parse YAML") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDAGPlanner_InvalidDAG(t *testing.T) {
	response := `name: cyclic
stages:
  - id: a
    type: CONTEXT
    depends_on: [b]
  - id: b
    type: WRITE
    depends_on: [a]
`
	client := &mockPlannerProvider{response: response}
	_, err := RunDAGPlanner(context.Background(), client, "test")
	if err == nil {
		t.Fatal("expected error for cyclic DAG")
	}
	if !strings.Contains(err.Error(), "invalid DAG") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStripMarkdownFences(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"```yaml\nname: test\n```", "name: test"},
		{"```yml\nname: test\n```", "name: test"},
		{"```\nname: test\n```", "name: test"},
		{"```yaml\nname: test\n```\ntrailing", "name: test"}, // trailing text after fence is ignored
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := stripMarkdownFences(tc.input)
			if got != tc.want {
				t.Errorf("stripMarkdownFences(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
