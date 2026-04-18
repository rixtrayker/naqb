package util

import (
	"testing"
)

func TestMergeBySignature_Dedup(t *testing.T) {
	a := []Result{
		{ID: "a", Content: "hello world", Score: 0.9},
	}
	b := []Result{
		{ID: "b", Content: "hello world", Score: 0.7}, // same content → dedup
		{ID: "c", Content: "different content", Score: 0.6},
	}
	merged := MergeBySignature(a, b)
	if len(merged) != 2 {
		t.Errorf("expected 2 results after dedup, got %d", len(merged))
	}
	// First should be from a (preferred)
	if merged[0].ID != "a" {
		t.Errorf("expected first result from a, got %s", merged[0].ID)
	}
}

func TestMergeBySignature_NoOverlap(t *testing.T) {
	a := []Result{{ID: "a", Content: "content one", Score: 0.9}}
	b := []Result{{ID: "b", Content: "content two", Score: 0.8}}
	merged := MergeBySignature(a, b)
	if len(merged) != 2 {
		t.Errorf("expected 2 results, got %d", len(merged))
	}
}

func TestMergeBySignature_Empty(t *testing.T) {
	merged := MergeBySignature(nil, nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 results, got %d", len(merged))
	}
}

func TestMergeBySignature_PreexistingSignature(t *testing.T) {
	// Pre-computed signatures should be respected
	a := []Result{
		{ID: "a", Content: "content", Score: 0.9, Signature: "sig1"},
	}
	b := []Result{
		{ID: "b", Content: "different content", Score: 0.8, Signature: "sig1"}, // same sig
	}
	merged := MergeBySignature(a, b)
	if len(merged) != 1 {
		t.Errorf("expected 1 result (same sig dedup), got %d", len(merged))
	}
}
