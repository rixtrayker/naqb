package booktools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/runtime"
)

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	responses       []string
	idx             int
	defaultResponse string
}

func (m *mockProvider) next() string {
	if m.idx < len(m.responses) {
		r := m.responses[m.idx]
		m.idx++
		return r
	}
	if m.defaultResponse != "" {
		return m.defaultResponse
	}
	return ""
}

func (m *mockProvider) Complete(ctx context.Context, model, system string, messages []llm.Message, maxTokens int) (string, error) {
	r := m.next()
	if r == "" {
		return "", errors.New("no more mock responses")
	}
	return r, nil
}

func (m *mockProvider) Stream(ctx context.Context, model, system string, messages []llm.Message, maxTokens int, onDelta llm.StreamFunc) (string, error) {
	r := m.next()
	if r == "" {
		return "", errors.New("no more mock responses")
	}
	if onDelta != nil {
		onDelta(r)
	}
	return r, nil
}

// mockTool records invocations for verification.
type mockTool struct {
	name        string
	description string
	result      string
	err         error
	invocations []string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Schema() any         { return nil }

func (m *mockTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	m.invocations = append(m.invocations, input)
	if m.err != nil {
		return "", m.err
	}
	return m.result, nil
}

func makePlanJSON(steps []PlanStep) string {
	b, _ := json.Marshal(steps)
	return string(b)
}

func TestPlanAndExecuteTool_Success(t *testing.T) {
	registry := runtime.NewToolRegistry()
	toolA := &mockTool{name: "tool_a", description: "does A", result: "result A"}
	toolB := &mockTool{name: "tool_b", description: "does B", result: "result B"}
	registry.Register(toolA)
	registry.Register(toolB)

	plan := []PlanStep{
		{ID: "step1", Tool: "tool_a", Description: "Run tool A", Params: map[string]interface{}{"x": 1}},
		{ID: "step2", Tool: "tool_b", Description: "Run tool B", Params: map[string]interface{}{"y": 2}},
	}

	client := &mockProvider{responses: []string{makePlanJSON(plan)}}
	tool := NewPlanAndExecuteTool(registry, client, nil)

	input := `{"goal":"do something"}`
	result, err := tool.Invoke(context.Background(), input)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty result")
	}

	if len(toolA.invocations) != 1 {
		t.Errorf("tool_a invocations = %d, want 1", len(toolA.invocations))
	}
	if len(toolB.invocations) != 1 {
		t.Errorf("tool_b invocations = %d, want 1", len(toolB.invocations))
	}

	if !strings.Contains(result, "result A") {
		t.Error("expected trace to contain result A")
	}
	if !strings.Contains(result, "result B") {
		t.Error("expected trace to contain result B")
	}
}

func TestPlanAndExecuteTool_UnknownTool(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(&mockTool{name: "known", description: "known tool", result: "ok"})

	plan := []PlanStep{
		{ID: "step1", Tool: "unknown", Description: "Run unknown", Params: nil},
	}

	client := &mockProvider{responses: []string{makePlanJSON(plan)}}
	tool := NewPlanAndExecuteTool(registry, client, nil)

	result, err := tool.Invoke(context.Background(), `{"goal":"test"}`)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if !strings.Contains(result, "unknown tool") {
		t.Errorf("expected trace to mention unknown tool, got: %s", result)
	}
}

func TestPlanNode_Resumption(t *testing.T) {
	registry := runtime.NewToolRegistry()
	registry.Register(&mockTool{name: "tool_a", description: "does A", result: "result A"})

	// When Steps are already populated, planNode should skip the LLM call.
	client := &mockProvider{responses: []string{}} // no responses needed
	tool := NewPlanAndExecuteTool(registry, client, nil)

	// Call planNode directly with pre-populated steps
	pet := tool.(*PlanAndExecuteTool)
	node := pet.planNode("test goal")
	state := planExecuteState{
		Goal:  "test goal",
		Steps: []PlanStep{{ID: "s1", Tool: "tool_a", Description: "already planned"}},
	}

	result, err := node(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("planNode: %v", err)
	}

	if len(result.Steps) != 1 {
		t.Errorf("Steps = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].ID != "s1" {
		t.Errorf("Step.ID = %q, want s1", result.Steps[0].ID)
	}
}

func TestPlanAndExecuteTool_ToolError(t *testing.T) {
	registry := runtime.NewToolRegistry()
	failing := &mockTool{name: "failing", description: "always fails", err: errors.New("boom")}
	registry.Register(failing)

	plan := []PlanStep{
		{ID: "step1", Tool: "failing", Description: "Run failing tool", Params: nil},
	}

	client := &mockProvider{responses: []string{makePlanJSON(plan)}}
	tool := NewPlanAndExecuteTool(registry, client, nil)

	result, err := tool.Invoke(context.Background(), `{"goal":"test"}`)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if !strings.Contains(result, "boom") {
		t.Errorf("expected trace to contain error message, got: %s", result)
	}
	if !strings.Contains(result, "✗ Error") {
		t.Errorf("expected trace to show error marker, got: %s", result)
	}
}

func TestPlanAndExecuteTool_MalformedPlan(t *testing.T) {
	registry := runtime.NewToolRegistry()
	client := &mockProvider{responses: []string{"this is not json"}}
	tool := NewPlanAndExecuteTool(registry, client, nil)

	_, err := tool.Invoke(context.Background(), `{"goal":"test"}`)
	if err == nil {
		t.Fatal("expected error for malformed plan")
	}
	if !strings.Contains(err.Error(), "plan parse failed") {
		t.Errorf("expected 'plan parse failed' in error, got: %v", err)
	}
}

func TestSerializeDeserializePlanState(t *testing.T) {
	original := planExecuteState{
		Goal:  "test goal",
		Trace: "some trace",
		Steps: []PlanStep{
			{ID: "s1", Tool: "t1", Description: "d1", Params: map[string]interface{}{"key": "val"}},
		},
		GatesPassed: []string{"s1"},
	}

	data, err := serializePlanState(original)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	restored, err := deserializePlanState(data)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}

	if restored.Goal != original.Goal {
		t.Errorf("Goal = %q, want %q", restored.Goal, original.Goal)
	}
	if restored.Trace != original.Trace {
		t.Errorf("Trace = %q, want %q", restored.Trace, original.Trace)
	}
	if len(restored.Steps) != len(original.Steps) {
		t.Fatalf("Steps len = %d, want %d", len(restored.Steps), len(original.Steps))
	}
	if restored.Steps[0].ID != original.Steps[0].ID {
		t.Errorf("Step.ID = %q, want %q", restored.Steps[0].ID, original.Steps[0].ID)
	}
	if len(restored.GatesPassed) != 1 || restored.GatesPassed[0] != "s1" {
		t.Errorf("GatesPassed = %v, want [s1]", restored.GatesPassed)
	}
}

func TestStripMarkdownFencesPlan(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"plain json", "plain json"},
		{"```json\n[{}]\n```", "[{}]"},
		{"```yaml\nsteps:\n  - id: a\n```", "steps:\n  - id: a"},
		{"```\n[{}]\n```", "[{}]"},
		{"```json\n[{}]\n```\ntrailing", "[{}]"}, // trailing text after fence is ignored
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			got := stripMarkdownFencesPlan(tc.input)
			if got != tc.want {
				t.Errorf("stripMarkdownFencesPlan(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
