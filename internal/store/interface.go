// Package store defines unified interfaces for vector, keyword, and hybrid
// document stores used throughout نقب's retrieval pipeline.
package store

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by backend stubs that are not yet built.
var ErrNotImplemented = errors.New("store: backend not implemented")

// ── Shared types ──────────────────────────────────────────────────────────────

// VectorDoc is a document stored in the vector index.
type VectorDoc struct {
	// ID is the unique stable identifier (e.g. "chapter-01" or a UUID).
	ID string
	// Vector is the embedding (must match CollectionConfig.Dimensions).
	Vector []float32
	// Content is the raw text stored alongside the vector.
	Content string
	// Metadata holds arbitrary string key-value pairs.
	Metadata map[string]string

	// Structured fields for naqb documents.
	BookID    string
	Chapter   int
	Paragraph int
	ClaimType string
	Language  string
}

// KeywordDoc is a document stored in the keyword (BM25) index.
type KeywordDoc struct {
	// ID matches VectorDoc.ID for dedup across backends.
	ID      string
	Content string
	// Fields are additional text fields for multi-field search (title, heading, etc.).
	Fields map[string]string
}

// SearchResult is a ranked result returned by any store backend.
type SearchResult struct {
	ID         string
	Content    string
	Score      float32
	Metadata   map[string]string
	// Signature is ContentSignature(Content) — used for dedup in HybridStore.
	Signature  string
}

// Filter specifies metadata constraints for search queries.
type Filter struct {
	// Clauses are AND-ed together.
	Clauses []FilterClause
}

// FilterClause is a single metadata equality constraint.
type FilterClause struct {
	Field string
	Value string
}

// CollectionConfig describes a vector collection.
type CollectionConfig struct {
	Name       string
	Dimensions int
	// Distance is the similarity metric: "cosine" (default), "l2", "dot".
	Distance string
}

// ── Store interfaces ──────────────────────────────────────────────────────────

// VectorStore manages a collection of dense vector documents.
type VectorStore interface {
	Upsert(ctx context.Context, docs []VectorDoc) error
	Delete(ctx context.Context, ids []string) error
	Search(ctx context.Context, query []float32, topK int, filter Filter) ([]SearchResult, error)
	SearchByID(ctx context.Context, id string) (*VectorDoc, error)
	CreateCollection(ctx context.Context, cfg CollectionConfig) error
	DropCollection(ctx context.Context, name string) error
	Close() error
}

// KeywordStore manages a BM25/keyword document index.
type KeywordStore interface {
	Index(ctx context.Context, doc KeywordDoc) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string, topK int, filter Filter) ([]SearchResult, error)
	Close() error
}

// HybridStore combines vector and keyword search with reranking and MMR.
type HybridStore interface {
	Search(ctx context.Context, query string, vec []float32, topK int, filter Filter) ([]SearchResult, error)
	Upsert(ctx context.Context, docs []VectorDoc) error
	Close() error
}
