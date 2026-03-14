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

// ConflictStage checks for cross-chapter contradictions.
type ConflictStage struct{ Level string }

func (s ConflictStage) Name() string { return "conflict" }
func (s ConflictStage) CommitMessage(n int) string {
	return fmt.Sprintf("conflict(%02d): Chapter %d conflict check complete", n, n)
}
func (s ConflictStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunConflictCheck(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, s.Level)
	if err != nil {
		return StageOutput{}, err
	}
	_ = agents.WriteConflictReport(in.BookDir, result)
	msg := "Conflict check: no issues"
	if result.HasIssues {
		msg = "Conflict check: ⚠ potential conflicts found — see pipeline-report.md"
	}
	return StageOutput{Message: msg}, nil
}

// GapStage checks whether the chapter covers its outline fully.
type GapStage struct{ Level string }

func (s GapStage) Name() string { return "gap" }
func (s GapStage) CommitMessage(n int) string {
	return fmt.Sprintf("gap(%02d): Chapter %d gap analysis complete", n, n)
}
func (s GapStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunGapAnalysis(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, s.Level)
	if err != nil {
		return StageOutput{}, err
	}
	_ = agents.WriteGapReport(in.BookDir, result)
	msg := "Gap analysis: outline well-covered"
	if result.HasGaps {
		msg = "Gap analysis: ⚠ coverage gaps found — see pipeline-report.md"
	}
	return StageOutput{Message: msg}, nil
}

// DefaultStages is the standard chapter pipeline (context → write → qa).
// For pipelines with conflict/gap checks use DefaultStagesFor instead.
var DefaultStages = []Stage{
	ContextStage{},
	WriteStage{},
	QAStage{},
}

// DefaultStagesFor returns the stage list for a chapter pipeline, conditionally
// appending ConflictStage and GapStage based on rules.yaml settings.
func DefaultStagesFor(rules *config.Rules) []Stage {
	stages := []Stage{ContextStage{}, WriteStage{}, QAStage{}}
	if rules == nil {
		return stages
	}
	if rules.QA.ConflictLevel != "off" && rules.QA.ConflictLevel != "" {
		stages = append(stages, ConflictStage{Level: rules.QA.ConflictLevel})
	}
	if rules.QA.GapLevel != "off" && rules.QA.GapLevel != "" {
		stages = append(stages, GapStage{Level: rules.QA.GapLevel})
	}
	return stages
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

// RunChapterPipeline runs the full stage set (context + write + qa + optional
// conflict + gap) for a chapter. Rules are loaded from config/rules.yaml.
func RunChapterPipeline(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, out io.Writer) error {
	rules, _ := config.LoadRules(bookDir)
	return Run(ctx, DefaultStagesFor(rules), StageInput{
		BookDir:    bookDir,
		Cfg:        cfg,
		Client:     client,
		ChapterNum: chapterNum,
		Out:        out,
	})
}
