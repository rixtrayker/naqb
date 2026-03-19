package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// makeResearchDir writes .md files into <bookDir>/.naqb/research/.
func makeResearchDir(t *testing.T, bookDir string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(bookDir, ".naqb", "research")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
}

// ── Open ──────────────────────────────────────────────────────────────────────

func TestOpen_CreatesVectorDir(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	store.Close()

	vectorDir := filepath.Join(dir, ".naqb", "vectors")
	if _, err := os.Stat(vectorDir); err != nil {
		t.Errorf("expected vector dir to exist: %v", err)
	}
}

// ── keywordSearch ─────────────────────────────────────────────────────────────

func TestKeywordSearch_FindsMatch(t *testing.T) {
	dir := t.TempDir()
	makeResearchDir(t, dir, map[string]string{
		"note-01.md": "# Neural Networks\n\nDeep learning transforms how we build AI systems.",
		"note-02.md": "# Database Indexing\n\nB-trees and hash indexes speed up queries.",
	})

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	results, err := store.keywordSearch(context.Background(), nil, "neural network deep learning", 5)
	if err != nil {
		t.Fatalf("keywordSearch: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].ID != "note-01.md" {
		t.Errorf("top result: got %q, want note-01.md", results[0].ID)
	}
}

func TestKeywordSearch_NoResults(t *testing.T) {
	dir := t.TempDir()
	makeResearchDir(t, dir, map[string]string{
		"note-01.md": "# Trees\n\nOak, pine, and maple.",
	})

	store, _ := Open(dir)
	defer store.Close()

	results, err := store.keywordSearch(context.Background(), nil, "quantum physics blockchain", 5)
	if err != nil {
		t.Fatalf("keywordSearch: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for unrelated query, got %d", len(results))
	}
}

func TestKeywordSearch_EmptyQuery(t *testing.T) {
	store, _ := Open(t.TempDir())
	defer store.Close()

	results, err := store.keywordSearch(context.Background(), nil, "", 5)
	if err != nil {
		t.Fatalf("keywordSearch empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for empty query, got %d", len(results))
	}
}

func TestKeywordSearch_HeadingScoresHigher(t *testing.T) {
	dir := t.TempDir()
	// note-01 has "cache" only in body; note-02 has it in heading → note-02 wins
	makeResearchDir(t, dir, map[string]string{
		"note-01.md": "# Storage\n\nCache invalidation is hard.",
		"note-02.md": "# Cache Strategies\n\nLRU and LFU are common eviction policies.",
	})

	store, _ := Open(dir)
	defer store.Close()

	results, err := store.keywordSearch(context.Background(), nil, "cache", 5)
	if err != nil {
		t.Fatalf("keywordSearch: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "note-02.md" {
		t.Errorf("expected heading match to rank higher; got %q first", results[0].ID)
	}
}

func TestKeywordSearch_TopKRespected(t *testing.T) {
	dir := t.TempDir()
	// All notes contain "go"
	for i := 1; i <= 5; i++ {
		name := filepath.Base(filepath.Join(dir, "note.md"))
		_ = name
		makeResearchDir(t, dir, map[string]string{
			filepath.Join("note-0" + string(rune('0'+i)) + ".md"): "# Go programming\n\nGo is fast.",
		})
	}

	store, _ := Open(dir)
	defer store.Close()

	results, err := store.keywordSearch(context.Background(), nil, "go programming", 3)
	if err != nil {
		t.Fatalf("keywordSearch: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("topK=3 not respected: got %d results", len(results))
	}
}

func TestKeywordSearch_MissingDirIsNotError(t *testing.T) {
	// bookDir with no research dir at all
	dir := t.TempDir()
	store, _ := Open(dir)
	defer store.Close()

	_, err := store.keywordSearch(context.Background(), nil, "anything", 5)
	if err != nil {
		t.Errorf("missing research dir should not error: %v", err)
	}
}

// ── Query (public facade) ─────────────────────────────────────────────────────

func TestQuery_KeywordFallback(t *testing.T) {
	dir := t.TempDir()
	makeResearchDir(t, dir, map[string]string{
		"golang.md": "# Go Language\n\nGo is a statically typed language.",
	})

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Without an embedder, Query falls back to keyword search.
	if store.HasEmbedder() {
		t.Skip("embedder available — semantic path, skip keyword fallback test")
	}

	results, err := store.Query(context.Background(), collectionResearch, "go language statically", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result via keyword fallback")
	}
}

func TestTokenizeWords(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"test-case_foo", []string{"test", "case", "foo"}},
		{"المحتوى العربي", []string{"المحتوى", "العربي"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := tokenizeWords(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("tokenizeWords(%q) = %v (len %d), want %v (len %d)",
				tc.input, got, len(got), tc.want, len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("tokenizeWords(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestQuery_EmptyCollection_ReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	// Don't create any research files — no data
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("MISTRAL_API_KEY", "")

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	results, err := store.Query(context.Background(), collectionResearch, "anything", 5)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// Keyword fallback on empty dir should return empty (not nil error)
	if results == nil {
		// nil is acceptable — keyword search returns nil for 0 matches from missing dir
		return
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty collection, got %d", len(results))
	}
}

func TestHasEmbedder_FalseWithoutKey(t *testing.T) {
	// Clear any env keys that might be set in the test environment.
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("MISTRAL_API_KEY", "")

	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	if store.HasEmbedder() {
		t.Error("expected no embedder without API keys")
	}
}
