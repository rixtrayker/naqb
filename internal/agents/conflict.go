package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// ConflictResult contains the output of a cross-chapter conflict check.
type ConflictResult struct {
	ChapterNum int
	Level      string
	Findings   string
	HasIssues  bool
}

// RunConflictCheck compares the current chapter against all preceding chapters
// and the outline to detect factual or narrative contradictions.
// Returns nil findings (no error) when level is "off" or there are no previous chapters.
func RunConflictCheck(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, level string) (*ConflictResult, error) {
	if level == "off" || level == "" {
		return &ConflictResult{ChapterNum: chapterNum, Level: level}, nil
	}

	log.Info("conflict check start", "chapter", chapterNum, "level", level)

	// Load the current chapter
	currentPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	currentData, err := os.ReadFile(currentPath)
	if err != nil {
		return nil, fmt.Errorf("conflict check: reading chapter %d: %w", chapterNum, err)
	}
	currentContent := string(currentData)

	// Collect preceding chapters (capped to avoid token overflow)
	preceding := collectPrecedingChapters(bookDir, cfg, chapterNum, level)
	if len(preceding) == 0 {
		log.Info("conflict check: no preceding chapters, skipping LLM call", "chapter", chapterNum)
		return &ConflictResult{ChapterNum: chapterNum, Level: level, Findings: "(No previous chapters to compare.)"}, nil
	}

	findings, err := runConflictLLM(ctx, client, cfg, chapterNum, currentContent, preceding, level)
	if err != nil {
		return nil, fmt.Errorf("conflict check LLM failed: %w", err)
	}

	hasIssues := parseVerdict(findings, looksLikeConflictHeuristic)
	log.Info("conflict check done", "chapter", chapterNum, "hasIssues", hasIssues)

	return &ConflictResult{
		ChapterNum: chapterNum,
		Level:      level,
		Findings:   findings,
		HasIssues:  hasIssues,
	}, nil
}

// WriteConflictReport appends conflict findings to pipeline-report.md.
func WriteConflictReport(bookDir string, result *ConflictResult) error {
	reportPath := filepath.Join(bookDir, "pipeline-report.md")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Conflict Check — Chapter %d\n\n", result.ChapterNum))
	sb.WriteString(fmt.Sprintf("**Level:** %s  \n", result.Level))
	if result.HasIssues {
		sb.WriteString("**Status:** ⚠ Potential conflicts found\n\n")
	} else {
		sb.WriteString("**Status:** ✓ No conflicts detected\n\n")
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

type precedingChapter struct {
	Num     int
	Title   string
	Content string
}

func collectPrecedingChapters(bookDir string, cfg *config.BookConfig, currentNum int, level string) []precedingChapter {
	maxCharsPerChapter := CharsForLevel(AnalysisLevel(level))

	var result []precedingChapter
	for _, ch := range cfg.Chapters {
		if ch.Number >= currentNum {
			continue
		}
		path := filepath.Join(bookDir, "chapters", config.ChapterFilename(ch.Number))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > maxCharsPerChapter {
			content = content[:maxCharsPerChapter] + "\n… (truncated)"
		}
		result = append(result, precedingChapter{
			Num:     ch.Number,
			Title:   ch.Title,
			Content: content,
		})
	}
	return result
}

func runConflictLLM(ctx context.Context, client llm.Provider, cfg *config.BookConfig, chapterNum int, current string, preceding []precedingChapter, level string) (string, error) {
	lvl := AnalysisLevel(level)
	depth := DepthInstruction(lvl, "conflict")

	system := `You are a professional book editor specializing in consistency and continuity.
Your job is to find contradictions, factual conflicts, and narrative inconsistencies between chapters.
Be precise and concise. Quote the specific passages that conflict.
If no conflicts are found, say so clearly.
At the end of your response, output exactly one line: VERDICT: YES (conflicts found) or VERDICT: NO (no conflicts)`

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Book: %s | Domain: %s | Language: %s\n\n", cfg.Title, cfg.Domain, cfg.Language))
	sb.WriteString(fmt.Sprintf("## Previous Chapters (for reference)\n\n"))
	for _, p := range preceding {
		sb.WriteString(fmt.Sprintf("### Chapter %d: %s\n\n%s\n\n", p.Num, p.Title, p.Content))
	}
	sb.WriteString(fmt.Sprintf("## Current Chapter %d (being checked)\n\n%s\n\n", chapterNum, current))
	sb.WriteString(fmt.Sprintf("---\n%s\n\nList any conflicts or contradictions between the current chapter and the preceding chapters.\n", depth))

	userMsg := sb.String()
	if len(userMsg) > llm.MaxInputQA {
		userMsg = userMsg[:llm.MaxInputQA] + "\n… (truncated)"
	}

	model := ModelFor(StageConflict, cfg)

	return client.Complete(ctx, model, system, []llm.Message{
		{Role: "user", Content: userMsg},
	}, llm.TokensAnalysis)
}

func looksLikeConflictHeuristic(findings string) bool {
	lower := strings.ToLower(findings)
	negative := []string{"no conflict", "no contradiction", "no inconsistenc", "none found", "no issues"}
	for _, n := range negative {
		if strings.Contains(lower, n) {
			return false
		}
	}
	conflict := []string{"conflict", "contradiction", "inconsistenc", "disagree", "contradict", "mismatch"}
	for _, c := range conflict {
		if strings.Contains(lower, c) {
			return true
		}
	}
	return false
}

// ConflictCheckTimestamp returns a short timestamp string for report headers.
func ConflictCheckTimestamp() string {
	return time.Now().Format("2006-01-02 15:04")
}
