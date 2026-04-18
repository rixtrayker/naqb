package knowledge

import (
	"context"
	"database/sql"

	"github.com/amr/naqb/pkg/runtime"
)

// EpistemicStore implements runtime.EpistemicStore backed by the nqb SQLite schema.
type EpistemicStore struct {
	DB *sql.DB
}

// compile-time check
var _ runtime.EpistemicStore = (*EpistemicStore)(nil)

// NewEpistemicStore creates an EpistemicStore backed by the given DB.
func NewEpistemicStore(db *sql.DB) *EpistemicStore {
	return &EpistemicStore{DB: db}
}

func (s *EpistemicStore) Load(ctx context.Context, bookID string) (runtime.EpistemicState, error) {
	return Load(ctx, s.DB, bookID)
}
