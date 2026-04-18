// Package vector provides VectorStore implementations and a factory function.
package vector

import (
	"fmt"

	"github.com/amr/naqb/internal/embedding"
	"github.com/amr/naqb/internal/store"
)

// VectorConfig holds configuration for creating a VectorStore backend.
type VectorConfig struct {
	// Driver selects the backend: "chroma", "lancedb", "zilliz".
	// Defaults to "chroma" if empty.
	Driver string
	// CollectionName is the collection/table name in the backend.
	CollectionName string
	// Dimensions is the vector size (must match the embedder's output).
	Dimensions int
	// Host/URL for the backend server.
	Host string
	// APIKey for cloud backends (Zilliz).
	APIKey string
}

// NewVectorStore creates a VectorStore from a VectorConfig.
func NewVectorStore(cfg VectorConfig) (store.VectorStore, error) {
	if cfg.Driver == "" {
		cfg.Driver = "chroma"
	}
	if cfg.CollectionName == "" {
		cfg.CollectionName = "naqb_chunks"
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = 1024
	}

	switch cfg.Driver {
	case "chroma":
		return NewChroma(cfg)
	case "lancedb":
		return NewLanceDB(cfg)
	case "zilliz":
		return NewZilliz(cfg)
	default:
		return nil, fmt.Errorf("vector store: unknown driver %q (valid: chroma, lancedb, zilliz)", cfg.Driver)
	}
}

// NewVectorStoreWithEmbedder creates a VectorStore whose dimensions are derived
// from the given Embedder. Returns an error if the config specifies a dimension
// that conflicts with the embedder's output size.
func NewVectorStoreWithEmbedder(cfg VectorConfig, emb embedding.Embedder) (store.VectorStore, error) {
	embDim := emb.Dimensions()
	if cfg.Dimensions > 0 && cfg.Dimensions != embDim {
		return nil, fmt.Errorf("vector store: config dimension %d does not match embedder dimension %d", cfg.Dimensions, embDim)
	}
	cfg.Dimensions = embDim
	return NewVectorStore(cfg)
}
