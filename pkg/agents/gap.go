package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// GapResult contains the output of an outline-coverage gap analysis.
type GapResult struct {
	ChapterNum int
	Level      string
	Findings   string
	HasGaps    bool
}

// RunGapAnalysis compares the written chapter against its outline section to
// find topics that were missed or under-covered.
// Returns a nil-gap result (no error) when level is "off".
func RunGapAnalysis(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, level string) (*GapResult, error) {
	if level == "off" || level == "" {
		return &GapResult{ChapterNum: chapterNum, Level: level}, nil
	}

	log.Info("gap analysis start", "chapter", chapterNum, "level", level)

	// Load the written chapter
	chapterPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	chapterData, err := os.ReadFile(chapterPath)
	if err != nil {
		return nil, fmt.Errorf("gap analysis: reading chapter %d: %w", chapterNum, err)
	}
	chapterContent := string(chapterData)

	// Get the outline section for this chapter
	outlineSection := extractOutlineSection(bookDir, chapterNum, chapterTitle(cfg, chapterNum))
	if outlineSection == "" {
		log.Info("gap analysis: no outline found, skipping", "chapter", chapterNum)
		return &GapResult{
			ChapterNum: chapterNum,
			Level:      level,
			Findings:   "(No outline section found — gap analysis skipped.)",
		}, nil
	}

	findings, err := runGapLLM(ctx, client, cfg, chapterNum, chapterContent, outlineSection, level)
	if err != nil {
		return nil, fmt.Errorf("gap analysis LLM failed: %w", err)
	}

	hasGaps := parseVerdict(findings, looksLikeGapHeuristic)
	log.Info("gap analysis done", "chapter", chapterNum, "hasGaps", hasGaps)

	return &GapResult{
		ChapterNum: chapterNum,
		Level:      level,
		Findings:   findings,
		HasGaps:    hasGaps,
	}, nil
}

// WriteGapReport appends gap analysis findings to pipeline-report.md.
func WriteGapReport(bookDir string, result *GapResult) error {
	reportPath := filepath.Join(bookDir, "pipeline-report.md")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Gap Analysis — Chapter %d\n\n", result.ChapterNum))
	sb.WriteString(fmt.Sprintf("**Level:** %s  \n", result.Level))
	if result.HasGaps {
		sb.WriteString("**Status:** ⚠ Coverage gaps found\n\n")
	} else {
		sb.WriteString("**Status:** ✓ Outline well-covered\n\n")
	}
	sb.WriteString(result.Findings)
	sb.WriteString("\n\n---\n")

	f, err := os.OpenFile(reportPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(sb.String())
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

func runGapLLM(ctx context.Context, client llm.Provider, cfg *config.BookConfig, chapterNum int, content, outline, level string) (string, error) {
	lvl := AnalysisLevel(level)
	depth := DepthInstruction(lvl, "gap")

	system := `You are a professional book editor checking coverage and completeness.
Your job is to compare a written chapter against its planned outline and identify topics that are:
- Missing (not covered at all)
- Superficial (mentioned but not adequately explained)
- Out of scope (covered but not in the outline)
Be concise and actionable. Quote specific outline items that were missed.
If all outline points are well-covered, say so clearly.
At the end of your response, output exactly one line: VERDICT: YES (gaps exist) or VERDICT: NO (fully covered)`

	// Truncate chapter if large
	chapterExcerpt := content
	maxChars := CharsForLevel(lvl)
	if len(chapterExcerpt) > maxChars {
		chapterExcerpt = chapterExcerpt[:maxChars] + "\n… (truncated)"
	}

	var ch *config.Chapter
	for i := range cfg.Chapters {
		if cfg.Chapters[i].Number == chapterNum {
			ch = &cfg.Chapters[i]
			break
		}
	}
	chTitle := fmt.Sprintf("Chapter %d", chapterNum)
	chSummary := ""
	if ch != nil {
		chTitle = fmt.Sprintf("Chapter %d: %s", chapterNum, ch.Title)
		chSummary = ch.Summary
	}

	userMsg := fmt.Sprintf(`Book: %s | Domain: %s | Language: %s

## Planned Outline for %s

%s

%s

---

## Written Chapter Content

%s

---

%s

Identify gaps between the outline and the written chapter.`,
		cfg.Title, cfg.Domain, cfg.Language,
		chTitle,
		outlineSummaryLine(chSummary),
		outline,
		chapterExcerpt,
		depth)

	model := ModelFor(StageGap, cfg)

	return client.Complete(ctx, model, system, []llm.Message{
		{Role: "user", Content: userMsg},
	}, llm.TokensAnalysis)
}

func outlineSummaryLine(summary string) string {
	if summary == "" {
		return ""
	}
	return fmt.Sprintf("**Summary/Goal:** %s\n", summary)
}

// looksLikeGapHeuristic is the fallback detection when the LLM omits the VERDICT line.
func looksLikeGapHeuristic(findings string) bool {
	lower := strings.ToLower(findings)
	covered := []string{"well-covered", "fully covered", "no gaps", "all outline", "comprehensive", "no missing"}
	for _, c := range covered {
		if strings.Contains(lower, c) {
			return false
		}
	}
	gap := []string{"missing", "not covered", "superficial", "gap", "omitted", "lacks", "absent"}
	for _, g := range gap {
		if strings.Contains(lower, g) {
			return true
		}
	}
	return false
}

// chapterTitle returns the title for a chapter from the config, or a fallback.
// (Duplicated from write.go — kept local to avoid package cycle.)
func chapterTitle(cfg *config.BookConfig, n int) string {
	for _, ch := range cfg.Chapters {
		if ch.Number == n {
			return ch.Title
		}
	}
	return fmt.Sprintf("Chapter %d", n)
}
