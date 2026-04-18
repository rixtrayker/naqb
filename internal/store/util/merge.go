package util

import (
	"github.com/amr/naqb/internal/searchutil"
)

// MergeBySignature merges two Result slices, deduplicating by ContentSignature.
// When duplicates are found, the result with the highest Score is kept.
func MergeBySignature(a, b []Result) []Result {
	// sigIdx maps signature → index in merged slice
	sigIdx := make(map[string]int, len(a)+len(b))
	merged := make([]Result, 0, len(a)+len(b))

	add := func(r Result) {
		sig := r.Signature
		if sig == "" {
			sig = searchutil.ContentSignature(r.Content)
		}
		r.Signature = sig
		if idx, exists := sigIdx[sig]; exists {
			// Keep whichever has the higher score
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
