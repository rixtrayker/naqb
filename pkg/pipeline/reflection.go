package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// ReflectionState carries mutable state through the reflection loop.
type ReflectionState struct {
	BookDir     string
	Cfg         *config.BookConfig
	Client      llm.Provider
	ChapterNum  int
	Out         io.Writer
	JobID       string
	GatesPassed []string

	Draft       string
	Issues      []string
	Attempts    int
	MaxAttempts int
	Done        bool
}

// ReflectionResult is the final output of a reflection pipeline.
type ReflectionResult struct {
	Draft     string
	Issues    []string
	Attempts  int
	WordCount int
}

// NewReflectionGraph builds a write → review → rewrite StateGraph.
// The caller must set ChapterNum, MaxAttempts, and Out on the initial state.
func NewReflectionGraph(client llm.Provider, bookDir string, cfg *config.BookConfig) *runtime.StateGraph[ReflectionState] {
	g := runtime.NewStateGraph[ReflectionState]()
	g.SetEntryPoint("write")
	g.AddNode("write", reflectionWriteNode(client, bookDir, cfg))
	g.AddNode("review", reflectionReviewNode(client, bookDir, cfg))
	g.AddNode("save", reflectionSaveNode())
	g.AddEdge("write", "review")
	g.AddConditionalEdges("review", func(state ReflectionState) string {
		if state.Done {
			return "save"
		}
		return "write"
	})
	g.AddEdge("save", "")
	return g
}

// RunReflectionPipeline runs a write → review → rewrite loop until quality passes
// or max attempts are exhausted.
func RunReflectionPipeline(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, out io.Writer) (*ReflectionResult, error) {
	if out == nil {
		out = io.Discard
	}

	graph := NewReflectionGraph(client, bookDir, cfg)
	state := ReflectionState{
		BookDir:     bookDir,
		Cfg:         cfg,
		Client:      client,
		ChapterNum:  chapterNum,
		Out:         out,
		MaxAttempts: 3,
	}

	compiled := graph.Compile()
	final, err := compiled.Invoke(ctx, state)
	if err != nil {
		return nil, err
	}

	return &ReflectionResult{
		Draft:     final.Draft,
		Issues:    final.Issues,
		Attempts:  final.Attempts,
		WordCount: wordcount.Count(final.Draft),
	}, nil
}

func reflectionWriteNode(client llm.Provider, bookDir string, cfg *config.BookConfig) runtime.Node[ReflectionState] {
	return func(ctx context.Context, state ReflectionState, rc *runtime.RunConfig) (ReflectionState, error) {
		state.Attempts++
		fmt.Fprintf(state.Out, "\n📝 Reflection attempt %d/%d — writing chapter %d...\n", state.Attempts, state.MaxAttempts, state.ChapterNum)

		// Build context first
		_, err := agents.WriteContextFile(bookDir, cfg, state.ChapterNum)
		if err != nil {
			log.Warn("reflection: context build failed", "err", err)
		}

		// Read context file
		contextPath := filepath.Join(bookDir, "contexts", config.ContextFilename(state.ChapterNum))
		contextData, err := os.ReadFile(contextPath)
		if err != nil {
			log.Warn("reflection: context file missing, building on-the-fly", "err", err)
			ctxStr, buildErr := agents.BuildContext(bookDir, cfg, state.ChapterNum)
			if buildErr != nil {
				return state, fmt.Errorf("reflection: cannot build context: %w", buildErr)
			}
			contextData = []byte(ctxStr)
		}

		// Read system prompt
		systemPrompt, err := readPrompt(bookDir, "write.md")
		if err != nil {
			systemPrompt = "You are an expert author. Write the requested chapter in full."
		}

		userContent := string(contextData)
		if len(state.Issues) > 0 {
			userContent += "\n\n## Rewrite Feedback — Address These Issues\n" + strings.Join(state.Issues, "\n") + "\n"
		}

		model := agents.ModelFor(agents.StageWrite, cfg)
		messages := []llm.Message{{Role: "user", Content: userContent}}

		content, err := client.Stream(ctx, model, systemPrompt, messages, llm.DefaultMaxTokens, nil)
		if err != nil {
			return state, fmt.Errorf("reflection write: %w", err)
		}

		// Save chapter file
		chaptersDir := filepath.Join(bookDir, "chapters")
		_ = os.MkdirAll(chaptersDir, 0o750)
		outPath := filepath.Join(chaptersDir, config.ChapterFilename(state.ChapterNum))
		if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
			return state, fmt.Errorf("reflection write: save file: %w", err)
		}

		state.Draft = content
		fmt.Fprintf(state.Out, "  Draft written — %d words\n", wordcount.Count(content))
		return state, nil
	}
}

func reflectionReviewNode(client llm.Provider, bookDir string, cfg *config.BookConfig) runtime.Node[ReflectionState] {
	return func(ctx context.Context, state ReflectionState, rc *runtime.RunConfig) (ReflectionState, error) {
		fmt.Fprintf(state.Out, "  🔍 Reviewing chapter %d...\n", state.ChapterNum)
		state.Issues = nil

		// Deterministic QA
		qaResult, err := agents.RunQA(ctx, client, bookDir, cfg, state.ChapterNum)
		if err != nil {
			log.Warn("reflection: QA failed", "err", err)
		} else if !qaResult.Passed {
			for _, issue := range qaResult.Issues {
				state.Issues = append(state.Issues, fmt.Sprintf("QA: %s", issue))
			}
		}

		// Conflict check
		rules, _ := config.LoadRules(bookDir)
		conflictLevel := "standard"
		if rules != nil && rules.QA.ConflictLevel != "" && rules.QA.ConflictLevel != "off" {
			conflictLevel = rules.QA.ConflictLevel
		}
		conflictResult, err := agents.RunConflictCheck(ctx, client, bookDir, cfg, state.ChapterNum, conflictLevel)
		if err != nil {
			log.Warn("reflection: conflict check failed", "err", err)
		} else if conflictResult.HasIssues {
			state.Issues = append(state.Issues, fmt.Sprintf("Conflict: %s", conflictResult.Findings))
		}

		// Gap check
		gapLevel := "standard"
		if rules != nil && rules.QA.GapLevel != "" && rules.QA.GapLevel != "off" {
			gapLevel = rules.QA.GapLevel
		}
		gapResult, err := agents.RunGapAnalysis(ctx, client, bookDir, cfg, state.ChapterNum, gapLevel)
		if err != nil {
			log.Warn("reflection: gap analysis failed", "err", err)
		} else if gapResult.HasGaps {
			state.Issues = append(state.Issues, fmt.Sprintf("Gap: %s", gapResult.Findings))
		}

		if len(state.Issues) == 0 {
			fmt.Fprintf(state.Out, "  ✓ Review passed — no issues found\n")
			state.Done = true
			return state, nil
		}

		fmt.Fprintf(state.Out, "  ⚠ Found %d issue(s):\n", len(state.Issues))
		for _, issue := range state.Issues {
			fmt.Fprintf(state.Out, "    • %s\n", issue)
		}

		if state.Attempts >= state.MaxAttempts {
			fmt.Fprintf(state.Out, "  Max attempts reached — keeping best draft\n")
			state.Done = true
		}
		return state, nil
	}
}

func reflectionSaveNode() runtime.Node[ReflectionState] {
	return func(ctx context.Context, state ReflectionState, cfg *runtime.RunConfig) (ReflectionState, error) {
		fmt.Fprintf(state.Out, "\n✅ Reflection complete — %d attempt(s), final draft %d words\n", state.Attempts, wordcount.Count(state.Draft))
		return state, nil
	}
}

func readPrompt(bookDir, name string) (string, error) {
	path := filepath.Join(bookDir, "prompts", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
