package agents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
)

func TestRunGapAnalysis_OffLevel(t *testing.T) {
	result, err := RunGapAnalysis(context.Background(), nil, "/tmp/test", nil, 1, "off")
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}
	if result.Level != "off" {
		t.Errorf("Level = %q, want off", result.Level)
	}
	if result.HasGaps {
		t.Error("expected HasGaps = false for off level")
	}
}

func TestRunGapAnalysis_NoOutlineFile(t *testing.T) {
	// When outline.md is missing, extractOutlineSection returns a fallback prompt.
	// RunGapAnalysis proceeds with that fallback rather than skipping.
	dir := setupBookDir(t, map[int]string{1: "chapter one content"})
	cfg := &config.BookConfig{
		Title:    "Test Book",
		Chapters: []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	client := &mockLLMProvider{responses: []string{"No gaps found. VERDICT: NO"}}
	result, err := RunGapAnalysis(context.Background(), client, dir, cfg, 1, "standard")
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}

	if result.HasGaps {
		t.Errorf("expected HasGaps = false, got true. Findings: %q", result.Findings)
	}
}

func TestRunGapAnalysis_WithMockLLM(t *testing.T) {
	dir := setupBookDir(t, map[int]string{1: "This chapter covers all planned topics thoroughly."})
	// Write an outline with a matching chapter heading
	outline := "## Chapter 1: Intro\n\n- Topic A\n- Topic B\n"
	if err := os.WriteFile(filepath.Join(dir, "outline.md"), []byte(outline), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:    "Test Book",
		Chapters: []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	client := &mockLLMProvider{responses: []string{"All outline points are fully covered. VERDICT: NO"}}
	result, err := RunGapAnalysis(context.Background(), client, dir, cfg, 1, "standard")
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}

	if result.HasGaps {
		t.Errorf("expected HasGaps = false, got true. Findings: %q", result.Findings)
	}
}

func TestRunGapAnalysis_FindsGaps(t *testing.T) {
	dir := setupBookDir(t, map[int]string{1: "This chapter only covers Topic A."})
	outline := "## Chapter 1: Intro\n\n- Topic A\n- Topic B\n- Topic C\n"
	if err := os.WriteFile(filepath.Join(dir, "outline.md"), []byte(outline), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:    "Test Book",
		Chapters: []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	client := &mockLLMProvider{responses: []string{"Topic B is missing. Topic C is not covered. VERDICT: YES"}}
	result, err := RunGapAnalysis(context.Background(), client, dir, cfg, 1, "standard")
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}

	if !result.HasGaps {
		t.Errorf("expected HasGaps = true, got false. Findings: %q", result.Findings)
	}
}

func TestRunGapAnalysis_LLMError(t *testing.T) {
	dir := setupBookDir(t, map[int]string{1: "chapter one"})
	outline := "## Chapter 1: Intro\n\n- Topic A\n"
	if err := os.WriteFile(filepath.Join(dir, "outline.md"), []byte(outline), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Chapters: []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	client := &mockLLMProvider{err: errors.New("llm unavailable")}
	_, err := RunGapAnalysis(context.Background(), client, dir, cfg, 1, "standard")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
	if !strings.Contains(err.Error(), "gap analysis LLM failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLooksLikeGapHeuristic(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"All outline points are well-covered. VERDICT: NO", false},
		{"Fully covered, no gaps.", false},
		{"Comprehensive coverage.", false},
		{"Topic A is missing from the chapter.", true},
		{"Coverage is superficial.", true},
		{"There is a gap in the explanation.", true},
		{"Section on B was omitted.", true},
		{"The chapter lacks detail on C.", true},
		{"Topic D is absent.", true},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := looksLikeGapHeuristic(tc.input)
			if got != tc.want {
				t.Errorf("looksLikeGapHeuristic(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
