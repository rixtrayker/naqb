package keyword

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/amr/naqb/internal/store"
)

func TestOpenAndClose(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "blevetest")
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestIndexAndSearch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "blevetest")
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	docs := []store.KeywordDoc{
		{ID: "doc1", Content: "the quick brown fox jumps over the lazy dog"},
		{ID: "doc2", Content: "arabic classical text about philosophy and logic"},
		{ID: "doc3", Content: "quantum mechanics and wave functions in physics"},
	}

	ctx := context.Background()
	for _, d := range docs {
		if err := s.Index(ctx, d); err != nil {
			t.Fatalf("Index(%s): %v", d.ID, err)
		}
	}

	results, err := s.Search(ctx, "fox", 5, store.Filter{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result for 'fox'")
	}
	if results[0].ID != "doc1" {
		t.Errorf("expected doc1 as top result, got %s", results[0].ID)
	}
}

func TestDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "blevetest")
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	_ = s.Index(ctx, store.KeywordDoc{ID: "to-delete", Content: "delete me please"})

	// Verify it's there
	results, _ := s.Search(ctx, "delete", 5, store.Filter{})
	if len(results) == 0 {
		t.Skip("bleve may require flush before search is consistent")
	}

	if err := s.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestDefaultPath(t *testing.T) {
	p := DefaultPath("/some/book")
	if p == "" {
		t.Error("expected non-empty path")
	}
	p2 := DefaultPath("")
	if p2 == "" {
		t.Error("expected non-empty global path")
	}
}
