// Package agents implements the LLM-driven pipeline stages: planning, context assembly, chapter writing, and QA.
package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// PlannerResult holds the output from the init interview.
type PlannerResult struct {
	BookConfig *config.BookConfig
	OutlineMD  string
}

// InterviewAnswers collects user responses during init.
type InterviewAnswers struct {
	Title       string
	Author      string
	Language    string
	Domain      string
	Synopsis    string
	NumChapters int
	Template    string // template ID (e.g. "arabic-research", "cs-book", "general")
}

// RunPlanner takes interview answers and generates book.yaml + outline.md via LLM.
func RunPlanner(ctx context.Context, client llm.Provider, answers InterviewAnswers) (*PlannerResult, error) {
	log.Info("planner start", "title", answers.Title, "language", answers.Language, "chapters", answers.NumChapters, "template", answers.Template)
	systemPrompt := `You are an expert book planner. Given information about a book project,
you will produce a structured chapter outline.

Respond with EXACTLY two sections separated by the literal line "---OUTLINE---":

Section 1: A YAML list of chapters. Each chapter must have: number, title, summary.
Example:
- number: 1
  title: "Introduction"
  summary: "Overview of the topic"
- number: 2
  title: "Core Concepts"
  summary: "Fundamental ideas"

Section 2: A detailed Markdown outline for the whole book.
Example:
# Book Outline
## Chapter 1: Introduction
- Key topics...`

	userMsg := fmt.Sprintf(`Please create a book plan for:

Title: %s
Author: %s
Language: %s
Domain/Subject: %s
Synopsis: %s
Number of Chapters: %d

Generate a complete chapter structure with titles and summaries, then a detailed outline.`,
		answers.Title, answers.Author, answers.Language,
		answers.Domain, answers.Synopsis, answers.NumChapters)

	response, err := client.Complete(ctx, llm.ModelHaiku, systemPrompt, []llm.Message{
		{Role: "user", Content: userMsg},
	}, 4096)
	if err != nil {
		log.Error("planner LLM failed", "err", err)
		return nil, fmt.Errorf("planner LLM call failed: %w", err)
	}
	log.Debug("planner LLM response", "chars", len(response))

	parts := strings.SplitN(response, "---OUTLINE---", 2)

	var chapters []config.Chapter
	var outlineMD string

	if len(parts) == 2 {
		chapters = parseChaptersFromYAMLBlock(strings.TrimSpace(parts[0]))
		outlineMD = strings.TrimSpace(parts[1])
	}

	if len(chapters) == 0 {
		log.Warn("planner: LLM did not return parseable chapters, using fallback")
		chapters = buildFallbackChapters(answers.NumChapters)
	}
	if outlineMD == "" {
		log.Warn("planner: LLM did not return outline section, using fallback")
		outlineMD = buildFallbackOutline(answers.Title, chapters)
	}
	log.Info("planner done", "chapters_parsed", len(chapters))

	// Ensure all chapters have file names and status
	for i := range chapters {
		if chapters[i].File == "" {
			chapters[i].File = config.ChapterFilename(chapters[i].Number)
		}
		if chapters[i].Status == "" {
			chapters[i].Status = "pending"
		}
	}

	bookCfg := &config.BookConfig{
		Title:       answers.Title,
		Author:      answers.Author,
		Language:    answers.Language,
		Domain:      answers.Domain,
		Synopsis:    answers.Synopsis,
		TargetWords: 3000,
		Chapters:    chapters,
		CreatedAt:   time.Now(),
		Version:     "0.1.0",
		LLM: config.LLMSettings{
			WriteModel: llm.ModelSonnet,
			QAModel:    llm.ModelSonnet,
			ChatModel:  llm.ModelOpus,
			InitModel:  llm.ModelHaiku,
		},
	}

	return &PlannerResult{
		BookConfig: bookCfg,
		OutlineMD:  outlineMD,
	}, nil
}

// parseChaptersFromYAMLBlock parses a simple YAML list of chapters.
// We use a line-by-line parser to avoid circular imports.
func parseChaptersFromYAMLBlock(yamlStr string) []config.Chapter {
	var chapters []config.Chapter
	var current *config.Chapter

	flush := func() {
		if current != nil && current.Title != "" {
			if current.Number == 0 {
				current.Number = len(chapters) + 1
			}
			chapters = append(chapters, *current)
			current = nil
		}
	}

	for _, rawLine := range strings.Split(yamlStr, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "- number:") {
			flush()
			current = &config.Chapter{}
			numStr := strings.TrimSpace(strings.TrimPrefix(line, "- number:"))
			fmt.Sscanf(numStr, "%d", &current.Number)
		} else if current != nil {
			if strings.HasPrefix(line, "title:") {
				current.Title = cleanYAMLString(strings.TrimPrefix(line, "title:"))
			} else if strings.HasPrefix(line, "summary:") {
				current.Summary = cleanYAMLString(strings.TrimPrefix(line, "summary:"))
			}
		}
	}
	flush()

	return chapters
}

func cleanYAMLString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return s
}

func buildFallbackChapters(n int) []config.Chapter {
	chapters := make([]config.Chapter, n)
	for i := range chapters {
		chapters[i] = config.Chapter{
			Number:  i + 1,
			Title:   fmt.Sprintf("Chapter %d", i+1),
			File:    config.ChapterFilename(i + 1),
			Status:  "pending",
			Summary: fmt.Sprintf("Content for chapter %d", i+1),
		}
	}
	return chapters
}

func buildFallbackOutline(title string, chapters []config.Chapter) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s — Outline\n\n", title))
	for _, ch := range chapters {
		sb.WriteString(fmt.Sprintf("## Chapter %d: %s\n\n%s\n\n", ch.Number, ch.Title, ch.Summary))
	}
	return sb.String()
}
