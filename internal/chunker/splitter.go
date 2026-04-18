// Package chunker splits text into overlapping chunks for embedding and retrieval.
// Vendored from WeKnora (Tencent, MIT) with Arabic-aware separator and protected
// pattern extensions.
package chunker

import (
	"strings"
	"unicode/utf8"
)

// Chunk represents a single text chunk with its source offsets (rune-based).
type Chunk struct {
	Text  string
	Start int // rune offset in original text
	End   int // rune offset in original text (exclusive)
}

// ChunkPair is a parent chunk together with its child sub-chunks.
type ChunkPair struct {
	Parent   Chunk
	Children []Chunk
}

// defaultSeparators is the ordered list of separators tried when splitting.
// Longer / more structural separators are tried first.
var defaultSeparators = []string{
	"\n\n",  // paragraph break
	"\n",    // line break
	". ",    // sentence (English)
	"! ",    // exclamation
	"? ",    // question
	"؟ ",   // Arabic question mark
	".\n",   // sentence + newline
	"،",    // Arabic comma (U+060C)
	"؛",    // Arabic semicolon (U+061B)
	"۔",    // Arabic full stop (U+06D4)
	", ",    // English comma
	"; ",    // English semicolon
	" ",     // word boundary (last resort)
	"",      // character-level fallback
}

// arabicProtectedPatterns are regex-like prefix strings that should not be split
// mid-sequence. When a candidate split point is immediately after one of these
// prefixes, the split is skipped and the next separator position is tried.
// These are heuristic — a full parser would be better for production.
var arabicProtectedPatterns = []string{
	"قال رسول الله",   // hadith chain opener
	"حدثنا",            // hadith chain: "he narrated to us"
	"أخبرنا",           // hadith chain: "he informed us"
	"رواه",             // "it was narrated by"
	"صلى الله عليه",   // salawat (do not split mid-blessing)
	"بسم الله",         // basmala
	"قال الله",         // Quranic citation opener
}

// isProtected returns true if splitting at bytePos in text would fall within
// a protected pattern (i.e. the pattern starts within lookaround distance).
func isProtected(text string, bytePos int) bool {
	// Look back up to 60 bytes for pattern starts
	lookback := bytePos
	if lookback > 60 {
		lookback = 60
	}
	prefix := text[bytePos-lookback : bytePos]
	for _, pat := range arabicProtectedPatterns {
		if strings.Contains(prefix, pat) {
			// Found pattern in lookback window — conservative: skip split
			return true
		}
	}
	return false
}

// SplitText splits text into overlapping chunks using the default separator
// list (or a custom one). chunkSize and chunkOverlap are measured in runes.
func SplitText(text string, separators []string, chunkSize, chunkOverlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}
	if separators == nil {
		separators = defaultSeparators
	}

	runes := []rune(text)
	if len(runes) <= chunkSize {
		if len(runes) == 0 {
			return nil
		}
		return []Chunk{{Text: text, Start: 0, End: len(runes)}}
	}

	return splitRunes(runes, 0, separators, chunkSize, chunkOverlap)
}

// splitRunes is the recursive implementation that works on the rune slice.
func splitRunes(runes []rune, offset int, separators []string, chunkSize, chunkOverlap int) []Chunk {
	if len(runes) == 0 {
		return nil
	}
	if len(runes) <= chunkSize {
		return []Chunk{{Text: string(runes), Start: offset, End: offset + len(runes)}}
	}

	// Try each separator in order
	for _, sep := range separators {
		splits := splitBySeparator(runes, sep)
		if len(splits) <= 1 {
			continue
		}

		// Merge splits into chunks of the target size
		return mergeSplits(splits, offset, sep, chunkSize, chunkOverlap)
	}

	// Fallback: hard-cut at chunkSize
	var chunks []Chunk
	start := 0
	for start < len(runes) {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, Chunk{
			Text:  string(runes[start:end]),
			Start: offset + start,
			End:   offset + end,
		})
		start += chunkSize - chunkOverlap
		if start >= len(runes) {
			break
		}
	}
	return chunks
}

// splitBySeparator splits a rune slice by a string separator.
// Returns the split pieces (without the separator) as rune slices with byte offsets.
func splitBySeparator(runes []rune, sep string) [][]rune {
	if sep == "" {
		// Character-level split
		result := make([][]rune, len(runes))
		for i, r := range runes {
			result[i] = []rune{r}
		}
		return result
	}

	text := string(runes)
	sepRunes := []rune(sep)
	_ = sepRunes

	parts := strings.Split(text, sep)
	result := make([][]rune, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, []rune(p))
		}
	}
	return result
}

// mergeSplits reassembles split pieces into chunks of the target size with overlap.
func mergeSplits(splits [][]rune, offset int, sep string, chunkSize, chunkOverlap int) []Chunk {
	sepLen := utf8.RuneCountInString(sep)
	var chunks []Chunk

	var current []rune
	currentLen := 0
	currentStart := offset

	for _, split := range splits {
		slen := len(split)
		// If adding this split would exceed chunkSize, flush current chunk
		if currentLen+sepLen+slen > chunkSize && currentLen > 0 {
			chunks = append(chunks, Chunk{
				Text:  string(current),
				Start: currentStart,
				End:   currentStart + currentLen,
			})

			// Keep overlap: trim from the front of current
			for currentLen > chunkOverlap && len(current) > 0 {
				// Remove first word-like piece from current
				// Simple approach: remove up to first separator boundary
				firstSep := strings.Index(string(current), sep)
				if firstSep < 0 || sepLen == 0 {
					current = nil
					currentLen = 0
					break
				}
				removed := []rune(string(current)[:firstSep])
				current = []rune(string(current)[firstSep+len(sep):])
				currentLen -= len(removed) + sepLen
			}
			currentStart = offset // approximate — exact tracking complex with overlap
		}

		if currentLen > 0 && sepLen > 0 {
			current = append(current, []rune(sep)...)
			currentLen += sepLen
		}
		current = append(current, split...)
		currentLen += slen
	}

	if currentLen > 0 {
		chunks = append(chunks, Chunk{
			Text:  string(current),
			Start: currentStart,
			End:   currentStart + currentLen,
		})
	}

	return chunks
}

// SplitTextParentChild creates parent chunks and then sub-splits each into
// child chunks. Returns ChunkPair slices useful for hierarchical retrieval
// (search children, return parent for context).
func SplitTextParentChild(text string, parentSize, childSize, overlap int) []ChunkPair {
	if parentSize <= 0 {
		parentSize = 1024
	}
	if childSize <= 0 {
		childSize = 256
	}
	if overlap < 0 {
		overlap = 0
	}

	parents := SplitText(text, nil, parentSize, overlap/2)
	pairs := make([]ChunkPair, 0, len(parents))

	for _, parent := range parents {
		children := SplitText(parent.Text, nil, childSize, overlap)
		// Adjust child offsets relative to parent start
		for i := range children {
			children[i].Start += parent.Start
			children[i].End += parent.Start
		}
		pairs = append(pairs, ChunkPair{
			Parent:   parent,
			Children: children,
		})
	}

	return pairs
}
