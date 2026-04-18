// Package store's hybrid.go implements HybridStore — concurrent vector + keyword
// dispatch with result merging, reranking, and MMR diversity.
package store

import (
	"context"
	"sort"
	"sync"

	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/internal/rerank"
	"github.com/amr/naqb/internal/searchutil"
)

const defaultMMRLambda = 0.7

// HybridStoreImpl combines a VectorStore and KeywordStore with a Reranker.
type HybridStoreImpl struct {
	vector   VectorStore
	keyword  KeywordStore
	reranker rerank.Reranker
}

// NewHybridStore creates a HybridStoreImpl. If reranker is nil, NullReranker is used.
func NewHybridStore(vector VectorStore, keyword KeywordStore, reranker rerank.Reranker) *HybridStoreImpl {
	if reranker == nil {
		reranker = rerank.NullReranker{}
	}
	return &HybridStoreImpl{
		vector:   vector,
		keyword:  keyword,
		reranker: reranker,
	}
}

// Search dispatches vector and keyword queries concurrently, merges results,
// reranks, and applies MMR for diversity.
func (h *HybridStoreImpl) Search(ctx context.Context, query string, vec []float32, topK int, filter Filter) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}
	fetchK := topK * 2 // fetch more to have room for dedup + MMR

	var (
		vectorResults  []SearchResult
		keywordResults []SearchResult
		vectorErr      error
		keywordErr     error
		wg             sync.WaitGroup
	)

	// Vector search (only if vector is non-nil and non-empty)
	if h.vector != nil && len(vec) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vectorResults, vectorErr = h.vector.Search(ctx, vec, fetchK, filter)
		}()
	}

	// Keyword search (only if keyword store available)
	if h.keyword != nil && query != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keywordResults, keywordErr = h.keyword.Search(ctx, query, fetchK, filter)
		}()
	}

	wg.Wait()

	// Both failed → hard error
	if vectorErr != nil && keywordErr != nil {
		return nil, vectorErr
	}
	// One source failed → log degradation, continue with partial results
	if vectorErr != nil {
		log.Warn("hybrid search: vector backend failed, using keyword only", "err", vectorErr)
	}
	if keywordErr != nil {
		log.Warn("hybrid search: keyword backend failed, using vector only", "err", keywordErr)
	}

	// Merge and dedup
	merged := mergeBySignature(vectorResults, keywordResults)

	if len(merged) == 0 {
		return nil, nil
	}

	// Rerank
	docs := make([]rerank.Document, len(merged))
	for i, r := range merged {
		docs[i] = rerank.Document{
			ID:        r.ID,
			Content:   r.Content,
			BaseScore: float64(r.Score),
			Position:  i,
		}
	}

	ranked, err := h.reranker.Rerank(ctx, query, docs)
	if err != nil {
		// Degraded: sort by base score
		sort.Slice(merged, func(i, j int) bool {
			return merged[i].Score > merged[j].Score
		})
		return applyMMRAndTrim(merged, topK), nil
	}

	// Convert back to SearchResult
	rerankedResults := make([]SearchResult, len(ranked))
	for i, r := range ranked {
		// Find original metadata
		var meta map[string]string
		for _, orig := range merged {
			if orig.ID == r.ID {
				meta = orig.Metadata
				break
			}
		}
		rerankedResults[i] = SearchResult{
			ID:       r.ID,
			Content:  r.Content,
			Score:    float32(r.FinalScore),
			Metadata: meta,
		}
	}

	return applyMMRAndTrim(rerankedResults, topK), nil
}

// mergeBySignature deduplicates two SearchResult slices by ContentSignature.
// When duplicates are found, the result with the highest Score is kept.
func mergeBySignature(a, b []SearchResult) []SearchResult {
	sigIdx := make(map[string]int, len(a)+len(b))
	merged := make([]SearchResult, 0, len(a)+len(b))

	add := func(r SearchResult) {
		sig := r.Signature
		if sig == "" {
			sig = searchutil.ContentSignature(r.Content)
		}
		r.Signature = sig
		if idx, exists := sigIdx[sig]; exists {
			if r.Score > merged[idx].Score {
				merged[idx] = r
			}
		} else {
			sigIdx[sig] = len(merged)
			merged = append(merged, r)
		}
	}

	for _, r := range a {
		add(r)
	}
	for _, r := range b {
		add(r)
	}
	return merged
}

// applyMMR applies Maximal Marginal Relevance with λ=0.7 and trims to k results.
func applyMMRAndTrim(results []SearchResult, k int) []SearchResult {
	results = mmr(results, defaultMMRLambda, k)
	if len(results) > k {
		return results[:k]
	}
	return results
}

// mmr is the inlined MMR implementation to avoid the import cycle.
func mmr(results []SearchResult, lambda float64, k int) []SearchResult {
	if len(results) == 0 {
		return results
	}
	if k <= 0 || k > len(results) {
		k = len(results)
	}

	tokens := make([][]string, len(results))
	for i, r := range results {
		tokens[i] = searchutil.TokenizeContent(r.Content)
	}

	selected := make([]SearchResult, 0, k)
	remaining := make([]int, len(results))
	for i := range remaining {
		remaining[i] = i
	}

	for len(selected) < k && len(remaining) > 0 {
		bestIdx := -1
		bestScore := -1.0

		for _, ri := range remaining {
			relevance := float64(results[ri].Score)
			maxSim := 0.0
			for _, sel := range selected {
				selToks := searchutil.TokenizeContent(sel.Content)
				sim := searchutil.JaccardSimilarity(tokens[ri], selToks)
				if sim > maxSim {
					maxSim = sim
				}
			}
			mmrScore := lambda*relevance - (1-lambda)*maxSim
			if bestIdx == -1 || mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = ri
			}
		}

		if bestIdx == -1 {
			break
		}
		selected = append(selected, results[bestIdx])
		newRemaining := remaining[:0]
		for _, ri := range remaining {
			if ri != bestIdx {
				newRemaining = append(newRemaining, ri)
			}
		}
		remaining = newRemaining
	}
	return selected
}

// Upsert delegates to the vector store.
func (h *HybridStoreImpl) Upsert(ctx context.Context, docs []VectorDoc) error {
	if h.vector == nil {
		return nil
	}
	return h.vector.Upsert(ctx, docs)
}

// Close closes both underlying stores.
func (h *HybridStoreImpl) Close() error {
	var firstErr error
	if h.vector != nil {
		if err := h.vector.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if h.keyword != nil {
		if err := h.keyword.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
