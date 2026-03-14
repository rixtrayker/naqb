package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// QAResult holds the results of a QA check on a chapter.
type QAResult struct {
	ChapterNum       int
	DeterministicOK  bool
	DeterministicMsg string
	LLMReport        string
	Passed           bool
	Issues           []string
}

// RunQA performs deterministic + LLM audit on a chapter.
func RunQA(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int) (*QAResult, error) {
	log.Info("QA start", "chapter", chapterNum)
	result := &QAResult{ChapterNum: chapterNum}

	chapterPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		log.Error("QA: chapter file not found", "chapter", chapterNum, "path", chapterPath, "err", err)
		return nil, fmt.Errorf("chapter file not found: %w", err)
	}
	content := string(data)
	log.Debug("QA: chapter loaded", "chapter", chapterNum, "bytes", len(data))

	// --- Deterministic checks ---
	issues := runDeterministicChecks(content, cfg)
	result.Issues = issues
	result.DeterministicOK = len(issues) == 0

	if result.DeterministicOK {
		result.DeterministicMsg = "All deterministic checks passed."
		log.Info("QA deterministic: passed", "chapter", chapterNum)
	} else {
		result.DeterministicMsg = fmt.Sprintf("%d issue(s) found:\n- %s",
			len(issues), strings.Join(issues, "\n- "))
		log.Warn("QA deterministic: issues found", "chapter", chapterNum, "count", len(issues))
		for _, issue := range issues {
			log.Debug("QA issue", "chapter", chapterNum, "issue", issue)
		}
	}

	// --- LLM audit ---
	var llmReport string
	if client != nil {
		log.Info("QA LLM audit start", "chapter", chapterNum)
		var auditErr error
		llmReport, auditErr = runLLMAudit(ctx, client, bookDir, cfg, chapterNum, content)
		if auditErr != nil {
			log.Error("QA LLM audit failed", "chapter", chapterNum, "err", auditErr)
			llmReport = fmt.Sprintf("LLM audit failed: %v", auditErr)
		} else {
			log.Info("QA LLM audit done", "chapter", chapterNum)
		}
	} else {
		log.Debug("QA LLM audit skipped (no client)", "chapter", chapterNum)
		llmReport = "(LLM audit skipped)"
	}
	result.LLMReport = llmReport
	result.Passed = result.DeterministicOK

	log.Info("QA done", "chapter", chapterNum, "passed", result.Passed)
	return result, nil
}

// WriteQAReport appends the QA result to pipeline-report.md.
func WriteQAReport(bookDir string, result *QAResult) error {
	reportPath := filepath.Join(bookDir, "pipeline-report.md")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## QA Report — Chapter %d\n\n", result.ChapterNum))
	sb.WriteString(fmt.Sprintf("**Deterministic:** %s\n\n", result.DeterministicMsg))
	if len(result.Issues) > 0 {
		sb.WriteString("**Issues:**\n")
		for _, issue := range result.Issues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("**LLM Audit:**\n\n")
	sb.WriteString(result.LLMReport)
	sb.WriteString("\n\n---\n")

	f, err := os.OpenFile(reportPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(sb.String())
	return err
}

func runDeterministicChecks(content string, cfg *config.BookConfig) []string {
	var issues []string

	lines := strings.Split(content, "\n")

	// 1. Heading hierarchy check (no skips h1→h3)
	if hi := checkHeadingHierarchy(lines); hi != "" {
		issues = append(issues, hi)
	}

	// 2. Code blocks have language tags
	if cl := checkCodeBlockLanguages(content); cl != "" {
		issues = append(issues, cl)
	}

	// 3. Word count check
	wordCount := countWords(content)
	minWords := 500
	maxWords := 8000
	if cfg != nil {
		if cfg.TargetWords > 0 {
			minWords = cfg.TargetWords / 2
			maxWords = cfg.TargetWords * 3
		}
	}
	if wordCount < minWords {
		issues = append(issues, fmt.Sprintf("Word count too low: %d words (minimum %d)", wordCount, minWords))
	} else if wordCount > maxWords {
		issues = append(issues, fmt.Sprintf("Word count too high: %d words (maximum %d)", wordCount, maxWords))
	}

	// 4. ADHD callout syntax check
	if cc := checkCalloutSyntax(content); cc != "" {
		issues = append(issues, cc)
	}

	return issues
}

func checkHeadingHierarchy(lines []string) string {
	prevLevel := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for _, c := range line {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		if level == 0 {
			continue
		}
		// Allow level 1 at start, or going up by at most 1 level at a time
		if prevLevel > 0 && level > prevLevel+1 {
			return fmt.Sprintf("Heading hierarchy skip at line %d: h%d after h%d", i+1, level, prevLevel)
		}
		prevLevel = level
	}
	return ""
}

var codeFence = regexp.MustCompile("^```(\\S*)$")

func checkCodeBlockLanguages(content string) string {
	lines := strings.Split(content, "\n")
	var unlabeled int
	inFence := false
	for _, line := range lines {
		m := codeFence.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		if inFence {
			// This is a closing fence — always bare, that's fine
			inFence = false
		} else {
			// This is an opening fence
			inFence = true
			if m[1] == "" {
				unlabeled++
			}
		}
	}
	if unlabeled > 0 {
		return fmt.Sprintf("%d code block(s) missing language tag", unlabeled)
	}
	return ""
}

func countWords(content string) int {
	// Simple word count: split on whitespace
	fields := strings.Fields(content)
	count := 0
	for _, f := range fields {
		if utf8.RuneCountInString(f) > 0 {
			count++
		}
	}
	return count
}

func checkCalloutSyntax(content string) string {
	// Warn if we see ad-hoc callout attempts that don't match expected format
	badPatterns := []string{"[Note]", "[Warning]", "[Info]", "[TODO]", "[FIXME]"}
	for _, p := range badPatterns {
		if strings.Contains(content, p) {
			return fmt.Sprintf("Non-standard callout found: %q — use [!], [?], or [X] instead", p)
		}
	}
	return ""
}

func runLLMAudit(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, content string) (string, error) {
	systemPrompt, err := readPrompt(bookDir, "qa.md")
	if err != nil {
		systemPrompt = "You are a professional book editor. Review this chapter and provide feedback."
	}

	model := cfg.LLM.QAModel
	if model == "" {
		model = llm.ModelSonnet
	}

	// Build context for QA
	finishedSummaries := buildFinishedSummaries(cfg, chapterNum)

	var ch *config.Chapter
	for i := range cfg.Chapters {
		if cfg.Chapters[i].Number == chapterNum {
			ch = &cfg.Chapters[i]
			break
		}
	}
	chTitle := fmt.Sprintf("Chapter %d", chapterNum)
	if ch != nil {
		chTitle = fmt.Sprintf("Chapter %d: %s", chapterNum, ch.Title)
	}

	userMsg := fmt.Sprintf(`Please review this chapter from the book "%s".

## Book Context
- Language: %s
- Domain: %s
%s

## Chapter Being Reviewed
%s

## Chapter Content
%s`,
		cfg.Title, cfg.Language, cfg.Domain,
		formatSummaries(finishedSummaries),
		chTitle,
		content)

	// Truncate if too long
	if len(userMsg) > 60000 {
		userMsg = userMsg[:60000] + "\n... (content truncated for review)"
	}

	report, err := client.Complete(ctx, model, systemPrompt, []llm.Message{
		{Role: "user", Content: userMsg},
	}, 2048)
	if err != nil {
		return "", err
	}
	return report, nil
}

func formatSummaries(s string) string {
	if s == "" {
		return "(No previous chapters)"
	}
	return "### Previous Chapters\n" + s
}
