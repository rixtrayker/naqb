package booktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/research"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/search"
)

// SearchResearchInput is the input schema for the search_research tool.
type SearchResearchInput struct {
	Query string `json:"query" jsonschema:"description=Search query to find relevant research notes"`
	TopK  int    `json:"top_k" jsonschema:"description=Maximum number of results to return (default 5)"`
}

// SearchResearchTool queries the chromem-go research store.
type SearchResearchTool struct {
	BookDir string
}

func NewSearchResearchTool(bookDir string) runtime.Tool { return &SearchResearchTool{BookDir: bookDir} }

func (t *SearchResearchTool) Name() string        { return "search_research" }
func (t *SearchResearchTool) Description() string { return "Search research notes for content relevant to a query. Returns top matching notes with their content." }
func (t *SearchResearchTool) Schema() any         { return nil }

func (t *SearchResearchTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SearchResearchInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Query == "" {
		return "query is required", nil
	}
	topK := args.TopK
	if topK <= 0 {
		topK = 5
	}

	store, err := search.Open(t.BookDir)
	if err != nil {
		return fmt.Sprintf("open search store: %v", err), nil
	}
	defer store.Close()

	results, err := store.QueryResearch(ctx, args.Query, topK)
	if err != nil {
		return fmt.Sprintf("search error: %v", err), nil
	}
	if len(results) == 0 {
		return "no research notes found for query: " + args.Query, nil
	}

	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "--- Result %d (score: %.2f) ---\n%s\n\n", i+1, r.Similarity, r.Content)
	}
	return sb.String(), nil
}

func (t *SearchResearchTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input SearchResearchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

// WebFetchInput is the input schema for the web_fetch tool.
type WebFetchInput struct {
	URL string `json:"url" jsonschema:"description=URL to fetch (converted to clean Markdown via Jina Reader)"`
}

// WebFetchTool fetches a URL via Jina Reader for clean Markdown.
type WebFetchTool struct{}

func NewWebFetchTool() runtime.Tool { return &WebFetchTool{} }

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string { return "Fetch a URL and return its content as clean Markdown (via Jina Reader). Useful for research enrichment." }
func (t *WebFetchTool) Schema() any         { return nil }

func (t *WebFetchTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args WebFetchInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.URL == "" {
		return "url is required", nil
	}
	content, err := research.FetchPage(ctx, args.URL, 50000)
	if err != nil {
		return fmt.Sprintf("fetch error: %v", err), nil
	}
	return content, nil
}

func (t *WebFetchTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input WebFetchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

// GrepChunksInput is the input schema for the grep_chunks tool.
type GrepChunksInput struct {
	Pattern string `json:"pattern"           jsonschema:"description=Keyword pattern to search in indexed chunks"`
	BookID  string `json:"book_id,omitempty" jsonschema:"description=Filter results to a specific book ID"`
	Chapter int    `json:"chapter,omitempty" jsonschema:"description=Filter results to a specific chapter number (0 = all)"`
}

// GrepChunksTool performs keyword-only chunk search.
type GrepChunksTool struct {
	BookDir string
}

func NewGrepChunksTool(bookDir string) runtime.Tool { return &GrepChunksTool{BookDir: bookDir} }

func (t *GrepChunksTool) Name() string        { return "grep_chunks" }
func (t *GrepChunksTool) Description() string { return "Search indexed chunks using keyword matching. Useful for exact term lookup across chapters and research notes." }
func (t *GrepChunksTool) Schema() any         { return nil }

func (t *GrepChunksTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args GrepChunksInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Pattern == "" {
		return "pattern is required", nil
	}

	store, err := search.Open(t.BookDir)
	if err != nil {
		return fmt.Sprintf("open search store: %v", err), nil
	}
	defer store.Close()

	results, err := store.QueryResearch(ctx, args.Pattern, 10)
	if err != nil {
		return fmt.Sprintf("grep chunks error: %v", err), nil
	}
	if len(results) == 0 {
		return "no chunks matched: " + args.Pattern, nil
	}

	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "--- Match %d ---\n%s\n\n", i+1, r.Content)
	}
	return sb.String(), nil
}

func (t *GrepChunksTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input GrepChunksInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}
