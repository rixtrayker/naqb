package vector

import (
	"context"

	"github.com/amr/naqb/internal/store"
)

// zillizStore is a stub — not yet implemented.
// TODO: implement with milvus-sdk-go/v2 when Zilliz cloud access is available.
type zillizStore struct{}

// NewZilliz returns an error — Zilliz support is not yet implemented.
func NewZilliz(_ VectorConfig) (store.VectorStore, error) {
	return nil, store.ErrNotImplemented
}

func (s *zillizStore) Upsert(_ context.Context, _ []store.VectorDoc) error {
	return store.ErrNotImplemented
}
func (s *zillizStore) Delete(_ context.Context, _ []string) error {
	return store.ErrNotImplemented
}
func (s *zillizStore) Search(_ context.Context, _ []float32, _ int, _ store.Filter) ([]store.SearchResult, error) {
	return nil, store.ErrNotImplemented
}
func (s *zillizStore) SearchByID(_ context.Context, _ string) (*store.VectorDoc, error) {
	return nil, store.ErrNotImplemented
}
func (s *zillizStore) CreateCollection(_ context.Context, _ store.CollectionConfig) error {
	return store.ErrNotImplemented
}
func (s *zillizStore) DropCollection(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}
func (s *zillizStore) Close() error { return nil }
