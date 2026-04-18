// Package searchutil provides pure utility functions for text search: normalization,
// tokenization, content signatures, and similarity scoring. Vendored from WeKnora
// (Tencent, MIT) with Arabic-aware extensions.
package searchutil

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeContent normalizes whitespace in text: trims leading/trailing space,
// collapses internal runs of whitespace to a single space, and applies Unicode
// NFC normalization. Safe for Arabic and mixed-script content.
func NormalizeContent(text string) string {
	// NFC normalization first (canonical decomposition then canonical composition)
	text = norm.NFC.String(text)

	// Collapse whitespace
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// TokenizeContent splits text into lowercase tokens, filtering out punctuation
// and short tokens (len < 2). Safe for Arabic — splits on Unicode word boundaries
// (spaces and punctuation).
func TokenizeContent(text string) []string {
	text = NormalizeContent(text)
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r) || unicode.IsSymbol(r)
	})

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		// Lowercase ASCII; Arabic letters are unchanged by ToLower
		t := strings.ToLower(p)
		if len([]rune(t)) >= 2 {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// stripTashkeel removes Arabic diacritical marks (tashkeel) from a string.
// This allows deduplication of text that differs only in vocalization.
// Tashkeel occupies Unicode block U+064B–U+065F.
func stripTashkeel(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r >= 0x064B && r <= 0x065F {
			continue // skip tashkeel
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ContentSignature returns a deterministic SHA-256 hex digest of the content
// after normalization and tashkeel stripping. Two texts that differ only in
// Arabic vocalization (tashkeel) will produce the same signature.
func ContentSignature(text string) string {
	normalized := NormalizeContent(stripTashkeel(text))
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h[:16]) // first 128 bits is enough for dedup
}

// JaccardSimilarity computes the Jaccard index between two token sets:
//
//	|A ∩ B| / |A ∪ B|
//
// Returns 1.0 when both sets are empty (identity property).
func JaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	setA := make(map[string]struct{}, len(a))
	for _, t := range a {
		setA[t] = struct{}{}
	}

	intersection := 0
	setB := make(map[string]struct{}, len(b))
	for _, t := range b {
		if _, ok := setA[t]; ok {
			intersection++
		}
		setB[t] = struct{}{}
	}

	union := len(setA)
	for t := range setB {
		if _, ok := setA[t]; !ok {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
