package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/amr/naqb/internal/chunker"
	"github.com/amr/naqb/internal/embedding"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/internal/store"
)

// IngestionConfig holds configuration for the contextual chunk ingestion pipeline.
type IngestionConfig struct {
	// BookID is the stable identifier for the book project.
	BookID string
	// Chapter number being ingested.
	Chapter int
	// Embedder produces vectors for each contextualized chunk.
	Embedder embedding.Embedder
	// VectorStore where embedded chunks are upserted.
	VectorStore store.VectorStore
	// KeywordStore where plain-text chunks are indexed.
	KeywordStore store.KeywordStore
	// ParentSize is the rune size of parent chunks (default: 1024).
	ParentSize int
	// ChildSize is the rune size of child chunks for embedding (default: 256).
	ChildSize int
	// Overlap is the rune overlap between adjacent chunks (default: 32).
	Overlap int
	// Language tag for the content (default: "ar").
	Language string
}

// IngestionResult summarizes the output of an ingestion run.
type IngestionResult struct {
	ChunksTotal    int
	ChunksEmbedded int
	ChunksIndexed  int
}

// IngestDocument runs the contextual chunking pipeline on a document:
//  1. Split into parent-child chunk pairs
//  2. Embed each child chunk (using Embedder)
//  3. Upsert into VectorStore (child) and KeywordStore (parent for context)
//
// If Embedder is nil, only keyword indexing is performed.
func IngestDocument(ctx context.Context, text string, cfg IngestionConfig) (*IngestionResult, error) {
	if cfg.ParentSize <= 0 {
		cfg.ParentSize = 1024
	}
	if cfg.ChildSize <= 0 {
		cfg.ChildSize = 256
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 32
	}
	if cfg.Language == "" {
		cfg.Language = "ar"
	}

	pairs := chunker.SplitTextParentChild(text, cfg.ParentSize, cfg.ChildSize, cfg.Overlap)
	result := &IngestionResult{ChunksTotal: len(pairs)}

	searchFrom := 0
	for paraIdx, pair := range pairs {
		// Index parent into keyword store for BM25 retrieval
		if cfg.KeywordStore != nil {
			parentID := fmt.Sprintf("%s-ch%02d-p%04d-parent", cfg.BookID, cfg.Chapter, paraIdx)
			doc := store.KeywordDoc{
				ID:      parentID,
				Content: pair.Parent.Text,
				Fields: map[string]string{
					"book_id":  cfg.BookID,
					"chapter":  fmt.Sprintf("%d", cfg.Chapter),
					"language": cfg.Language,
				},
			}
			if err := cfg.KeywordStore.Index(ctx, doc); err != nil {
				log.Warn("ingestion: keyword index failed", "chunk", parentID, "err", err)
			} else {
				result.ChunksIndexed++
			}
		}

		// Embed and upsert child chunks into vector store
		if cfg.Embedder != nil && cfg.VectorStore != nil {
			for childIdx, child := range pair.Children {
				childID := fmt.Sprintf("%s-ch%02d-p%04d-c%04d", cfg.BookID, cfg.Chapter, paraIdx, childIdx)

				// Contextualize: prepend a brief note about where in the document this chunk is
				var contextNote string
				contextNote, searchFrom = contextualizeChunk(text, child.Text, cfg.Chapter, paraIdx, searchFrom)
				contextualText := contextNote + "\n\n" + child.Text

				vecs, err := cfg.Embedder.Embed(ctx, []string{contextualText})
				if err != nil {
					log.Warn("ingestion: embedding failed", "chunk", childID, "err", err)
					continue
				}
				if len(vecs) == 0 || len(vecs[0]) == 0 {
					continue
				}

				vdoc := store.VectorDoc{
					ID:        childID,
					Vector:    vecs[0],
					Content:   child.Text,
					BookID:    cfg.BookID,
					Chapter:   cfg.Chapter,
					Paragraph: paraIdx,
					Language:  cfg.Language,
					Metadata: map[string]string{
						"book_id":   cfg.BookID,
						"chapter":   fmt.Sprintf("%d", cfg.Chapter),
						"paragraph": fmt.Sprintf("%d", paraIdx),
						"language":  cfg.Language,
					},
				}
				if err := cfg.VectorStore.Upsert(ctx, []store.VectorDoc{vdoc}); err != nil {
					log.Warn("ingestion: vector upsert failed", "chunk", childID, "err", err)
					continue
				}
				result.ChunksEmbedded++
			}
		}
	}

	return result, nil
}

// contextualizeChunk produces a brief contextual prefix for a chunk.
// This implements the Anthropic contextual retrieval pattern.
// searchFrom is the byte offset in fullDoc to start searching from, avoiding
// false matches when the same chunk text appears multiple times in the document.
// Returns the context string and the byte offset where the chunk was found (or searchFrom if not found).
func contextualizeChunk(fullDoc, chunk string, chapter, paragraph, searchFrom int) (string, int) {
	context := fmt.Sprintf("Chapter %d, paragraph %d.", chapter, paragraph+1)

	chunkStart := strings.Index(fullDoc[searchFrom:], chunk)
	if chunkStart >= 0 {
		chunkStart += searchFrom // adjust to absolute position
		// Look for the nearest preceding heading
		preceding := fullDoc[:chunkStart]
		lines := strings.Split(preceding, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "#") {
				context = fmt.Sprintf("Chapter %d, paragraph %d. Section: %s",
					chapter, paragraph+1, strings.TrimLeft(line, "# "))
				break
			}
		}
		return context, chunkStart + len(chunk)
	}

	return context, searchFrom
}

// NewClaimFromText is a helper that creates a Claim with a generated ID.
func NewClaimFromText(bookID string, chapter, paragraph, sentIdx int, text string, ct ClaimType) Claim {
	return Claim{
		ID:          uuid.NewString(),
		BookID:      bookID,
		Chapter:     chapter,
		Paragraph:   paragraph,
		SentenceIdx: sentIdx,
		Text:        text,
		ClaimType:   ct,
		Confidence:  1.0,
		Language:    "ar",
	}
}
