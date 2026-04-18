package vector

import (
	"context"

	"github.com/amr/naqb/internal/store"
)

// lanceDBStore is a stub — not yet implemented.
// TODO: implement when lancedb-go matures.
type lanceDBStore struct{}

// NewLanceDB returns an error — LanceDB support is not yet implemented.
func NewLanceDB(_ VectorConfig) (store.VectorStore, error) {
	return nil, store.ErrNotImplemented
}

func (s *lanceDBStore) Upsert(_ context.Context, _ []store.VectorDoc) error {
	return store.ErrNotImplemented
}
func (s *lanceDBStore) Delete(_ context.Context, _ []string) error {
	return store.ErrNotImplemented
}
func (s *lanceDBStore) Search(_ context.Context, _ []float32, _ int, _ store.Filter) ([]store.SearchResult, error) {
	return nil, store.ErrNotImplemented
}
func (s *lanceDBStore) SearchByID(_ context.Context, _ string) (*store.VectorDoc, error) {
	return nil, store.ErrNotImplemented
}
func (s *lanceDBStore) CreateCollection(_ context.Context, _ store.CollectionConfig) error {
	return store.ErrNotImplemented
}
func (s *lanceDBStore) DropCollection(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}
func (s *lanceDBStore) Close() error { return nil }
