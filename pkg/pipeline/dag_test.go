package pipeline

import (
	"testing"
)

func TestNewDAG_Valid(t *testing.T) {
	decls := []StageDecl{
		{ID: "a", Type: StageTypeContext},
		{ID: "b", Type: StageTypeWrite, DependsOn: []string{"a"}},
		{ID: "c", Type: StageTypeQA, DependsOn: []string{"b"}},
	}
	dag, err := NewDAG(decls)
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	if len(dag.Stages()) != 3 {
		t.Errorf("expected 3 stages, got %d", len(dag.Stages()))
	}
}

func TestNewDAG_DuplicateID(t *testing.T) {
	decls := []StageDecl{
		{ID: "a", Type: StageTypeContext},
		{ID: "a", Type: StageTypeWrite},
	}
	_, err := NewDAG(decls)
	if err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestNewDAG_UnknownDependency(t *testing.T) {
	decls := []StageDecl{
		{ID: "a", Type: StageTypeContext, DependsOn: []string{"nonexistent"}},
	}
	_, err := NewDAG(decls)
	if err == nil {
		t.Error("expected error for unknown dependency")
	}
}

func TestNewDAG_Cycle(t *testing.T) {
	decls := []StageDecl{
		{ID: "a", Type: StageTypeContext, DependsOn: []string{"c"}},
		{ID: "b", Type: StageTypeWrite, DependsOn: []string{"a"}},
		{ID: "c", Type: StageTypeQA, DependsOn: []string{"b"}},
	}
	_, err := NewDAG(decls)
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestNewDAG_EmptyID(t *testing.T) {
	decls := []StageDecl{
		{ID: "", Type: StageTypeContext},
	}
	_, err := NewDAG(decls)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestTopologicalOrder_Linear(t *testing.T) {
	decls := []StageDecl{
		{ID: "context", Type: StageTypeContext},
		{ID: "write", Type: StageTypeWrite, DependsOn: []string{"context"}},
		{ID: "qa", Type: StageTypeQA, DependsOn: []string{"write"}},
	}
	dag, err := NewDAG(decls)
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	batches, err := dag.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	if len(batches) != 3 {
		t.Errorf("expected 3 batches for linear chain, got %d", len(batches))
	}
	// First batch must be context (no deps)
	if batches[0][0] != "context" {
		t.Errorf("expected 'context' first, got %q", batches[0][0])
	}
}

func TestTopologicalOrder_Parallel(t *testing.T) {
	// write and conflict can run in parallel after context
	decls := []StageDecl{
		{ID: "context", Type: StageTypeContext},
		{ID: "write", Type: StageTypeWrite, DependsOn: []string{"context"}},
		{ID: "conflict", Type: StageTypeConflict, DependsOn: []string{"context"}},
		{ID: "qa", Type: StageTypeQA, DependsOn: []string{"write", "conflict"}},
	}
	dag, err := NewDAG(decls)
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	batches, err := dag.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	// Expect: [context], [write, conflict], [qa]
	if len(batches) != 3 {
		t.Errorf("expected 3 batches, got %d", len(batches))
	}
	if len(batches[1]) != 2 {
		t.Errorf("expected 2 parallel stages in batch 2, got %d", len(batches[1]))
	}
}

func TestLoadTemplate_Standard(t *testing.T) {
	tmpl, err := LoadTemplate("standard")
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if tmpl.Name != "standard" {
		t.Errorf("expected name 'standard', got %q", tmpl.Name)
	}
	if len(tmpl.Stages) == 0 {
		t.Error("expected stages in standard template")
	}
}

func TestLoadTemplate_NotFound(t *testing.T) {
	_, err := LoadTemplate("nonexistent-template-xyz")
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestContextDebt_Budget(t *testing.T) {
	debt := &ContextDebt{
		TokenBudget: 1000,
		Action:      DebtDegrade,
	}
	v := debt.Record("stage1", 400, 400) // 800 total
	if v != nil {
		t.Error("expected no violation under budget")
	}
	v = debt.Record("stage2", 200, 200) // 1200 total > 1000
	if v == nil {
		t.Error("expected violation when budget exceeded")
	}
	if v.Action != DebtDegrade {
		t.Errorf("expected DEGRADE action, got %v", v.Action)
	}
	if !debt.HasViolations() {
		t.Error("expected HasViolations to be true")
	}
}
