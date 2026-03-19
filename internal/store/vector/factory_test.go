package vector

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amr/naqb/internal/store"
)

// stubEmbedder implements embedding.Embedder for tests.
type stubEmbedder struct{ dim int }

func (s stubEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) { return nil, nil }
func (s stubEmbedder) Dimensions() int                                          { return s.dim }

func TestNewVectorStoreWithEmbedder_DimensionMismatch(t *testing.T) {
	cfg := VectorConfig{Driver: "chroma", Dimensions: 1024}
	emb := stubEmbedder{dim: 768}
	_, err := NewVectorStoreWithEmbedder(cfg, emb)
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
	if !strings.Contains(err.Error(), "dimension") {
		t.Errorf("error should mention dimension, got: %v", err)
	}
}

func TestNewVectorStoreWithEmbedder_Match(t *testing.T) {
	cfg := VectorConfig{Driver: "chroma", Dimensions: 1024}
	emb := stubEmbedder{dim: 1024}
	s, err := NewVectorStoreWithEmbedder(cfg, emb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewVectorStoreWithEmbedder_ZeroConfig(t *testing.T) {
	cfg := VectorConfig{Driver: "chroma", Dimensions: 0}
	emb := stubEmbedder{dim: 1024}
	s, err := NewVectorStoreWithEmbedder(cfg, emb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewLanceDB_ReturnsError(t *testing.T) {
	_, err := NewLanceDB(VectorConfig{})
	if !errors.Is(err, store.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

func TestNewZilliz_ReturnsError(t *testing.T) {
	_, err := NewZilliz(VectorConfig{})
	if !errors.Is(err, store.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got: %v", err)
	}
}

// Group 7: Chroma dimension validation
func TestChromaUpsert_DimensionMismatch(t *testing.T) {
	// Create a ChromaStore with dim=1024 directly (no HTTP calls needed for validation)
	s := &ChromaStore{
		host:       "http://localhost:1", // will never be called
		collection: "test",
		dimensions: 1024,
		collID:     "fake-id", // pre-set to skip ensureCollection
	}

	docs := []store.VectorDoc{
		{
			ID:      "doc1",
			Content: "test content",
			Vector:  make([]float32, 768), // wrong dimension
		},
	}
	err := s.Upsert(context.Background(), docs)
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
	if !strings.Contains(err.Error(), "dimension") {
		t.Errorf("error should mention dimension, got: %v", err)
	}
}
