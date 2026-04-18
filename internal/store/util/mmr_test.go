package util

import (
	"testing"
)

func TestApplyMMR_Empty(t *testing.T) {
	result := ApplyMMR(nil, 0.7, 5)
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestApplyMMR_FewerThanK(t *testing.T) {
	results := []Result{
		{ID: "a", Content: "alpha beta gamma", Score: 0.9},
		{ID: "b", Content: "delta epsilon zeta", Score: 0.8},
	}
	got := ApplyMMR(results, 0.7, 5)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestApplyMMR_DiversityBias(t *testing.T) {
	// Two identical docs followed by a different one
	results := []Result{
		{ID: "a", Content: "the quick brown fox", Score: 0.95},
		{ID: "b", Content: "the quick brown fox", Score: 0.90}, // near-duplicate
		{ID: "c", Content: "completely different content about science", Score: 0.70},
	}
	// With lambda=0.3 (diversity bias), c should be preferred over b
	got := ApplyMMR(results, 0.3, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	// First result should always be highest scoring
	if got[0].ID != "a" {
		t.Errorf("expected first result to be 'a', got %s", got[0].ID)
	}
}

func TestApplyMMR_RelevanceBias(t *testing.T) {
	results := []Result{
		{ID: "a", Content: "the quick brown fox", Score: 0.95},
		{ID: "b", Content: "different content entirely about music", Score: 0.60},
		{ID: "c", Content: "another result about animals and nature", Score: 0.50},
	}
	got := ApplyMMR(results, 1.0, 2) // pure relevance
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("highest score should be first with lambda=1: got %s", got[0].ID)
	}
}
