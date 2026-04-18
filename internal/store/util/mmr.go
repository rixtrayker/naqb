// Package util provides result merging and diversity utilities for hybrid search.
// This package defines its own SearchResult to avoid import cycles with the
// parent store package.
package util

import (
	"github.com/amr/naqb/internal/searchutil"
)

// Result is a minimal search result for MMR and merge operations.
// It is structurally compatible with store.SearchResult.
type Result struct {
	ID        string
	Content   string
	Score     float32
	Metadata  map[string]string
	Signature string
}

// ApplyMMR applies Maximal Marginal Relevance to reduce redundancy in results.
// lambda=1.0 → pure relevance, lambda=0.0 → pure diversity.
// Recommended: lambda=0.7 for a relevance-biased diverse set.
func ApplyMMR(results []Result, lambda float64, k int) []Result {
	if len(results) == 0 {
		return results
	}
	if k <= 0 || k > len(results) {
		k = len(results)
	}

	// Pre-tokenize all results
	tokens := make([][]string, len(results))
	for i, r := range results {
		tokens[i] = searchutil.TokenizeContent(r.Content)
	}

	selected := make([]Result, 0, k)
	remaining := make([]int, len(results))
	for i := range remaining {
		remaining[i] = i
	}

	for len(selected) < k && len(remaining) > 0 {
		bestIdx := -1
		bestScore := -1.0

		for _, ri := range remaining {
			relevance := float64(results[ri].Score)

			// Compute max similarity to already-selected docs
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

		// Remove bestIdx from remaining
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
