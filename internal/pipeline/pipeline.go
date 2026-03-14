// Package pipeline orchestrates the chapter writing stages and manages git commits between them.
package pipeline

import (
	"context"
	"fmt"
	"io"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// RunChapterPipeline runs stages 1 (context) + 2 (write) + 3 (qa) for a chapter.
func RunChapterPipeline(ctx context.Context, client *llm.Client, bookDir string, cfg *config.BookConfig, chapterNum int, out io.Writer) error {
	log.Info("pipeline start", "chapter", chapterNum, "book", cfg.Title)

	// Stage 1: Build context
	fmt.Fprintf(out, "  [1/3] Building context for chapter %d...\n", chapterNum)
	contextPath, err := agents.WriteContextFile(bookDir, cfg, chapterNum)
	if err != nil {
		log.Error("pipeline: context stage failed", "chapter", chapterNum, "err", err)
		return fmt.Errorf("context stage failed: %w", err)
	}
	fmt.Fprintf(out, "        Context written → %s\n", contextPath)
	log.Info("pipeline: context done", "chapter", chapterNum, "path", contextPath)

	if err := GitCommit(bookDir, fmt.Sprintf("context(%02d): Chapter %d context assembled", chapterNum, chapterNum)); err != nil {
		log.Warn("pipeline: git commit skipped after context", "chapter", chapterNum, "err", err)
		fmt.Fprintf(out, "        (git commit skipped: %v)\n", err)
	}

	// Stage 2: Write chapter
	fmt.Fprintf(out, "  [2/3] Writing chapter %d...\n", chapterNum)
	chapterPath, err := agents.WriteChapter(ctx, client, bookDir, cfg, chapterNum, nil)
	if err != nil {
		log.Error("pipeline: write stage failed", "chapter", chapterNum, "err", err)
		return fmt.Errorf("write stage failed: %w", err)
	}
	fmt.Fprintf(out, "        Chapter written → %s\n", chapterPath)
	log.Info("pipeline: write done", "chapter", chapterNum, "path", chapterPath)

	if err := GitCommit(bookDir, fmt.Sprintf("chapter(%02d): Chapter %d first draft", chapterNum, chapterNum)); err != nil {
		log.Warn("pipeline: git commit skipped after write", "chapter", chapterNum, "err", err)
		fmt.Fprintf(out, "        (git commit skipped: %v)\n", err)
	}

	// Stage 3: QA
	fmt.Fprintf(out, "  [3/3] Running QA on chapter %d...\n", chapterNum)
	result, err := agents.RunQA(ctx, client, bookDir, cfg, chapterNum)
	if err != nil {
		log.Error("pipeline: QA stage failed", "chapter", chapterNum, "err", err)
		return fmt.Errorf("QA stage failed: %w", err)
	}

	if err := agents.WriteQAReport(bookDir, result); err != nil {
		log.Warn("pipeline: QA report write failed", "chapter", chapterNum, "err", err)
		fmt.Fprintf(out, "        (QA report write failed: %v)\n", err)
	}

	if result.Passed {
		fmt.Fprintf(out, "        QA: PASSED — %s\n", result.DeterministicMsg)
	} else {
		fmt.Fprintf(out, "        QA: ISSUES FOUND\n%s\n", result.DeterministicMsg)
	}
	log.Info("pipeline: QA done", "chapter", chapterNum, "passed", result.Passed)

	if err := GitCommit(bookDir, fmt.Sprintf("qa(%02d): Chapter %d QA complete", chapterNum, chapterNum)); err != nil {
		log.Warn("pipeline: git commit skipped after QA", "chapter", chapterNum, "err", err)
		fmt.Fprintf(out, "        (git commit skipped: %v)\n", err)
	}

	log.Info("pipeline complete", "chapter", chapterNum)
	return nil
}
