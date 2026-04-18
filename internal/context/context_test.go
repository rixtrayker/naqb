package context

import (
	"context"
	"strings"
	"testing"
)

func TestContextStack_LayerAt(t *testing.T) {
	stack := &ContextStack{
		Name: "test",
		Layers: []ContextLayer{
			{Position: 0, Name: "foundational", Content: "base content"},
			{Position: 3, Name: "analytical", Content: "analysis content"},
		},
	}

	l0 := stack.LayerAt(0)
	if l0 == nil || l0.Name != "foundational" {
		t.Errorf("expected foundational layer at position 0, got %v", l0)
	}

	l3 := stack.LayerAt(3)
	if l3 == nil || l3.Name != "analytical" {
		t.Errorf("expected analytical layer at position 3, got %v", l3)
	}

	lNone := stack.LayerAt(99)
	if lNone != nil {
		t.Error("expected nil for missing position")
	}
}

func TestContextStack_PromptContext(t *testing.T) {
	stack := &ContextStack{
		Name: "test",
		Layers: []ContextLayer{
			{Position: 0, Name: "layer1", Content: "content one"},
			{Position: 1, Name: "layer2", Content: ""},       // empty — should be skipped
			{Position: 2, Name: "layer3", Content: "content three"},
		},
	}

	prompt := stack.PromptContext()
	if prompt == "" {
		t.Error("expected non-empty prompt context")
	}
	if contains(prompt, "layer2") {
		t.Error("empty layer should not appear in prompt context")
	}
	if !contains(prompt, "content one") {
		t.Error("expected layer1 content in prompt")
	}
	if !contains(prompt, "content three") {
		t.Error("expected layer3 content in prompt")
	}
}

func TestStackRegistry_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewStackRegistry(dir)
	if err != nil {
		t.Fatalf("NewStackRegistry: %v", err)
	}

	stack := &ContextStack{
		Name:    "test-stack",
		Version: "1.0",
		Layers: []ContextLayer{
			{Position: 0, Name: "base", Description: "base layer", Content: "base content"},
		},
	}

	if err := reg.Save(stack); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := reg.Load("test-stack")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "test-stack" {
		t.Errorf("expected name 'test-stack', got %q", loaded.Name)
	}
	if len(loaded.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(loaded.Layers))
	}
	if loaded.Layers[0].Content != "base content" {
		t.Errorf("content not preserved: %q", loaded.Layers[0].Content)
	}
}

func TestStackRegistry_List(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewStackRegistry(dir)
	if err != nil {
		t.Fatalf("NewStackRegistry: %v", err)
	}

	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		if err := reg.Save(&ContextStack{Name: n}); err != nil {
			t.Fatalf("Save(%s): %v", n, err)
		}
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != len(names) {
		t.Errorf("expected %d stacks, got %d", len(names), len(list))
	}
}

func TestStackRegistry_Delete(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewStackRegistry(dir)
	if err != nil {
		t.Fatalf("NewStackRegistry: %v", err)
	}

	_ = reg.Save(&ContextStack{Name: "to-delete"})
	if err := reg.Delete("to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = reg.Load("to-delete")
	if err == nil {
		t.Error("expected error loading deleted stack")
	}
}

func TestStackRegistry_LoadMissing(t *testing.T) {
	reg, _ := NewStackRegistry(t.TempDir())
	_, err := reg.Load("nonexistent")
	if err == nil {
		t.Error("expected error for missing stack")
	}
}

func TestDefaultStacksDir(t *testing.T) {
	dir := DefaultStacksDir()
	if dir == "" {
		t.Error("expected non-empty default stacks dir")
	}
}

func TestBraidedField_Weave(t *testing.T) {
	field := &BraidedField{
		Strands: []ContextStack{
			{Name: "grammar", Layers: []ContextLayer{{Position: 0, Name: "l1", Content: "grammar rules"}}},
			{Name: "rhetoric", Layers: []ContextLayer{{Position: 0, Name: "l1", Content: "rhetorical analysis"}}},
		},
	}

	points, err := field.Weave(context.Background(), "Test paragraph text.", 0)
	if err != nil {
		t.Fatalf("Weave: %v", err)
	}
	if len(points) == 0 {
		t.Error("expected at least one braid point")
	}
	// Both strands are active, so we expect RESONANCE
	if points[0].Type != BraidResonance {
		t.Errorf("expected RESONANCE, got %v", points[0].Type)
	}
}

func TestBraidedField_Weave_Silence(t *testing.T) {
	field := &BraidedField{
		Strands: []ContextStack{
			{Name: "grammar", Layers: []ContextLayer{{Position: 0, Name: "l1", Content: "grammar content"}}},
			{Name: "empty-strand"}, // no layers — silent
		},
	}

	points, err := field.Weave(context.Background(), "Test paragraph.", 0)
	if err != nil {
		t.Fatalf("Weave: %v", err)
	}
	if len(points) == 0 {
		t.Error("expected at least one braid point (silence)")
	}
	found := false
	for _, pt := range points {
		if pt.Type == BraidSilence {
			found = true
		}
	}
	if !found {
		t.Error("expected a SILENCE braid point for empty strand")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
