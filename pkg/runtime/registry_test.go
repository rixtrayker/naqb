package runtime

import (
	"context"
	"testing"
)

type dummyTool struct {
	name string
}

func (d *dummyTool) Name() string        { return d.name }
func (d *dummyTool) Description() string { return "dummy tool" }
func (d *dummyTool) Schema() any         { return nil }
func (d *dummyTool) Invoke(ctx context.Context, input string, opts ...Option) (string, error) {
	return "ok", nil
}

func TestToolRegistry_RegisterAndResolve(t *testing.T) {
	r := NewToolRegistry()
	tool := &dummyTool{name: "test_tool"}
	r.Register(tool)

	resolved, ok := r.Resolve("test_tool")
	if !ok {
		t.Fatal("expected to resolve test_tool")
	}
	if resolved.Name() != "test_tool" {
		t.Errorf("Name = %q, want test_tool", resolved.Name())
	}
}

func TestToolRegistry_ResolveMissing(t *testing.T) {
	r := NewToolRegistry()
	_, ok := r.Resolve("missing")
	if ok {
		t.Error("expected Resolve to return false for missing tool")
	}
}

func TestToolRegistry_List(t *testing.T) {
	r := NewToolRegistry()
	if len(r.List()) != 0 {
		t.Errorf("expected empty list, got %d", len(r.List()))
	}

	r.Register(&dummyTool{name: "a"})
	r.Register(&dummyTool{name: "b"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("expected 2 tools, got %d", len(list))
	}
}

func TestToolRegistry_Overwrite(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&dummyTool{name: "same"})
	r.Register(&dummyTool{name: "same"}) // overwrite

	list := r.List()
	if len(list) != 1 {
		t.Errorf("expected 1 tool after overwrite, got %d", len(list))
	}
}
