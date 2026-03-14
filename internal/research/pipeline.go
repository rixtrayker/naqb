package research

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
	"github.com/amr/naqb/internal/search"
)

// RunResult summarises what the pipeline produced.
type RunResult struct {
	Queries int
	Results int
	Notes   []Note
}

// Run executes the full Scout → Explorer → Scribe pipeline for one chapter.
// Progress lines are written to out.
func Run(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, rules *config.Rules, out io.Writer) (*RunResult, error) {
	log.Info("research pipeline start", "chapter", chapterNum)

	// 1. Build searcher
	provider := ""
	if rules != nil {
		provider = rules.Research.SearchProvider
	}
	searcher, err := NewSearcher(provider)
	if err != nil {
		return nil, fmt.Errorf("search backend: %w", err)
	}
	fmt.Fprintf(out, "  [1/3] Scout — generating queries (provider: %s)...\n", searcher.Name())

	// 2. Scout: generate queries
	maxQ := 5
	if rules != nil && rules.Research.MaxQueriesPerChapter > 0 {
		maxQ = rules.Research.MaxQueriesPerChapter
	}
	queries, err := Scout(ctx, client, cfg, chapterNum, maxQ)
	if err != nil {
		return nil, fmt.Errorf("scout failed: %w", err)
	}
	fmt.Fprintf(out, "        %d queries generated\n", len(queries))
	for _, q := range queries {
		fmt.Fprintf(out, "        • %s\n", q)
	}

	if searcher.Name() == "none" || len(queries) == 0 {
		fmt.Fprintf(out, "  [2/3] Explorer — skipped (no search provider configured)\n")
		fmt.Fprintf(out, "  [3/3] Scribe   — skipped (no results to synthesise)\n")
		log.Info("research pipeline done (no-op)", "chapter", chapterNum)
		return &RunResult{Queries: len(queries)}, nil
	}

	// 3. Explorer: fetch results
	fmt.Fprintf(out, "  [2/3] Explorer — fetching search results...\n")
	maxR := 3
	if rules != nil && rules.Research.MaxResultsPerQuery > 0 {
		maxR = rules.Research.MaxResultsPerQuery
	}
	raw, err := Explorer(ctx, searcher, queries, maxR)
	if err != nil {
		return nil, fmt.Errorf("explorer failed: %w", err)
	}
	total := countResults(raw)
	fmt.Fprintf(out, "        %d results fetched across %d queries\n", total, len(raw))

	// 4. Scribe: synthesise notes
	fmt.Fprintf(out, "  [3/3] Scribe — synthesising atomic notes...\n")
	notes, err := Scribe(ctx, client, cfg, chapterNum, raw, bookDir)
	if err != nil {
		return nil, fmt.Errorf("scribe failed: %w", err)
	}
	fmt.Fprintf(out, "        %d notes saved to .naqb/research/\n", len(notes))

	// 5. Index new notes into the vector store (best-effort).
	indexResearchNotes(ctx, bookDir, notes)

	log.Info("research pipeline done", "chapter", chapterNum, "notes", len(notes))
	return &RunResult{Queries: len(queries), Results: total, Notes: notes}, nil
}

// indexResearchNotes adds newly written research notes to the vector store.
// Errors are swallowed — indexing is best-effort and must not block the pipeline.
func indexResearchNotes(ctx context.Context, bookDir string, notes []Note) {
	if len(notes) == 0 {
		return
	}
	store, err := search.Open(bookDir)
	if err != nil {
		log.Warn("vector index: failed to open store", "err", err)
		return
	}
	defer store.Close()

	notesDir := filepath.Join(bookDir, ".naqb", "research")
	for _, note := range notes {
		if note.Filename == "" {
			continue
		}
		if indexErr := store.IndexResearchNote(ctx, bookDir, note.Filename); indexErr != nil {
			log.Warn("vector index: failed to index note", "file", note.Filename, "err", indexErr)
		}
	}
	_ = notesDir
	log.Info("vector index: research notes indexed", "count", len(notes))
}
