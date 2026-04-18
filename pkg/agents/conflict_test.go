package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// mockLLMProvider implements llm.Provider for testing agents.
type mockLLMProvider struct {
	responses []string
	idx       int
	err       error
}

func (m *mockLLMProvider) Complete(ctx context.Context, model, system string, messages []llm.Message, maxTokens int) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.idx >= len(m.responses) {
		return "", errors.New("no more mock responses")
	}
	r := m.responses[m.idx]
	m.idx++
	return r, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, model, system string, messages []llm.Message, maxTokens int, onDelta llm.StreamFunc) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.idx >= len(m.responses) {
		return "", errors.New("no more mock responses")
	}
	r := m.responses[m.idx]
	m.idx++
	if onDelta != nil {
		onDelta(r)
	}
	return r, nil
}

func setupBookDir(t *testing.T, chapters map[int]string) string {
	dir := t.TempDir()
	chapDir := filepath.Join(dir, "chapters")
	if err := os.MkdirAll(chapDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for num, content := range chapters {
		path := filepath.Join(chapDir, config.ChapterFilename(num))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunConflictCheck_OffLevel(t *testing.T) {
	result, err := RunConflictCheck(context.Background(), nil, "/tmp/test", nil, 1, "off")
	if err != nil {
		t.Fatalf("RunConflictCheck: %v", err)
	}
	if result.Level != "off" {
		t.Errorf("Level = %q, want off", result.Level)
	}
	if result.HasIssues {
		t.Error("expected HasIssues = false for off level")
	}
}

func TestRunConflictCheck_NoPreceding(t *testing.T) {
	// Chapter 1 has no preceding chapters
	dir := setupBookDir(t, map[int]string{1: "chapter one content"})
	cfg := &config.BookConfig{
		Chapters: []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	result, err := RunConflictCheck(context.Background(), nil, dir, cfg, 1, "standard")
	if err != nil {
		t.Fatalf("RunConflictCheck: %v", err)
	}

	if !strings.Contains(result.Findings, "No previous chapters") {
		t.Errorf("expected 'No previous chapters' in findings, got: %q", result.Findings)
	}
	if result.HasIssues {
		t.Error("expected HasIssues = false when no preceding chapters")
	}
}

func TestRunConflictCheck_WithMockLLM(t *testing.T) {
	dir := setupBookDir(t, map[int]string{
		1: "The protagonist was born in Cairo.",
		2: "The protagonist grew up in Alexandria.",
	})
	cfg := &config.BookConfig{
		Title:    "Test Book",
		Chapters: []config.Chapter{{Number: 1, Title: "Birth"}, {Number: 2, Title: "Childhood"}},
	}

	client := &mockLLMProvider{responses: []string{"No conflicts found. VERDICT: NO"}}
	result, err := RunConflictCheck(context.Background(), client, dir, cfg, 2, "standard")
	if err != nil {
		t.Fatalf("RunConflictCheck: %v", err)
	}

	if result.HasIssues {
		t.Errorf("expected HasIssues = false, got true. Findings: %q", result.Findings)
	}
}

func TestRunConflictCheck_FindsIssues(t *testing.T) {
	dir := setupBookDir(t, map[int]string{
		1: "The protagonist was born in Cairo.",
		2: "The protagonist was born in Alexandria.",
	})
	cfg := &config.BookConfig{
		Title:    "Test Book",
		Chapters: []config.Chapter{{Number: 1, Title: "Birth"}, {Number: 2, Title: "Details"}},
	}

	client := &mockLLMProvider{responses: []string{"Contradiction: birthplace mismatch. VERDICT: YES"}}
	result, err := RunConflictCheck(context.Background(), client, dir, cfg, 2, "standard")
	if err != nil {
		t.Fatalf("RunConflictCheck: %v", err)
	}

	if !result.HasIssues {
		t.Errorf("expected HasIssues = true, got false. Findings: %q", result.Findings)
	}
}

func TestRunConflictCheck_LLMError(t *testing.T) {
	dir := setupBookDir(t, map[int]string{
		1: "chapter one",
		2: "chapter two",
	})
	cfg := &config.BookConfig{
		Chapters: []config.Chapter{{Number: 1, Title: "One"}, {Number: 2, Title: "Two"}},
	}

	client := &mockLLMProvider{err: errors.New("llm unavailable")}
	_, err := RunConflictCheck(context.Background(), client, dir, cfg, 2, "standard")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
	if !strings.Contains(err.Error(), "conflict check LLM failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLooksLikeConflictHeuristic(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"No conflicts found. VERDICT: NO", false},
		{"No contradiction detected.", false},
		{"Everything is consistent.", false},
		{"Contradiction in chapter 3.", true},
		{"Factual conflict found.", true},
		{"Inconsistency between sources.", true},
		{"The dates disagree.", true},
		{"Character description mismatch.", true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := looksLikeConflictHeuristic(tc.input)
			if got != tc.want {
				t.Errorf("looksLikeConflictHeuristic(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
