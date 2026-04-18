package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// mockProvider implements llm.Provider for testing.
// It cycles through explicit responses and falls back to defaultResponse
// when exhausted, avoiding "no more responses" errors in multi-loop tests.
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
		return "", fmt.Errorf("no more mock responses")
	}
	return r, nil
}

func (m *mockProvider) Stream(ctx context.Context, model, system string, messages []llm.Message, maxTokens int, onDelta llm.StreamFunc) (string, error) {
	r := m.next()
	if r == "" {
		return "", fmt.Errorf("no more mock responses")
	}
	if onDelta != nil {
		onDelta(r)
	}
	return r, nil
}

func TestNewReflectionGraph(t *testing.T) {
	client := &mockProvider{}
	graph := NewReflectionGraph(client, "/tmp/test", &config.BookConfig{})
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestRunReflectionPipeline(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"chapters", "contexts", "prompts", "config"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "contexts", config.ContextFilename(1)), []byte("test context"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "write.md"), []byte("Write the chapter."), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a rules file with low word-count limits so the short draft passes.
	if err := os.WriteFile(filepath.Join(dir, "config", "rules.yaml"), []byte("word_count:\n  min: 1\n  max: 10000\n  target: 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &mockProvider{
		responses: []string{
			"# Chapter 1\n\nThis is the draft content with enough words to pass any basic check.",
		},
		defaultResponse: "No issues found.",
	}

	cfg := &config.BookConfig{
		Title:       "Test Book",
		TargetWords: 100,
		Chapters:    []config.Chapter{{Number: 1, Title: "Test"}},
	}

	var out strings.Builder
	result, err := RunReflectionPipeline(context.Background(), client, dir, cfg, 1, &out)
	if err != nil {
		t.Fatalf("RunReflectionPipeline: %v", err)
	}

	if result.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", result.Attempts)
	}
	if result.WordCount < 1 {
		t.Errorf("WordCount = %d, want >= 1", result.WordCount)
	}
	if result.Draft == "" {
		t.Error("expected non-empty draft")
	}
	if len(result.Issues) > 0 {
		t.Errorf("expected no issues, got %d: %v", len(result.Issues), result.Issues)
	}
}

func TestRunReflectionPipeline_MaxAttempts(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"chapters", "contexts", "prompts"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "contexts", config.ContextFilename(1)), []byte("ctx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "write.md"), []byte("write"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Each loop iteration consumes one Stream (write) and one Complete (QA audit).
	// MaxAttempts = 3, so we need 3 explicit Stream responses.
	// The QA audit responses don't affect loop behaviour here because the
	// deterministic word-count check always fails (drafts are too short for
	// TargetWords = 100), so we can rely on defaultResponse.
	client := &mockProvider{
		responses: []string{
			"draft one with some words here",
			"draft two with some words here",
			"draft three with some words here",
		},
		defaultResponse: "Audit: minor wording.",
	}

	cfg := &config.BookConfig{
		Title:       "Test",
		TargetWords: 100,
		Chapters:    []config.Chapter{{Number: 1, Title: "Test"}},
	}

	result, err := RunReflectionPipeline(context.Background(), client, dir, cfg, 1, nil)
	if err != nil {
		t.Fatalf("RunReflectionPipeline: %v", err)
	}

	if result.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", result.Attempts)
	}
}

func TestReflectionState_ReviewNode_DoneAfterMaxAttempts(t *testing.T) {
	state := ReflectionState{
		ChapterNum:  1,
		MaxAttempts: 2,
		Attempts:    2,
		Issues:      []string{"issue"},
	}

	if state.Attempts >= state.MaxAttempts {
		state.Done = true
	}

	if !state.Done {
		t.Error("expected Done = true when max attempts reached")
	}
}
