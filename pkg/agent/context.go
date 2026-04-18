package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// BuildChapterTask constructs a task prompt for writing a specific chapter,
// injecting the context file (if it exists) or a minimal outline-based prompt.
// When epistemic is non-nil, the knowledge-graph summary for the book is loaded and injected.
// This is the task string passed to Agent.Run for chapter-write jobs.
func BuildChapterTask(bookDir string, cfg *config.BookConfig, chapterNum int, epistemic runtime.EpistemicStore) string {
	if cfg == nil {
		return fmt.Sprintf("Write Chapter %d.\nNo book configuration available.", chapterNum)
	}
	var epistemicSummary string
	if epistemic != nil {
		bookID := cfg.Title // use book title as ID (stable within a project)
		if state, err := epistemic.Load(nil, bookID); err == nil {
			epistemicSummary = state.Summary()
		}
	}
	return buildChapterTaskCore(bookDir, cfg, chapterNum, epistemicSummary)
}

func buildChapterTaskCore(bookDir string, cfg *config.BookConfig, chapterNum int, epistemicSummary string) string {
	if cfg == nil {
		return fmt.Sprintf("Write Chapter %d.\nNo book configuration available.", chapterNum)
	}
	chapterTitle := ""
	chapterSummary := ""
	for _, ch := range cfg.Chapters {
		if ch.Number == chapterNum {
			chapterTitle = ch.Title
			chapterSummary = ch.Summary
			break
		}
	}

	// Try to load a pre-built context file
	contextPath := filepath.Join(bookDir, "contexts", config.ContextFilename(chapterNum))
	if data, err := os.ReadFile(contextPath); err == nil {
		epistemic := ""
		if epistemicSummary != "" {
			epistemic = "\n\n" + epistemicSummary
		}
		return fmt.Sprintf(`Write Chapter %d: %s
%s
Use the context file below as your primary source of guidance.
Use the read_file, search_research, knowledge_search, and web_fetch tools as needed for additional context.
When you have a complete draft, write it to "chapters/%s" using write_file.

<context>
%s
</context>`,
			chapterNum, chapterTitle,
			epistemic,
			config.ChapterFilename(chapterNum),
			string(data))
	}

	// Fall back to outline-based prompt
	outlineText := loadOutlineSection(bookDir, chapterNum)
	epistemic := ""
	if epistemicSummary != "" {
		epistemic = "\n\n" + epistemicSummary + "\n"
	}
	return fmt.Sprintf(`Write Chapter %d: %s
%s
%s
Use search_research and knowledge_search to find relevant research notes and claims.
Use read_file to check previous chapters for continuity.
When you have a complete draft, write it to "chapters/%s" using write_file.
Target length: ~%d words.`,
		chapterNum, chapterTitle,
		epistemic,
		buildChapterBrief(chapterSummary, outlineText),
		config.ChapterFilename(chapterNum),
		cfg.TargetWords)
}

// buildChapterBrief assembles a brief description from summary and outline.
func buildChapterBrief(summary, outline string) string {
	var sb strings.Builder
	if summary != "" {
		sb.WriteString("## Chapter Goal\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}
	if outline != "" {
		sb.WriteString("## Outline\n")
		sb.WriteString(outline)
	}
	return sb.String()
}

// loadOutlineSection tries to extract the section for chapterNum from outline.md.
func loadOutlineSection(bookDir string, chapterNum int) string {
	outlinePath := filepath.Join(bookDir, "outline.md")
	data, err := os.ReadFile(outlinePath)
	if err != nil {
		return ""
	}
	return string(data)
}

// BuildSessionSummary creates a short summary of a session for display.
func BuildSessionSummary(bookDir string, cfg *config.BookConfig, chapterNum int) string {
	if chapterNum == 0 {
		return fmt.Sprintf("Book-level chat for %s", cfg.Title)
	}
	for _, ch := range cfg.Chapters {
		if ch.Number == chapterNum {
			return fmt.Sprintf("Chapter %d: %s", ch.Number, ch.Title)
		}
	}
	return fmt.Sprintf("Chapter %d", chapterNum)
}

// ChapterWordCount returns the current word count for a chapter draft, or 0 if not found.
func ChapterWordCount(bookDir string, chapterNum int) int {
	path := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return wordcount.Count(string(data))
}
