package research

import (
	"context"
	"fmt"

	"github.com/amr/naqb/pkg/log"
)

// RawResult is the collected output for one query.
type RawResult struct {
	Query   string
	Results []SearchResult
}

// Explorer runs each Scout query through the search backend and optionally
// fetches page bodies to enrich snippets.
func Explorer(ctx context.Context, searcher Searcher, queries []string, maxPerQuery int) ([]RawResult, error) {
	log.Info("research explorer start", "provider", searcher.Name(), "queries", len(queries))

	var all []RawResult
	for i, q := range queries {
		log.Debug("explorer query", "n", i+1, "query", q)

		results, err := searcher.Search(ctx, q, maxPerQuery)
		if err != nil {
			log.Warn("explorer search failed", "query", q, "err", err)
			// Soft failure: continue with other queries
			all = append(all, RawResult{Query: q})
			continue
		}

		// Enrich: fetch page body for first result if snippet is short
		for j := range results {
			if len(results[j].Snippet) < 200 && results[j].URL != "" {
				body, fetchErr := FetchPage(ctx, results[j].URL, 4000)
				if fetchErr == nil && body != "" {
					results[j].Body = body
				}
			}
		}

		all = append(all, RawResult{Query: q, Results: results})
		log.Debug("explorer query done", "query", q, "results", len(results))
	}

	log.Info("research explorer done", "total_results", countResults(all))
	return all, nil
}

// FormatRaw formats raw results as plain text for the Scribe LLM prompt.
func FormatRaw(raw []RawResult) string {
	var sb fmt.Stringer
	_ = sb
	var out string
	for _, r := range raw {
		out += fmt.Sprintf("## Query: %s\n\n", r.Query)
		if len(r.Results) == 0 {
			out += "(no results)\n\n"
			continue
		}
		for _, res := range r.Results {
			out += fmt.Sprintf("### %s\n", res.Title)
			if res.URL != "" {
				out += fmt.Sprintf("Source: %s\n", res.URL)
			}
			text := res.Snippet
			if res.Body != "" && len(res.Body) > len(res.Snippet) {
				text = res.Body
			}
			if len(text) > 1500 {
				text = text[:1500] + "…"
			}
			out += text + "\n\n"
		}
	}
	return out
}

func countResults(raw []RawResult) int {
	total := 0
	for _, r := range raw {
		total += len(r.Results)
	}
	return total
}
