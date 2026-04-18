package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/runtime"
)

func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b", "c"}, "b") {
		t.Error("expected containsString to find b")
	}
	if containsString([]string{"a", "b", "c"}, "z") {
		t.Error("expected containsString to not find z")
	}
	if containsString([]string{}, "a") {
		t.Error("expected containsString to not find a in empty slice")
	}
}

func TestPipelineState_ToInput(t *testing.T) {
	state := PipelineState{
		BookDir:    "/books/test",
		Cfg:        &config.BookConfig{Title: "Test"},
		ChapterNum: 3,
		JobID:      "job-123",
		GatesPassed: []string{"context"},
	}

	in := state.toInput()

	if in.BookDir != "/books/test" {
		t.Errorf("BookDir = %q, want /books/test", in.BookDir)
	}
	if in.ChapterNum != 3 {
		t.Errorf("ChapterNum = %d, want 3", in.ChapterNum)
	}
	if in.JobID != "job-123" {
		t.Errorf("JobID = %q, want job-123", in.JobID)
	}
	if len(in.GatesPassed) != 1 || in.GatesPassed[0] != "context" {
		t.Errorf("GatesPassed = %v, want [context]", in.GatesPassed)
	}
}

// mockStage is a test stage that records executions.
type mockStage struct {
	name       string
	gate       GateType
	commitMsg  string
	outputMsg  string
	outputErr  error
	executions int
}

func (m *mockStage) Name() string                  { return m.name }
func (m *mockStage) CommitMessage(n int) string    { return m.commitMsg }
func (m *mockStage) Gate() GateType                { return m.gate }
func (m *mockStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	m.executions++
	return StageOutput{Message: m.outputMsg}, m.outputErr
}

func TestStageNode_Run(t *testing.T) {
	stage := &mockStage{name: "test", outputMsg: "done"}
	node := stageNode(stage, 0, 1)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  make(map[string]bool),
	}

	result, err := node(context.Background(), state, &runtime.RunConfig{})
	if err != nil {
		t.Fatalf("node: %v", err)
	}

	if stage.executions != 1 {
		t.Errorf("executions = %d, want 1", stage.executions)
	}
	if len(result.Stages) != 1 {
		t.Fatalf("stages = %d, want 1", len(result.Stages))
	}
	if result.Stages[0].Message != "done" {
		t.Errorf("message = %q, want done", result.Stages[0].Message)
	}
	if !result.Completed["test"] {
		t.Error("expected stage to be marked completed")
	}
}

func TestStageNode_SkipCompleted(t *testing.T) {
	stage := &mockStage{name: "test", outputMsg: "done"}
	node := stageNode(stage, 0, 1)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  map[string]bool{"test": true},
	}

	result, err := node(context.Background(), state, &runtime.RunConfig{})
	if err != nil {
		t.Fatalf("node: %v", err)
	}

	if stage.executions != 0 {
		t.Errorf("executions = %d, want 0 (skipped)", stage.executions)
	}
	if len(result.Stages) != 0 {
		t.Errorf("stages = %d, want 0", len(result.Stages))
	}
}

func TestStageNode_GateAlwaysBlocks(t *testing.T) {
	stage := &mockStage{name: "review", gate: GateAlways}
	node := stageNode(stage, 0, 1)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  make(map[string]bool),
	}

	_, err := node(context.Background(), state, &runtime.RunConfig{})
	if err == nil {
		t.Fatal("expected gate to block execution")
	}

	interrupted, ok := runtime.IsInterrupted(err)
	if !ok {
		t.Fatalf("expected InterruptedError, got: %T", err)
	}
	if interrupted.NodeID != "review" {
		t.Errorf("NodeID = %q, want review", interrupted.NodeID)
	}
}

func TestStageNode_GateAlwaysPasses(t *testing.T) {
	stage := &mockStage{name: "review", gate: GateAlways, outputMsg: "ok"}
	node := stageNode(stage, 0, 1)

	var out strings.Builder
	state := PipelineState{
		BookDir:     t.TempDir(),
		ChapterNum:  1,
		Out:         &out,
		Completed:   make(map[string]bool),
		GatesPassed: []string{"review"},
	}

	result, err := node(context.Background(), state, &runtime.RunConfig{})
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	if stage.executions != 1 {
		t.Errorf("executions = %d, want 1", stage.executions)
	}
	if len(result.Stages) != 1 {
		t.Fatalf("stages = %d, want 1", len(result.Stages))
	}
}

func TestStageNode_GateAutoTriggered(t *testing.T) {
	stage := &mockStage{name: "fix", gate: GateAuto, outputMsg: "fixed"}
	node := stageNode(stage, 1, 2)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  make(map[string]bool),
		Stages: []StageOutput{
			{Message: "issues found in QA"},
		},
	}

	_, err := node(context.Background(), state, &runtime.RunConfig{})
	if err == nil {
		t.Fatal("expected auto-gate to trigger")
	}

	interrupted, ok := runtime.IsInterrupted(err)
	if !ok {
		t.Fatalf("expected InterruptedError, got: %T", err)
	}
	if !strings.Contains(interrupted.Reason, "auto-gate triggered") {
		t.Errorf("Reason = %q, expected auto-gate triggered", interrupted.Reason)
	}
}

func TestStageNode_GateAutoPasses(t *testing.T) {
	stage := &mockStage{name: "fix", gate: GateAuto, outputMsg: "fixed"}
	node := stageNode(stage, 1, 2)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  make(map[string]bool),
		Stages: []StageOutput{
			{Message: "all good"},
		},
	}

	_, err := node(context.Background(), state, &runtime.RunConfig{})
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	if stage.executions != 1 {
		t.Errorf("executions = %d, want 1", stage.executions)
	}
}

func TestStageNode_RunError(t *testing.T) {
	stage := &mockStage{name: "fail", outputErr: errors.New("stage failed")}
	node := stageNode(stage, 0, 1)

	var out strings.Builder
	state := PipelineState{
		BookDir:    t.TempDir(),
		ChapterNum: 1,
		Out:        &out,
		Completed:  make(map[string]bool),
	}

	_, err := node(context.Background(), state, &runtime.RunConfig{})
	if err == nil {
		t.Fatal("expected error from failing stage")
	}
	if !strings.Contains(err.Error(), "stage failed") {
		t.Errorf("error = %v, expected 'stage failed'", err)
	}
}
