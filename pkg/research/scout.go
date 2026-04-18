package research

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// Scout generates targeted search queries for a chapter using the LLM.
func Scout(ctx context.Context, client llm.Provider, cfg *config.BookConfig, chapterNum int, maxQueries int) ([]string, error) {
	// Find the chapter
	var ch *config.Chapter
	for i := range cfg.Chapters {
		if cfg.Chapters[i].Number == chapterNum {
			ch = &cfg.Chapters[i]
			break
		}
	}
	if ch == nil {
		return nil, fmt.Errorf("chapter %d not found in book.yaml", chapterNum)
	}

	log.Info("research scout start", "chapter", chapterNum, "title", ch.Title)

	system := `You are a research assistant helping an author find source material.
Given a book chapter title, summary, and domain, generate a list of focused search queries
that will surface high-quality references, facts, and examples for this chapter.
Return ONLY the queries, one per line, no numbering, no explanation.`

	userMsg := fmt.Sprintf(`Book: %s
Domain: %s
Language: %s

Chapter %d: %s
Summary: %s

Generate %d targeted search queries to research this chapter.`,
		cfg.Title, cfg.Domain, cfg.Language,
		ch.Number, ch.Title,
		ch.Summary,
		maxQueries)

	model := cfg.LLM.InitModel
	if model == "" {
		model = llm.ModelHaiku // Scout uses fast/cheap model
	}

	resp, err := client.Complete(ctx, model, system, []llm.Message{
		{Role: "user", Content: userMsg},
	}, 512)
	if err != nil {
		return nil, fmt.Errorf("scout LLM failed: %w", err)
	}

	queries := parseQueryList(resp, maxQueries)
	log.Info("research scout done", "chapter", chapterNum, "queries", len(queries))
	return queries, nil
}

// parseQueryList splits an LLM response into individual query strings.
func parseQueryList(resp string, max int) []string {
	var queries []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		// Remove common list prefixes: "1. ", "- ", "* "
		if len(line) > 2 && (line[1] == '.' || line[0] == '-' || line[0] == '*') {
			line = strings.TrimSpace(line[2:])
		}
		if line == "" {
			continue
		}
		queries = append(queries, line)
		if len(queries) >= max {
			break
		}
	}
	return queries
}
