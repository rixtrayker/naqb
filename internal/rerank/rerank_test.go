package rerank

import (
	"context"
	"testing"
)

func TestNullReranker_Empty(t *testing.T) {
	r := NullReranker{}
	ranked, err := r.Rerank(context.Background(), "query", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ranked) != 0 {
		t.Errorf("expected 0 results, got %d", len(ranked))
	}
}

func TestNullReranker_SortsByScore(t *testing.T) {
	r := NullReranker{}
	docs := []Document{
		{ID: "a", Content: "low", BaseScore: 0.2, Position: 0},
		{ID: "b", Content: "high", BaseScore: 0.9, Position: 1},
		{ID: "c", Content: "mid", BaseScore: 0.5, Position: 2},
	}
	ranked, err := r.Rerank(context.Background(), "query", docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ranked) != 3 {
		t.Fatalf("expected 3 results, got %d", len(ranked))
	}
	// First result should have the highest FinalScore
	for i := 1; i < len(ranked); i++ {
		if ranked[i].FinalScore > ranked[i-1].FinalScore {
			t.Errorf("results not sorted: ranked[%d].FinalScore=%f > ranked[%d].FinalScore=%f",
				i, ranked[i].FinalScore, i-1, ranked[i-1].FinalScore)
		}
	}
}

func TestCompositeScore(t *testing.T) {
	// Full model score, zero base, position 0
	s := compositeScore(1.0, 0.0, 0)
	// 0.6*1 + 0.3*0 + 0.1*(1/(1+0)) = 0.6 + 0 + 0.1 = 0.7
	want := 0.7
	if s < want-0.001 || s > want+0.001 {
		t.Errorf("compositeScore(1,0,0) = %f; want ~%f", s, want)
	}

	// Position prior decays
	s0 := compositeScore(0.5, 0.5, 0)
	s5 := compositeScore(0.5, 0.5, 5)
	if s0 <= s5 {
		t.Errorf("position 0 should score higher than position 5: %f vs %f", s0, s5)
	}
}

func TestNormalizeScores_AllSame(t *testing.T) {
	docs := []RankedDoc{
		{FinalScore: 0.5},
		{FinalScore: 0.5},
	}
	result := NormalizeScores(docs)
	for _, d := range result {
		if d.FinalScore != 1.0 {
			t.Errorf("expected 1.0 for all-same scores, got %f", d.FinalScore)
		}
	}
}

func TestNormalizeScores_Range(t *testing.T) {
	docs := []RankedDoc{
		{FinalScore: 0.0},
		{FinalScore: 0.5},
		{FinalScore: 1.0},
	}
	result := NormalizeScores(docs)
	if result[0].FinalScore != 0.0 {
		t.Errorf("min should normalize to 0, got %f", result[0].FinalScore)
	}
	if result[2].FinalScore != 1.0 {
		t.Errorf("max should normalize to 1, got %f", result[2].FinalScore)
	}
}

func TestNullReranker_PreservesContent(t *testing.T) {
	r := NullReranker{}
	docs := []Document{
		{ID: "x", Content: "test content", BaseScore: 0.8, Position: 0},
	}
	ranked, err := r.Rerank(context.Background(), "query", docs)
	if err != nil {
		t.Fatal(err)
	}
	if ranked[0].ID != "x" {
		t.Errorf("expected ID x, got %s", ranked[0].ID)
	}
	if ranked[0].Content != "test content" {
		t.Errorf("content not preserved: %s", ranked[0].Content)
	}
}
