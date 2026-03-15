package agents

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
	"github.com/amr/naqb/internal/wordcount"
)

// FixMode controls what source of issues drives the rewrite.
type FixMode string

const (
	FixModeQA      FixMode = "qa"      // default: read pipeline-report.md → rewrite
	FixModeGap     FixMode = "gap"     // RunGapAnalysis() → rewrite
	FixModeStyle   FixMode = "style"   // extractStyleMarkers() → rewrite
	FixModeRefresh FixMode = "refresh" // rebuild context + re-run QA, no rewrite
)

// FixResult contains the outcome of a fix operation.
type FixResult struct {
	ChapterNum  int
	Mode        FixMode
	IssuesFound []string
	BackupPath  string   // chapters/ch-XX.md.bak
	NewPath     string   // chapters/ch-XX.md (rewritten)
	QAResult    *QAResult
	Skipped     bool   // true for refresh mode (no rewrite)
	Warning     string // e.g. word-count drop warning
}

// fixPromptData is the template input for the fix LLM call.
type fixPromptData struct {
	ChapterNum   int
	ChapterTitle string
	BookTitle    string
	Issues       []string
	OriginalContent string
}

const fixPromptTmpl = `# FIX INSTRUCTIONS
Rewriting Chapter {{.ChapterNum}}: {{.ChapterTitle}} from "{{.BookTitle}}".

Issues to fix:
{{- range .Issues}}
- {{.}}
{{- end}}

# ORIGINAL CHAPTER
{{.OriginalContent}}

# TASK
Fix ALL issues listed above. Preserve the overall content and structure.
No meta-commentary. Output ONLY the rewritten chapter starting with the first heading.`

// FixChapter rewrites a chapter based on the chosen mode.
func FixChapter(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, mode FixMode) (*FixResult, error) {
	log.Info("fix chapter start", "chapter", chapterNum, "mode", mode)

	result := &FixResult{
		ChapterNum: chapterNum,
		Mode:       mode,
	}

	chapterPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))

	// ── Refresh mode: rebuild context + QA, no rewrite ──────────────────────
	if mode == FixModeRefresh {
		result.Skipped = true
		if _, err := WriteContextFile(bookDir, cfg, chapterNum); err != nil {
			return nil, fmt.Errorf("fix refresh: WriteContextFile: %w", err)
		}
		qaResult, err := RunQA(ctx, client, bookDir, cfg, chapterNum)
		if err != nil {
			return nil, fmt.Errorf("fix refresh: RunQA: %w", err)
		}
		_ = WriteQAReport(bookDir, qaResult)
		result.QAResult = qaResult
		log.Info("fix refresh done", "chapter", chapterNum)
		return result, nil
	}

	// ── Read original chapter ────────────────────────────────────────────────
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		return nil, fmt.Errorf("fix: reading chapter %d: %w", chapterNum, err)
	}
	originalContent := string(data)
	originalWordCount := wordcount.Count(originalContent)

	// ── Collect issues by mode ───────────────────────────────────────────────
	var issues []string

	switch mode {
	case FixModeQA:
		issues = ReadQAIssues(bookDir, chapterNum)
		if len(issues) == 0 {
			log.Info("fix qa: no issues found in pipeline-report.md", "chapter", chapterNum)
		}

	case FixModeGap:
		gapResult, err := RunGapAnalysis(ctx, client, bookDir, cfg, chapterNum, "moderate")
		if err != nil {
			return nil, fmt.Errorf("fix gap: RunGapAnalysis: %w", err)
		}
		if gapResult.HasGaps && gapResult.Findings != "" {
			issues = []string{gapResult.Findings}
		}

	case FixModeStyle:
		styleMarkers, err := extractStyleMarkers(ctx, client, cfg, bookDir, chapterNum)
		if err != nil {
			return nil, fmt.Errorf("fix style: extractStyleMarkers: %w", err)
		}
		if styleMarkers != "" {
			issues = []string{styleMarkers}
		}
	}

	result.IssuesFound = issues

	// ── Find chapter title ───────────────────────────────────────────────────
	chTitle := fmt.Sprintf("Chapter %d", chapterNum)
	for _, ch := range cfg.Chapters {
		if ch.Number == chapterNum {
			chTitle = fmt.Sprintf("Chapter %d: %s", ch.Number, ch.Title)
			break
		}
	}

	// ── Run LLM fix ──────────────────────────────────────────────────────────
	rewritten, err := runFixLLM(ctx, client, cfg, chapterNum, chTitle, originalContent, issues, mode)
	if err != nil {
		return nil, fmt.Errorf("fix LLM: %w", err)
	}

	// ── Post-rewrite sanity: warn if word count dropped >40% ─────────────────
	newWordCount := wordcount.Count(rewritten)
	if originalWordCount > 0 && newWordCount < int(float64(originalWordCount)*0.6) {
		result.Warning = fmt.Sprintf("word count dropped from %d to %d (>40%% reduction)", originalWordCount, newWordCount)
		log.Warn("fix: word count drop warning", "chapter", chapterNum, "before", originalWordCount, "after", newWordCount)
	}

	// ── Backup original chapter ───────────────────────────────────────────────
	backupPath, err := backupChapter(bookDir, chapterNum)
	if err != nil {
		return nil, fmt.Errorf("fix: backing up chapter: %w", err)
	}
	result.BackupPath = backupPath

	// ── Write rewritten chapter ───────────────────────────────────────────────
	if err := os.WriteFile(chapterPath, []byte(rewritten), 0o644); err != nil {
		return nil, fmt.Errorf("fix: writing rewritten chapter: %w", err)
	}
	result.NewPath = chapterPath

	// ── Post-fix QA ──────────────────────────────────────────────────────────
	qaResult, err := RunQA(ctx, client, bookDir, cfg, chapterNum)
	if err != nil {
		log.Warn("fix: post-QA failed", "chapter", chapterNum, "err", err)
	} else {
		_ = WriteQAReport(bookDir, qaResult)
		result.QAResult = qaResult
	}

	log.Info("fix chapter done", "chapter", chapterNum, "mode", mode, "backup", backupPath)
	return result, nil
}

// ReadQAIssues extracts issue bullets from the last QA report section in pipeline-report.md.
func ReadQAIssues(bookDir string, chapterNum int) []string {
	reportPath := filepath.Join(bookDir, "pipeline-report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil
	}

	content := string(data)
	marker := fmt.Sprintf("## QA Report — Chapter %d", chapterNum)

	// Find the LAST occurrence of this marker
	lastIdx := strings.LastIndex(content, marker)
	if lastIdx < 0 {
		return nil
	}

	// Extract everything from the last section onwards
	section := content[lastIdx:]

	var issues []string
	inIssues := false
	inLLMAudit := false
	var auditLines []string

	for _, line := range strings.Split(section, "\n") {
		// New top-level section (another ## heading) ends parsing
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, marker) {
			break
		}

		if strings.HasPrefix(line, "**Issues:**") {
			inIssues = true
			inLLMAudit = false
			continue
		}
		if strings.HasPrefix(line, "**LLM Audit:**") {
			inIssues = false
			inLLMAudit = true
			continue
		}
		if strings.HasPrefix(line, "**") && (inIssues || inLLMAudit) {
			inIssues = false
			inLLMAudit = false
		}

		if inIssues && strings.HasPrefix(line, "- ") {
			issues = append(issues, strings.TrimPrefix(line, "- "))
		}
		if inLLMAudit && strings.TrimSpace(line) != "" && line != "---" {
			auditLines = append(auditLines, line)
		}
	}

	// Append LLM audit block as one combined issue item
	if len(auditLines) > 0 {
		issues = append(issues, "LLM Audit:\n"+strings.Join(auditLines, "\n"))
	}

	return issues
}

// backupChapter copies chapters/ch-XX.md → chapters/ch-XX.md.bak and returns the backup path.
func backupChapter(bookDir string, chapterNum int) (string, error) {
	src := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	dst := src + ".bak"

	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("backupChapter: reading source: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("backupChapter: writing backup: %w", err)
	}
	return dst, nil
}

// runFixLLM calls the LLM with the fix prompt and returns the rewritten chapter.
func runFixLLM(ctx context.Context, client llm.Provider, cfg *config.BookConfig, chapterNum int, chapterTitle, originalContent string, issues []string, mode FixMode) (string, error) {
	tmpl, err := template.New("fix").Parse(fixPromptTmpl)
	if err != nil {
		return "", fmt.Errorf("runFixLLM: parse template: %w", err)
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, fixPromptData{
		ChapterNum:      chapterNum,
		ChapterTitle:    chapterTitle,
		BookTitle:       cfg.Title,
		Issues:          issues,
		OriginalContent: originalContent,
	}); err != nil {
		return "", fmt.Errorf("runFixLLM: execute template: %w", err)
	}

	systemPrompt := "You are an expert book editor and author. Follow the fix instructions precisely."

	model := ModelFor(StageFix, cfg)

	rewritten, err := client.Complete(ctx, model, systemPrompt, []llm.Message{
		{Role: "user", Content: promptBuf.String()},
	}, llm.DefaultMaxTokens)
	if err != nil {
		return "", err
	}
	return rewritten, nil
}

// extractStyleMarkers reads the adjacent chapters (±1) and uses the LLM to
// derive style consistency markers, then returns them as a string of issues.
func extractStyleMarkers(ctx context.Context, client llm.Provider, cfg *config.BookConfig, bookDir string, chapterNum int) (string, error) {
	// Read adjacent chapters
	var samples []string
	for _, offset := range []int{-1, 1} {
		adjNum := chapterNum + offset
		if adjNum < 1 {
			continue
		}
		adjPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(adjNum))
		data, err := os.ReadFile(adjPath)
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > 3000 {
			content = content[:3000] + "\n... (truncated)"
		}
		samples = append(samples, fmt.Sprintf("## Adjacent Chapter %d\n%s", adjNum, content))
	}

	// Read the chapter being fixed
	chapterPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chapterNum))
	chapterData, err := os.ReadFile(chapterPath)
	if err != nil {
		return "", fmt.Errorf("extractStyleMarkers: reading chapter: %w", err)
	}
	chapterContent := string(chapterData)
	if len(chapterContent) > 3000 {
		chapterContent = chapterContent[:3000] + "\n... (truncated)"
	}

	adjacentCtx := strings.Join(samples, "\n\n")
	if adjacentCtx == "" {
		adjacentCtx = "(no adjacent chapters available)"
	}

	userMsg := fmt.Sprintf(`Compare Chapter %d's writing style against these adjacent chapters.
Identify ONLY style inconsistencies (tone, heading depth, callout usage, list formatting, sentence length).
List each issue as a brief bullet point.

%s

## Chapter Being Reviewed (Chapter %d)
%s`, chapterNum, adjacentCtx, chapterNum, chapterContent)

	model := ModelFor(StageQA, cfg)

	result, err := client.Complete(ctx, model, "You are a style editor. Be concise and specific.", []llm.Message{
		{Role: "user", Content: userMsg},
	}, 1024)
	if err != nil {
		return "", err
	}
	return result, nil
}
