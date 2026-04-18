// Package keyword provides a BM25 keyword store backed by Bleve.
// Storage: ~/.naqb/bleve/ (global) or <bookDir>/.naqb/bleve/ (per-project).
package keyword

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/lang/ar"
	"github.com/blevesearch/bleve/v2/mapping"
	bquery "github.com/blevesearch/bleve/v2/search/query"

	"github.com/amr/naqb/internal/store"
)

// BleveStore is a keyword store backed by a Bleve index.
type BleveStore struct {
	index bleve.Index
}

// indexDoc is the struct Bleve indexes.
type indexDoc struct {
	ID      string            `json:"id"`
	Content string            `json:"content"`
	Fields  map[string]string `json:"fields"`
}

// Open opens (or creates) a Bleve index at path.
func Open(path string) (*BleveStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("keyword store: mkdir: %w", err)
	}

	idx, err := bleve.Open(path)
	if err == bleve.ErrorIndexPathDoesNotExist {
		idx, err = bleve.New(path, buildMapping())
	}
	if err != nil {
		return nil, fmt.Errorf("keyword store: open index: %w", err)
	}
	return &BleveStore{index: idx}, nil
}

// buildMapping creates a Bleve index mapping with Arabic analyzer on the
// content field.
func buildMapping() mapping.IndexMapping {
	m := bleve.NewIndexMapping()

	// Arabic-aware document mapping
	arabicField := bleve.NewTextFieldMapping()
	arabicField.Analyzer = ar.AnalyzerName

	// Default content field uses Arabic analyzer
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("content", arabicField)

	m.DefaultMapping = docMapping
	m.DefaultAnalyzer = "standard"
	return m
}

// Index adds or replaces a document in the keyword index.
func (s *BleveStore) Index(_ context.Context, doc store.KeywordDoc) error {
	d := indexDoc{
		ID:      doc.ID,
		Content: doc.Content,
		Fields:  doc.Fields,
	}
	return s.index.Index(doc.ID, d)
}

// Delete removes a document from the index.
func (s *BleveStore) Delete(_ context.Context, id string) error {
	return s.index.Delete(id)
}

// Search performs a BM25 search and returns up to topK results.
// filter clauses are applied as must-match term queries.
func (s *BleveStore) Search(_ context.Context, query string, topK int, filter store.Filter) ([]store.SearchResult, error) {
	if topK <= 0 {
		topK = 10
	}

	// Build query
	var q bquery.Query
	matchQ := bleve.NewMatchQuery(query)
	matchQ.SetField("content")

	if len(filter.Clauses) == 0 {
		q = matchQ
	} else {
		must := make([]bquery.Query, 0, len(filter.Clauses)+1)
		must = append(must, matchQ)
		for _, clause := range filter.Clauses {
			tq := bleve.NewTermQuery(clause.Value)
			tq.SetField(clause.Field)
			must = append(must, tq)
		}
		boolQ := bleve.NewBooleanQuery()
		for _, mq := range must {
			boolQ.AddMust(mq)
		}
		q = boolQ
	}

	req := bleve.NewSearchRequestOptions(q, topK, 0, false)
	req.Fields = []string{"content"}

	searchResult, err := s.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}

	results := make([]store.SearchResult, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		content := ""
		if c, ok := hit.Fields["content"]; ok {
			content = fmt.Sprintf("%v", c)
		}
		results = append(results, store.SearchResult{
			ID:      hit.ID,
			Content: content,
			Score:   float32(hit.Score),
		})
	}
	return results, nil
}

// Close closes the Bleve index.
func (s *BleveStore) Close() error {
	return s.index.Close()
}

// DefaultPath returns the default Bleve index path for a book directory.
// If bookDir is empty, returns the global path under ~/.naqb/.
func DefaultPath(bookDir string) string {
	if bookDir != "" {
		return filepath.Join(bookDir, ".naqb", "bleve")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".naqb", "bleve")
}
