package pipeline

import (
	"testing"
)

func TestStageRegistry_RegisterAndResolve(t *testing.T) {
	r := NewStageRegistry()
	var created bool
	r.Register(StageTypeContext, func() Stage {
		created = true
		return ContextStage{}
	})

	stage, ok := r.Resolve(StageTypeContext)
	if !ok {
		t.Fatal("expected to resolve CONTEXT stage")
	}
	if !created {
		t.Error("factory was not called")
	}
	if stage.Name() != "context" {
		t.Errorf("Name = %q, want context", stage.Name())
	}
}

func TestStageRegistry_ResolveMissing(t *testing.T) {
	r := NewStageRegistry()
	_, ok := r.Resolve(StageTypeCustom)
	if ok {
		t.Error("expected Resolve to return false for unregistered type")
	}
}

func TestDefaultRegistry_Builtins(t *testing.T) {
	// The default registry is populated in init() with built-in stages.
	for _, stageType := range []StageType{
		StageTypeContext,
		StageTypeWrite,
		StageTypeQA,
		StageTypeConflict,
		StageTypeGap,
	} {
		stage, ok := ResolveStage(stageType)
		if !ok {
			t.Errorf("expected built-in stage %q to be registered", stageType)
			continue
		}
		if stage == nil {
			t.Errorf("resolved stage %q is nil", stageType)
		}
	}
}

func TestResolveStage_Unknown(t *testing.T) {
	_, ok := ResolveStage(StageTypeResearch)
	// RESEARCH is not registered in init()
	if ok {
		t.Error("expected RESEARCH to not be registered")
	}
}
