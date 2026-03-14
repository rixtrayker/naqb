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

// StageInput is the shared input passed to every Stage.
type StageInput struct {
	BookDir    string
	Cfg        *config.BookConfig
	Client     llm.Provider
	ChapterNum int
	Out        io.Writer
}

// StageOutput carries results from a completed Stage.
type StageOutput struct {
	// Path is the primary file produced by this stage (may be empty).
	Path string
	// Message is a short human-readable summary of what was done.
	Message string
}

// Stage is the interface implemented by each pipeline step.
type Stage interface {
	// Name returns a short identifier used in logs and progress output.
	Name() string
	// Run executes the stage and returns its output.
	Run(ctx context.Context, in StageInput) (StageOutput, error)
	// CommitMessage returns the git commit message to use after this stage,
	// or an empty string to skip the commit.
	CommitMessage(chapterNum int) string
}

// ── Built-in stages ──────────────────────────────────────────────────────────

// ContextStage assembles the single-shot context file for a chapter.
type ContextStage struct{}

func (ContextStage) Name() string { return "context" }
func (ContextStage) CommitMessage(n int) string {
	return fmt.Sprintf("context(%02d): Chapter %d context assembled", n, n)
}
func (ContextStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	path, err := agents.WriteContextFile(in.BookDir, in.Cfg, in.ChapterNum)
	if err != nil {
		return StageOutput{}, err
	}
	return StageOutput{Path: path, Message: "Context written → " + path}, nil
}

// WriteStage calls the LLM to write the chapter draft.
type WriteStage struct{}

func (WriteStage) Name() string { return "write" }
func (WriteStage) CommitMessage(n int) string {
	return fmt.Sprintf("chapter(%02d): Chapter %d first draft", n, n)
}
func (WriteStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	path, err := agents.WriteChapter(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, nil)
	if err != nil {
		return StageOutput{}, err
	}
	return StageOutput{Path: path, Message: "Chapter written → " + path}, nil
}

// QAStage runs deterministic + LLM quality checks on a chapter.
type QAStage struct{}

func (QAStage) Name() string { return "qa" }
func (QAStage) CommitMessage(n int) string {
	return fmt.Sprintf("qa(%02d): Chapter %d QA complete", n, n)
}
func (QAStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunQA(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum)
	if err != nil {
		return StageOutput{}, err
	}
	_ = agents.WriteQAReport(in.BookDir, result)
	msg := "QA passed — " + result.DeterministicMsg
	if !result.Passed {
		msg = "QA issues: " + result.DeterministicMsg
	}
	return StageOutput{Message: msg}, nil
}

// DefaultStages is the standard chapter pipeline (context → write → qa).
var DefaultStages = []Stage{
	ContextStage{},
	WriteStage{},
	QAStage{},
}

// ── Runner ───────────────────────────────────────────────────────────────────

// Run executes a slice of stages in order for a chapter, committing after each.
func Run(ctx context.Context, stages []Stage, in StageInput) error {
	total := len(stages)
	log.Info("pipeline start", "chapter", in.ChapterNum, "book", in.Cfg.Title, "stages", total)

	for i, stage := range stages {
		fmt.Fprintf(in.Out, "  [%d/%d] %s (chapter %d)...\n", i+1, total, stage.Name(), in.ChapterNum)

		out, err := stage.Run(ctx, in)
		if err != nil {
			log.Error("pipeline stage failed", "stage", stage.Name(), "chapter", in.ChapterNum, "err", err)
			return fmt.Errorf("%s stage failed: %w", stage.Name(), err)
		}

		if out.Message != "" {
			fmt.Fprintf(in.Out, "        %s\n", out.Message)
		}
		log.Info("pipeline stage done", "stage", stage.Name(), "chapter", in.ChapterNum)

		if msg := stage.CommitMessage(in.ChapterNum); msg != "" {
			if err := GitCommit(in.BookDir, msg); err != nil {
				log.Warn("pipeline: git commit skipped", "stage", stage.Name(), "chapter", in.ChapterNum, "err", err)
				fmt.Fprintf(in.Out, "        (git commit skipped: %v)\n", err)
			}
		}
	}

	log.Info("pipeline complete", "chapter", in.ChapterNum)
	return nil
}

// RunChapterPipeline runs the default stages (context + write + qa) for a chapter.
// This is the main entry point used by the pipeline CLI command.
func RunChapterPipeline(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, out io.Writer) error {
	return Run(ctx, DefaultStages, StageInput{
		BookDir:    bookDir,
		Cfg:        cfg,
		Client:     client,
		ChapterNum: chapterNum,
		Out:        out,
	})
}
