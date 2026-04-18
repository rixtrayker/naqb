package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// ChapterAnalysis holds the analysis of a single chapter.
type ChapterAnalysis struct {
	Number      int
	Title       string
	Status      string
	Words       int
	Target      int
	Percent     int
	HasContext   bool
	HasQAReport bool
}

// ProjectAnalysis holds a snapshot of the entire book project state.
type ProjectAnalysis struct {
	BookDir           string
	Title             string
	Author            string
	Language          string
	Domain            string
	Synopsis          string
	TotalChapters     int
	TotalWords        int
	TotalTarget       int
	Chapters          []ChapterAnalysis
	OutlineExists     bool
	ResearchNoteCount int
	EpistemicSummary  string
}

// Analyze scans a book project and returns a ProjectAnalysis.
// epistemic may be nil (epistemic summary will be empty).
func Analyze(bookDir string, cfg *config.BookConfig, epistemic runtime.EpistemicStore) *ProjectAnalysis {
	if cfg == nil {
		return &ProjectAnalysis{BookDir: bookDir}
	}

	a := &ProjectAnalysis{
		BookDir:       bookDir,
		Title:         cfg.Title,
		Author:        cfg.Author,
		Language:      cfg.Language,
		Domain:        cfg.Domain,
		Synopsis:      cfg.Synopsis,
		TotalChapters: len(cfg.Chapters),
	}

	perChapterTarget := cfg.TargetWords
	if perChapterTarget <= 0 {
		perChapterTarget = 3000
	}
	a.TotalTarget = perChapterTarget * len(cfg.Chapters)

	// Scan each chapter
	for _, ch := range cfg.Chapters {
		ca := ChapterAnalysis{
			Number: ch.Number,
			Title:  ch.Title,
			Status: string(ch.Status),
			Target: perChapterTarget,
		}
		if ca.Status == "" {
			ca.Status = "pending"
		}

		// Check chapter file
		chPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(ch.Number))
		if data, err := os.ReadFile(chPath); err == nil {
			ca.Words = wordcount.Count(string(data))
			if ca.Status == "pending" && ca.Words > 0 {
				ca.Status = "written"
			}
		}

		// Progress percentage
		if ca.Target > 0 {
			ca.Percent = ca.Words * 100 / ca.Target
		}

		// Check context file
		ctxPath := filepath.Join(bookDir, "contexts", config.ContextFilename(ch.Number))
		if _, err := os.Stat(ctxPath); err == nil {
			ca.HasContext = true
		}

		// Check QA report existence in pipeline-report.md
		reportPath := filepath.Join(bookDir, "pipeline-report.md")
		if data, err := os.ReadFile(reportPath); err == nil {
			marker := fmt.Sprintf("QA Report — Chapter %d", ch.Number)
			ca.HasQAReport = strings.Contains(string(data), marker)
		}

		a.Chapters = append(a.Chapters, ca)
		a.TotalWords += ca.Words
	}

	// Check outline
	if _, err := os.Stat(filepath.Join(bookDir, "outline.md")); err == nil {
		a.OutlineExists = true
	}

	// Count research notes
	researchDir := filepath.Join(bookDir, ".naqb", "research")
	if entries, err := os.ReadDir(researchDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				a.ResearchNoteCount++
			}
		}
	}
	// Also check legacy research/ directory
	legacyDir := filepath.Join(bookDir, "research")
	if entries, err := os.ReadDir(legacyDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				a.ResearchNoteCount++
			}
		}
	}

	// Epistemic summary
	if epistemic != nil {
		if state, err := epistemic.Load(context.Background(), cfg.Title); err == nil {
			a.EpistemicSummary = state.Summary()
		}
	}

	return a
}

// SystemPromptSection renders a Markdown section suitable for injection into
// the agent system prompt. It includes the project status table and background
// work tool descriptions.
func (a *ProjectAnalysis) SystemPromptSection() string {
	if a == nil || a.Title == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Book Project\n")
	fmt.Fprintf(&sb, "- **Title:** %s\n", a.Title)
	fmt.Fprintf(&sb, "- **Author:** %s\n", a.Author)
	fmt.Fprintf(&sb, "- **Language:** %s\n", a.Language)
	fmt.Fprintf(&sb, "- **Domain:** %s\n", a.Domain)
	if a.Synopsis != "" {
		fmt.Fprintf(&sb, "- **Synopsis:** %s\n", a.Synopsis)
	}

	sb.WriteString("\n## Project Status\n")
	sb.WriteString("| Ch | Title | Status | Words | Progress | Context | QA |\n")
	sb.WriteString("|----|-------|--------|-------|----------|---------|----|\n")
	for _, ch := range a.Chapters {
		ctx := " "
		if ch.HasContext {
			ctx = "Y"
		}
		qa := " "
		if ch.HasQAReport {
			qa = "Y"
		}
		fmt.Fprintf(&sb, "| %d | %s | %s | %d | %d%% | %s | %s |\n",
			ch.Number, ch.Title, ch.Status, ch.Words, ch.Percent, ctx, qa)
	}

	totalPct := 0
	if a.TotalTarget > 0 {
		totalPct = a.TotalWords * 100 / a.TotalTarget
	}
	fmt.Fprintf(&sb, "\nTotal: %d / %d words (%d%%)\n", a.TotalWords, a.TotalTarget, totalPct)

	if a.OutlineExists {
		sb.WriteString("Outline: exists\n")
	}
	if a.ResearchNoteCount > 0 {
		fmt.Fprintf(&sb, "Research notes: %d\n", a.ResearchNoteCount)
	}

	if a.EpistemicSummary != "" {
		sb.WriteString("\n## Epistemic State\n")
		sb.WriteString(a.EpistemicSummary)
		sb.WriteString("\n")
	}

	return sb.String()
}
