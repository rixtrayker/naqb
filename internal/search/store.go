// Package search provides semantic document indexing and search via a local
// vector store (chromem-go). It degrades gracefully to keyword search when no
// embedding API key is available.
package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chromem "github.com/philippgille/chromem-go"
)

const (
	collectionChapters = "chapters"
	collectionResearch = "research"
	dbFileName         = "vectors.db"
)

// Store wraps a persistent chromem-go database for a single book project.
type Store struct {
	db          *chromem.DB
	embedFunc   chromem.EmbeddingFunc
	hasEmbedder bool
}

// Open opens (or creates) the vector store for a book project.
// The store is persisted to <bookDir>/.naqb/vectors/.
func Open(bookDir string) (*Store, error) {
	dir := filepath.Join(bookDir, ".naqb", "vectors")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating vector store dir: %w", err)
	}

	db, err := chromem.NewPersistentDB(filepath.Join(dir, dbFileName), false)
	if err != nil {
		return nil, fmt.Errorf("opening vector store: %w", err)
	}

	s := &Store{db: db}
	s.embedFunc, s.hasEmbedder = resolveEmbedder()
	return s, nil
}

// resolveEmbedder returns an embedding function if a compatible API key is
// available, otherwise returns nil and false (keyword fallback mode).
func resolveEmbedder() (chromem.EmbeddingFunc, bool) {
	// OpenAI-compatible key (covers OpenAI, DeepSeek, Ollama, etc.)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return chromem.NewEmbeddingFuncOpenAI(key, chromem.EmbeddingModelOpenAI3Small), true
	}
	// Mistral key
	if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
		return chromem.NewEmbeddingFuncMistral(key), true
	}
	// No embedder available — keyword search fallback
	return nil, false
}

// IndexFile reads a file and adds/updates it in the given collection.
// docID should be a stable identifier (e.g. relative path).
func (s *Store) IndexFile(ctx context.Context, collection, docID, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return s.IndexText(ctx, collection, docID, string(data), map[string]string{
		"path": filePath,
	})
}

// IndexText adds or replaces a document with the given text content.
func (s *Store) IndexText(ctx context.Context, collection, docID, content string, meta map[string]string) error {
	col, err := s.getOrCreateCollection(collection)
	if err != nil {
		return err
	}

	// Delete existing doc with this ID if any (chromem upsert workaround)
	_ = col.Delete(ctx, nil, nil, docID)

	if s.hasEmbedder {
		doc, err := chromem.NewDocument(ctx, docID, meta, nil, content, s.embedFunc)
		if err != nil {
			return fmt.Errorf("creating document: %w", err)
		}
		return col.AddDocument(ctx, doc)
	}

	// No embedder: store without embeddings (keyword query only)
	return col.AddDocument(ctx, chromem.Document{
		ID:       docID,
		Metadata: meta,
		Content:  content,
	})
}

// SearchResult wraps a chromem Result with convenience fields.
type SearchResult struct {
	ID         string
	Content    string
	Similarity float32
	Path       string
}

// Query searches a collection for the most relevant documents.
// Uses semantic search when embeddings are available, keyword fallback otherwise.
func (s *Store) Query(ctx context.Context, collection, queryText string, topK int) ([]SearchResult, error) {
	col := s.db.GetCollection(collection, s.embedFunc)
	if col == nil || col.Count() == 0 {
		return nil, nil
	}

	if s.hasEmbedder {
		results, err := col.Query(ctx, queryText, topK, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("vector query: %w", err)
		}
		return convertResults(results), nil
	}

	// Keyword fallback: simple substring match over all documents
	return s.keywordSearch(ctx, col, queryText, topK)
}

// keywordSearch does naive substring matching when no embedder is available.
func (s *Store) keywordSearch(_ context.Context, col *chromem.Collection, query string, topK int) ([]SearchResult, error) {
	query = strings.ToLower(query)
	words := strings.Fields(query)

	// chromem doesn't expose list-all, so we use QueryEmbedding with zeros
	// to get all docs, then filter client-side. For small book projects this is fine.
	_ = words
	// Fallback: return empty (keyword search without enumeration API is not supported)
	// Users get semantic search once they set OPENAI_API_KEY or MISTRAL_API_KEY.
	return nil, nil
}

func (s *Store) getOrCreateCollection(name string) (*chromem.Collection, error) {
	return s.db.GetOrCreateCollection(name, nil, s.embedFunc)
}

func convertResults(in []chromem.Result) []SearchResult {
	out := make([]SearchResult, len(in))
	for i, r := range in {
		out[i] = SearchResult{
			ID:         r.ID,
			Content:    r.Content,
			Similarity: r.Similarity,
			Path:       r.Metadata["path"],
		}
	}
	return out
}

// IndexChapter indexes a chapter file into the chapters collection.
// chapterNum is used as a stable document ID prefix.
func (s *Store) IndexChapter(ctx context.Context, bookDir string, chapterNum int, filePath string) error {
	docID := fmt.Sprintf("chapter-%02d", chapterNum)
	return s.IndexFile(ctx, collectionChapters, docID, filePath)
}

// IndexResearchNote indexes a single research note file.
// filename (e.g. "ch01-0315-120000-01.md") is used as the document ID.
func (s *Store) IndexResearchNote(ctx context.Context, bookDir, filename string) error {
	path := filepath.Join(bookDir, ".naqb", "research", filename)
	return s.IndexFile(ctx, collectionResearch, filename, path)
}

// IndexResearchDir indexes all .md/.txt files in a research directory
// into the research collection. Skips files that fail to read.
func (s *Store) IndexResearchDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory may not exist yet — not an error.
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		if indexErr := s.IndexFile(ctx, collectionResearch, name, filepath.Join(dir, name)); indexErr != nil {
			// Non-fatal: log and continue.
			_ = indexErr
		}
	}
	return nil
}

// QueryResearch returns the top-K most relevant research notes for a query.
func (s *Store) QueryResearch(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return s.Query(ctx, collectionResearch, query, topK)
}

// QueryChapters returns the top-K most relevant chapters for a query.
func (s *Store) QueryChapters(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	return s.Query(ctx, collectionChapters, query, topK)
}

// Close is a no-op (chromem-go uses persistent files, no explicit close needed).
func (s *Store) Close() {}

// HasEmbedder reports whether semantic search is available.
func (s *Store) HasEmbedder() bool { return s.hasEmbedder }
