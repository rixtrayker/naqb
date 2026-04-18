package booktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/search"
)

// KnowledgeSearchInput is the input schema for the knowledge_search tool.
type KnowledgeSearchInput struct {
	Query  string `json:"query"  jsonschema:"description=Query to search the knowledge graph and vector index"`
	TopK   int    `json:"top_k,omitempty"  jsonschema:"description=Maximum number of results (default 5)"`
	BookID string `json:"book_id,omitempty" jsonschema:"description=Filter results to a specific book ID"`
}

// KnowledgeSearchTool queries the hybrid knowledge store.
type KnowledgeSearchTool struct {
	BookDir string
}

func NewKnowledgeSearchTool(bookDir string) runtime.Tool { return &KnowledgeSearchTool{BookDir: bookDir} }

func (t *KnowledgeSearchTool) Name() string        { return "knowledge_search" }
func (t *KnowledgeSearchTool) Description() string { return "Search the knowledge graph and indexed chunks for content relevant to a query. Returns top matching results with context." }
func (t *KnowledgeSearchTool) Schema() any         { return nil }

func (t *KnowledgeSearchTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args KnowledgeSearchInput
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

	researchResults, _ := store.QueryResearch(ctx, args.Query, topK)
	chapterResults, _ := store.QueryChapters(ctx, args.Query, topK)

	if len(researchResults) == 0 && len(chapterResults) == 0 {
		return "no results found for: " + args.Query, nil
	}

	var sb strings.Builder
	if len(chapterResults) > 0 {
		fmt.Fprintf(&sb, "=== Chapter Content ===\n")
		for i, r := range chapterResults {
			fmt.Fprintf(&sb, "--- Result %d (score: %.2f) ---\n%s\n\n", i+1, r.Similarity, r.Content)
		}
	}
	if len(researchResults) > 0 {
		fmt.Fprintf(&sb, "=== Research Notes ===\n")
		for i, r := range researchResults {
			fmt.Fprintf(&sb, "--- Result %d (score: %.2f) ---\n%s\n\n", i+1, r.Similarity, r.Content)
		}
	}
	return sb.String(), nil
}

func (t *KnowledgeSearchTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input KnowledgeSearchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}
