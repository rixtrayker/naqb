package runtime

import (
	"context"
	"errors"
	"testing"
)

// simpleState is a mutable state for testing.
type simpleState struct {
	Value   int
	Visited []string
	Done    bool
}

func nodeThatAppends(name string) Node[simpleState] {
	return func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		state.Visited = append(state.Visited, name)
		return state, nil
	}
}

func nodeThatAdds(n int) Node[simpleState] {
	return func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		state.Value += n
		return state, nil
	}
}

func nodeThatErrors(msg string) Node[simpleState] {
	return func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		return state, errors.New(msg)
	}
}

// ── Invoke: linear flow ─────────────────────────────────────────────────────

func TestInvoke_LinearFlow(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatAdds(10))
	g.AddNode("c", nodeThatAdds(100))
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	result, err := compiled.Invoke(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if result.Value != 111 {
		t.Errorf("Value = %d, want 111", result.Value)
	}
}

func TestInvoke_SingleNode(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("only", nodeThatAdds(42))
	g.SetEntryPoint("only")

	compiled := g.Compile()
	result, err := compiled.Invoke(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if result.Value != 42 {
		t.Errorf("Value = %d, want 42", result.Value)
	}
}

// ── Invoke: conditional edges ───────────────────────────────────────────────

func TestInvoke_ConditionalEdges(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("start", nodeThatAdds(0))
	g.AddNode("hot", nodeThatAdds(100))
	g.AddNode("cold", nodeThatAdds(1))
	g.AddConditionalEdges("start", func(state simpleState) string {
		if state.Value >= 50 {
			return "hot"
		}
		return "cold"
	})
	g.SetEntryPoint("start")

	compiled := g.Compile()

	// Path: start → cold (Value < 50)
	result, err := compiled.Invoke(context.Background(), simpleState{Value: 10})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Value != 11 {
		t.Errorf("cold path: Value = %d, want 11", result.Value)
	}

	// Path: start → hot (Value >= 50)
	result, err = compiled.Invoke(context.Background(), simpleState{Value: 60})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Value != 160 {
		t.Errorf("hot path: Value = %d, want 160", result.Value)
	}
}

func TestInvoke_ConditionalToExit(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("start", func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		state.Done = true
		return state, nil
	})
	g.AddConditionalEdges("start", func(state simpleState) string {
		if state.Done {
			return "" // exit
		}
		return "start"
	})
	g.SetEntryPoint("start")

	compiled := g.Compile()
	result, err := compiled.Invoke(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !result.Done {
		t.Error("expected Done = true")
	}
}

// ── Invoke: error handling ──────────────────────────────────────────────────

func TestInvoke_NodeError(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("ok", nodeThatAdds(1))
	g.AddNode("fail", nodeThatErrors("boom"))
	g.AddEdge("ok", "fail")
	g.SetEntryPoint("ok")

	compiled := g.Compile()
	_, err := compiled.Invoke(context.Background(), simpleState{})
	if err == nil {
		t.Fatal("expected error from failing node")
	}
	if !errors.Is(err, errors.New("boom")) {
		// wrapped error
		if !containsStr(err.Error(), "boom") {
			t.Errorf("expected error to contain 'boom', got: %v", err)
		}
	}
}

func TestInvoke_UnknownNode(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.SetEntryPoint("missing")

	compiled := g.Compile()
	_, err := compiled.Invoke(context.Background(), simpleState{})
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
	if !containsStr(err.Error(), "unknown node") {
		t.Errorf("expected 'unknown node' in error, got: %v", err)
	}
}

// ── Invoke: max steps guard ─────────────────────────────────────────────────

func TestInvoke_MaxSteps(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("loop", nodeThatAdds(1))
	g.AddEdge("loop", "loop") // infinite loop
	g.SetEntryPoint("loop")

	compiled := g.Compile()
	_, err := compiled.Invoke(context.Background(), simpleState{})
	if err == nil {
		t.Fatal("expected max steps error")
	}
	if !containsStr(err.Error(), "max steps") {
		t.Errorf("expected 'max steps' in error, got: %v", err)
	}
}

// ── Invoke: checkpoint integration ──────────────────────────────────────────

type memCheckpointer struct {
	states map[string]simpleState
}

func newMemCheckpointer() *memCheckpointer {
	return &memCheckpointer{states: make(map[string]simpleState)}
}

func (m *memCheckpointer) Get(ctx context.Context, threadID string) (simpleState, error) {
	if s, ok := m.states[threadID]; ok {
		return s, nil
	}
	var zero simpleState
	return zero, ErrCheckpointNotFound
}

func (m *memCheckpointer) Put(ctx context.Context, threadID string, state simpleState) error {
	m.states[threadID] = state
	return nil
}

func TestInvoke_CheckpointSaveAndLoad(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatAdds(10))
	g.AddEdge("a", "b")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	cp := newMemCheckpointer()
	compiled.SetCheckpointer(cp)

	_, err := compiled.Invoke(context.Background(), simpleState{}, WithThreadID("t1"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	// Checkpoint should have been saved after each node
	if len(cp.states) != 1 {
		t.Errorf("checkpoints = %d, want 1", len(cp.states))
	}
	if cp.states["t1"].Value != 11 {
		t.Errorf("checkpointed value = %d, want 11", cp.states["t1"].Value)
	}
}

func TestInvoke_CheckpointLoadResumes(t *testing.T) {
	visitCount := 0
	g := NewStateGraph[simpleState]()
	g.AddNode("a", func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		visitCount++
		state.Value += 1
		return state, nil
	})
	g.AddNode("b", nodeThatAdds(100))
	g.AddEdge("a", "b")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	cp := newMemCheckpointer()
	cp.states["t2"] = simpleState{Value: 999} // pre-populated checkpoint
	compiled.SetCheckpointer(cp)

	result, err := compiled.Invoke(context.Background(), simpleState{}, WithThreadID("t2"))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if visitCount != 1 {
		t.Errorf("visitCount = %d, want 1 (checkpoint was loaded)", visitCount)
	}
	if result.Value != 1100 {
		t.Errorf("Value = %d, want 1100 (999 + 1 + 100)", result.Value)
	}
}

func TestInvoke_CheckpointSkipLoad(t *testing.T) {
	visitCount := 0
	g := NewStateGraph[simpleState]()
	g.AddNode("a", func(ctx context.Context, state simpleState, cfg *RunConfig) (simpleState, error) {
		visitCount++
		state.Value += 1
		return state, nil
	})
	g.SetEntryPoint("a")

	compiled := g.Compile()
	cp := newMemCheckpointer()
	cp.states["t3"] = simpleState{Value: 500}
	compiled.SetCheckpointer(cp)

	result, err := compiled.Invoke(context.Background(), simpleState{Value: 0}, WithThreadID("t3"), WithSkipCheckpointLoad())
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if visitCount != 1 {
		t.Errorf("visitCount = %d, want 1", visitCount)
	}
	if result.Value != 1 {
		t.Errorf("Value = %d, want 1 (skip load should ignore checkpoint)", result.Value)
	}
}

// ── Invoke: callbacks ───────────────────────────────────────────────────────

type recordingCallback struct {
	starts []string
	ends   []string
}

func (r *recordingCallback) OnNodeStart(ctx context.Context, nodeID string, state any) {
	r.starts = append(r.starts, nodeID)
}

func (r *recordingCallback) OnNodeEnd(ctx context.Context, nodeID string, state any, err error) {
	r.ends = append(r.ends, nodeID)
}

func (r *recordingCallback) OnToolStart(ctx context.Context, toolName string, input string)  {}
func (r *recordingCallback) OnToolEnd(ctx context.Context, toolName string, output string, err error) {}
func (r *recordingCallback) OnLLMStart(ctx context.Context, model string, messages []any)    {}
func (r *recordingCallback) OnLLMEnd(ctx context.Context, model string, output string, usage any) {}

func TestInvoke_Callbacks(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatAdds(10))
	g.AddEdge("a", "b")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	cb := &recordingCallback{}
	_, err := compiled.Invoke(context.Background(), simpleState{}, WithCallbacks(cb))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	wantStarts := []string{"a", "b"}
	wantEnds := []string{"a", "b"}
	if !sliceEq(cb.starts, wantStarts) {
		t.Errorf("starts = %v, want %v", cb.starts, wantStarts)
	}
	if !sliceEq(cb.ends, wantEnds) {
		t.Errorf("ends = %v, want %v", cb.ends, wantEnds)
	}
}

func TestInvoke_CallbackOnError(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("fail", nodeThatErrors("boom"))
	g.AddEdge("a", "fail")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	cb := &recordingCallback{}
	_, err := compiled.Invoke(context.Background(), simpleState{}, WithCallbacks(cb))
	if err == nil {
		t.Fatal("expected error")
	}

	// Should have start+end for "a", and end for "fail" (with error)
	if len(cb.starts) != 2 || cb.starts[0] != "a" || cb.starts[1] != "fail" {
		t.Errorf("starts = %v, want [a fail]", cb.starts)
	}
	if len(cb.ends) != 2 || cb.ends[0] != "a" || cb.ends[1] != "fail" {
		t.Errorf("ends = %v, want [a fail]", cb.ends)
	}
}

// ── InvokeParallel ──────────────────────────────────────────────────────────

func TestInvokeParallel_LinearDAG(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatAdds(10))
	g.AddNode("c", nodeThatAdds(100))
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	result, err := compiled.InvokeParallel(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("InvokeParallel: %v", err)
	}

	if result.Value != 111 {
		t.Errorf("Value = %d, want 111", result.Value)
	}
}

func TestInvokeParallel_ConcurrentBatch(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("root", nodeThatAdds(0))
	g.AddNode("left", nodeThatAdds(1))
	g.AddNode("right", nodeThatAdds(10))
	g.AddNode("merge", nodeThatAdds(100))
	g.AddEdge("root", "left")
	g.AddEdge("root", "right")
	g.AddEdge("left", "merge")
	g.AddEdge("right", "merge")
	g.SetEntryPoint("root")

	compiled := g.Compile()
	result, err := compiled.InvokeParallel(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("InvokeParallel: %v", err)
	}

	// runBatch merges concurrent states by overwriting with the last state.
	// So left(+1) and right(+10) run concurrently from 0; the last write
	// wins (10), then merge adds 100 → 110.
	if result.Value != 110 {
		t.Errorf("Value = %d, want 110", result.Value)
	}
}

func TestInvokeParallel_CycleDetection(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatAdds(10))
	g.AddEdge("a", "b")
	g.AddEdge("b", "a") // cycle
	g.SetEntryPoint("a")

	compiled := g.Compile()
	_, err := compiled.InvokeParallel(context.Background(), simpleState{})
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !containsStr(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

func TestInvokeParallel_NodeError(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddNode("b", nodeThatErrors("parallel boom"))
	g.AddEdge("a", "b")
	g.SetEntryPoint("a")

	compiled := g.Compile()
	_, err := compiled.InvokeParallel(context.Background(), simpleState{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsStr(err.Error(), "parallel boom") {
		t.Errorf("expected error to contain 'parallel boom', got: %v", err)
	}
}

// ── topologicalBatches ──────────────────────────────────────────────────────

func TestTopologicalBatches_Simple(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(0))
	g.AddNode("b", nodeThatAdds(0))
	g.AddNode("c", nodeThatAdds(0))
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	batches, err := g.topologicalBatches()
	if err != nil {
		t.Fatalf("topologicalBatches: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	expectBatch(t, batches[0], []string{"a"})
	expectBatch(t, batches[1], []string{"b"})
	expectBatch(t, batches[2], []string{"c"})
}

func TestTopologicalBatches_FanOutFanIn(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("root", nodeThatAdds(0))
	g.AddNode("left", nodeThatAdds(0))
	g.AddNode("right", nodeThatAdds(0))
	g.AddNode("merge", nodeThatAdds(0))
	g.AddEdge("root", "left")
	g.AddEdge("root", "right")
	g.AddEdge("left", "merge")
	g.AddEdge("right", "merge")

	batches, err := g.topologicalBatches()
	if err != nil {
		t.Fatalf("topologicalBatches: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(batches))
	}
	expectBatch(t, batches[0], []string{"root"})
	expectBatch(t, batches[1], []string{"left", "right"})
	expectBatch(t, batches[2], []string{"merge"})
}

func TestTopologicalBatches_Cycle(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(0))
	g.AddNode("b", nodeThatAdds(0))
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")

	_, err := g.topologicalBatches()
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func expectBatch(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("batch size = %d, want %d", len(got), len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("batch[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStateGraph_AddNodeDuplicate(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("x", nodeThatAdds(1))
	g.AddNode("x", nodeThatAdds(10)) // overwrite
	g.SetEntryPoint("x")

	compiled := g.Compile()
	result, err := compiled.Invoke(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Value != 10 {
		t.Errorf("Value = %d, want 10 (duplicate should overwrite)", result.Value)
	}
}

func TestStateGraph_EdgeToEmptyString(t *testing.T) {
	g := NewStateGraph[simpleState]()
	g.AddNode("a", nodeThatAdds(1))
	g.AddEdge("a", "") // exit edge
	g.SetEntryPoint("a")

	compiled := g.Compile()
	result, err := compiled.Invoke(context.Background(), simpleState{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Value != 1 {
		t.Errorf("Value = %d, want 1", result.Value)
	}
}
