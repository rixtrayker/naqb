// Package rerank provides document reranking for search result refinement.
// Vendored from WeKnora (Tencent, MIT) with composite scoring and NullReranker.
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

// Document is a search result candidate to be reranked.
type Document struct {
	ID      string
	Content string
	// BaseScore is the retrieval score from the vector/keyword store (0–1).
	BaseScore float64
	// Position is the 0-based rank in the original result list (used for position prior).
	Position int
}

// RankedDoc is a document with its final composite score.
type RankedDoc struct {
	Document
	// ModelScore is the reranker model's raw relevance score (0–1).
	ModelScore float64
	// FinalScore is the composite score used for final ranking.
	FinalScore float64
}

// Reranker is the interface implemented by all reranking backends.
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []Document) ([]RankedDoc, error)
}

// ── Composite scoring ─────────────────────────────────────────────────────────

// compositeScore computes the weighted combination:
//
//	score = 0.6 × model_score + 0.3 × base_score + 0.1 × position_prior
//
// position_prior decays as 1/(1+position).
func compositeScore(modelScore, baseScore float64, position int) float64 {
	positionPrior := 1.0 / (1.0 + float64(position))
	return 0.6*modelScore + 0.3*baseScore + 0.1*positionPrior
}

// ── NullReranker ──────────────────────────────────────────────────────────────

// NullReranker is a passthrough that returns docs sorted by BaseScore.
// Use it when no reranker API is configured.
type NullReranker struct{}

func (NullReranker) Rerank(_ context.Context, _ string, docs []Document) ([]RankedDoc, error) {
	ranked := make([]RankedDoc, len(docs))
	for i, d := range docs {
		ranked[i] = RankedDoc{
			Document:   d,
			ModelScore: d.BaseScore,
			FinalScore: compositeScore(d.BaseScore, d.BaseScore, d.Position),
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})
	return ranked, nil
}

// ── Cohere-compatible reranker ────────────────────────────────────────────────

const (
	defaultCohereBase      = "https://api.cohere.ai/v1"
	defaultCohereModel     = "rerank-multilingual-v3.0"
	defaultThreshold       = 0.5
	thresholdDegradeFactor = 0.7
	thresholdFloor         = 0.3
)

// CohereReranker calls the Cohere (or compatible) rerank API.
type CohereReranker struct {
	apiKey    string
	baseURL   string
	model     string
	threshold float64
	client    *http.Client
}

// NewCohere creates a Cohere-compatible reranker.
func NewCohere(apiKey, baseURL, model string, threshold float64) *CohereReranker {
	if baseURL == "" {
		baseURL = defaultCohereBase
	}
	if model == "" {
		model = defaultCohereModel
	}
	if threshold <= 0 {
		threshold = defaultThreshold
	}
	return &CohereReranker{
		apiKey:    apiKey,
		baseURL:   baseURL,
		model:     model,
		threshold: threshold,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *CohereReranker) Rerank(ctx context.Context, query string, docs []Document) ([]RankedDoc, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}

	reqBody := map[string]any{
		"model":     r.model,
		"query":     query,
		"documents": texts,
		"top_n":     len(docs),
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("rerank: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.baseURL+"/rerank", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("rerank: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank: HTTP: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rerank: API error %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("rerank: decode: %w", err)
	}

	threshold := r.threshold
	ranked := make([]RankedDoc, 0, len(result.Results))
	for _, res := range result.Results {
		if res.Index < 0 || res.Index >= len(docs) {
			continue
		}
		doc := docs[res.Index]
		modelScore := res.RelevanceScore
		final := compositeScore(modelScore, doc.BaseScore, doc.Position)
		if final >= threshold {
			ranked = append(ranked, RankedDoc{
				Document:   doc,
				ModelScore: modelScore,
				FinalScore: final,
			})
		}
	}

	// Threshold degradation: if too few results, retry with lower threshold.
	// Reset ranked slice first to prevent any possibility of duplicates.
	if len(ranked) == 0 && threshold > thresholdFloor {
		degraded := threshold * thresholdDegradeFactor
		if degraded < thresholdFloor {
			degraded = thresholdFloor
		}
		ranked = ranked[:0]
		for _, res := range result.Results {
			if res.Index < 0 || res.Index >= len(docs) {
				continue
			}
			doc := docs[res.Index]
			modelScore := res.RelevanceScore
			final := compositeScore(modelScore, doc.BaseScore, doc.Position)
			if final >= degraded {
				ranked = append(ranked, RankedDoc{
					Document:   doc,
					ModelScore: modelScore,
					FinalScore: final,
				})
			}
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].FinalScore > ranked[j].FinalScore
	})
	return ranked, nil
}

// ── Normalize utility ─────────────────────────────────────────────────────────

// NormalizeScores min-max normalizes the FinalScore across a slice of RankedDoc.
func NormalizeScores(docs []RankedDoc) []RankedDoc {
	if len(docs) == 0 {
		return docs
	}
	minS, maxS := math.MaxFloat64, -math.MaxFloat64
	for _, d := range docs {
		if d.FinalScore < minS {
			minS = d.FinalScore
		}
		if d.FinalScore > maxS {
			maxS = d.FinalScore
		}
	}
	if maxS == minS {
		for i := range docs {
			docs[i].FinalScore = 1.0
		}
		return docs
	}
	for i := range docs {
		docs[i].FinalScore = (docs[i].FinalScore - minS) / (maxS - minS)
	}
	return docs
}
