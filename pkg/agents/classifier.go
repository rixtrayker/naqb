package agents

import (
	"context"
	"strings"

	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// Complexity is a 1-5 score for task difficulty.
type Complexity int

const (
	ComplexityTrivial Complexity = 1 // simple lookup, formatting, single-fact fix
	ComplexitySimple  Complexity = 2 // minor rewrite, single-issue correction
	ComplexityMedium  Complexity = 3 // moderate rewrite, multi-issue fix
	ComplexityHard    Complexity = 4 // significant restructuring
	ComplexityDeep    Complexity = 5 // full rewrite, complex reasoning required
)

// ModelForComplexity maps a complexity score to a model tier.
// Haiku (fast/cheap) handles simple tasks; Opus handles deep reasoning.
func ModelForComplexity(c Complexity) string {
	switch {
	case c <= ComplexitySimple:
		return llm.ModelHaiku
	case c <= ComplexityMedium:
		return llm.ModelSonnet
	default:
		return llm.ModelOpus
	}
}

// ClassifyTask makes a single cheap LLM call to score task complexity (1-5).
// Falls back to ComplexityMedium on any error — never blocks the pipeline.
func ClassifyTask(ctx context.Context, client llm.Provider, taskDescription string) Complexity {
	system := `You are a task complexity classifier. Score the task from 1 to 5:
1 = trivial (single fact, formatting, typo)
2 = simple (minor fix, one clear issue)
3 = medium (multiple issues, moderate rewrite)
4 = hard (significant restructuring)
5 = deep (full rewrite, complex reasoning)

Respond with ONLY a single digit (1-5). No explanation.`

	result, err := client.Complete(ctx, llm.ModelHaiku, system, []llm.Message{
		{Role: "user", Content: taskDescription},
	}, 10) // 10 tokens — just need a single digit
	if err != nil {
		log.Debug("classifier: LLM call failed, defaulting to medium", "err", err)
		return ComplexityMedium
	}

	trimmed := strings.TrimSpace(result)
	if len(trimmed) == 0 {
		return ComplexityMedium
	}
	switch trimmed[0] {
	case '1':
		return ComplexityTrivial
	case '2':
		return ComplexitySimple
	case '3':
		return ComplexityMedium
	case '4':
		return ComplexityHard
	case '5':
		return ComplexityDeep
	}
	log.Debug("classifier: unexpected response, defaulting to medium", "response", trimmed)
	return ComplexityMedium
}
