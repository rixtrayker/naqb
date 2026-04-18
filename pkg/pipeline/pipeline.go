// Package pipeline orchestrates the chapter writing stages and manages git commits between them.
package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/agent"
	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/runtime"
)

func init() {
	RegisterStage(StageTypeContext, func() Stage { return &ContextStage{} })
	RegisterStage(StageTypeWrite, func() Stage { return &WriteStage{} })
	RegisterStage(StageTypeQA, func() Stage { return &QAStage{} })
	RegisterStage(StageTypeConflict, func() Stage { return &ConflictStage{} })
	RegisterStage(StageTypeGap, func() Stage { return &GapStage{} })
}

// StageInput is the shared input passed to every Stage.
type StageInput struct {
	BookDir    string
	Cfg        *config.BookConfig
	Client     llm.Provider
	ChapterNum int
	Out        io.Writer

	// Optional — when both are non-nil, WriteStage uses the fantasy agent loop
	// instead of the legacy single-shot WriteChapter call.
	// Pass nil for both to retain the existing behaviour (backward compatible).
	DB               *sql.DB          // nil = no session persistence
	FantasyProvider  fantasy.Provider // nil = use legacy Client
	FantasyModelID   string           // model to use via fantasy (empty = use cfg default)

	// Optional — when JobID is non-empty and DB is non-nil, stages that have
	// already been recorded as complete in stage_progress will be skipped.
	// This enables resuming a failed batch job from where it stopped.
	JobID string
	// GatesPassed lists stage names that have already cleared human-in-the-loop gates.
	// Used when resuming an interrupted pipeline run.
	GatesPassed []string

	// Optional stores for agent session persistence and knowledge-graph summaries.
	SessionStore   runtime.SessionStore
	EpistemicStore runtime.EpistemicStore
}

// StageOutput carries results from a completed Stage.
type StageOutput struct {
	// Path is the primary file produced by this stage (may be empty).
	Path string
	// Message is a short human-readable summary of what was done.
	Message string
	// Token usage and timing — populated by Run() after each stage.
	TokensIn      int
	TokensOut     int
	Duration      time.Duration
	EstimatedCost float64 // USD
	StageName     string
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
	// Gate returns the human-review policy for this stage.
	Gate() GateType
}

// ── Built-in stages ──────────────────────────────────────────────────────────

// ContextStage assembles the single-shot context file for a chapter.
type ContextStage struct{}

func (ContextStage) Name() string { return "context" }
func (ContextStage) CommitMessage(n int) string {
	return fmt.Sprintf("context(%02d): Chapter %d context assembled", n, n)
}
func (ContextStage) Gate() GateType { return GateNone }
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
	return fmt.Sprintf("draft(%02d): Chapter %d first draft", n, n)
}
func (WriteStage) Gate() GateType { return GateNone }
func (WriteStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	// When a fantasy provider is wired in, use the agentic loop.
	if in.FantasyProvider != nil {
		return runWriteStageWithAgent(ctx, in)
	}
	// Legacy path: single-shot LLM call.
	path, err := agents.WriteChapter(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, nil)
	if err != nil {
		return StageOutput{}, err
	}
	return StageOutput{Path: path, Message: "Chapter written → " + path}, nil
}

// runWriteStageWithAgent executes the WriteStage using the fantasy agent loop.
func runWriteStageWithAgent(ctx context.Context, in StageInput) (StageOutput, error) {
	modelID := in.FantasyModelID
	if modelID == "" {
		modelID = in.Cfg.LLM.WriteModel
	}
	if modelID == "" {
		modelID = llm.ModelDefault
	}

	a := agent.New(in.FantasyProvider, modelID, in.BookDir, in.Cfg,
		agent.WithSessionStore(in.SessionStore),
		agent.WithEpistemicStore(in.EpistemicStore),
	)
	task := agent.BuildChapterTask(in.BookDir, in.Cfg, in.ChapterNum, in.EpistemicStore)

	result, err := a.Run(ctx, task, "", func(delta string) {
		fmt.Fprint(in.Out, delta)
	})
	if err != nil {
		return StageOutput{}, fmt.Errorf("agent write stage: %w", err)
	}

	chapterPath := in.BookDir + "/chapters/" + config.ChapterFilename(in.ChapterNum)
	return StageOutput{
		Path:      chapterPath,
		Message:   fmt.Sprintf("Chapter written via agent (%d steps, %d+%d tok)", result.Steps, result.TokensIn, result.TokensOut),
		TokensIn:  int(result.TokensIn),
		TokensOut: int(result.TokensOut),
	}, nil
}

// QAStage runs deterministic + LLM quality checks on a chapter.
type QAStage struct{}

func (QAStage) Name() string { return "qa" }
func (QAStage) CommitMessage(n int) string {
	return fmt.Sprintf("reviewed(%02d): Chapter %d QA complete", n, n)
}
func (QAStage) Gate() GateType { return GateNone }
func (QAStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunQA(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum)
	if err != nil {
		return StageOutput{}, err
	}
	if err := agents.WriteQAReport(in.BookDir, result); err != nil {
		log.Warn("pipeline: QA report write failed", "chapter", in.ChapterNum, "err", err)
	}
	msg := "QA passed — " + result.DeterministicMsg
	if !result.Passed {
		msg = "QA issues: " + result.DeterministicMsg
	}
	return StageOutput{Message: msg}, nil
}

// ConflictStage checks for cross-chapter contradictions.
type ConflictStage struct{ Level string; HumanGate GateType }

func (s ConflictStage) Name() string { return "conflict" }
func (s ConflictStage) CommitMessage(n int) string {
	return fmt.Sprintf("conflict(%02d): Chapter %d conflict check complete", n, n)
}
func (s ConflictStage) Gate() GateType {
	if s.HumanGate != "" {
		return s.HumanGate
	}
	return GateNone
}
func (s ConflictStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunConflictCheck(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, s.Level)
	if err != nil {
		return StageOutput{}, err
	}
	if err := agents.WriteConflictReport(in.BookDir, result); err != nil {
		log.Warn("pipeline: conflict report write failed", "chapter", in.ChapterNum, "err", err)
	}
	msg := "Conflict check: no issues"
	if result.HasIssues {
		msg = "Conflict check: ⚠ potential conflicts found — see pipeline-report.md"
	}
	return StageOutput{Message: msg}, nil
}

// GapStage checks whether the chapter covers its outline fully.
type GapStage struct{ Level string; HumanGate GateType }

func (s GapStage) Name() string { return "gap" }
func (s GapStage) CommitMessage(n int) string {
	return fmt.Sprintf("gap(%02d): Chapter %d gap analysis complete", n, n)
}
func (s GapStage) Gate() GateType {
	if s.HumanGate != "" {
		return s.HumanGate
	}
	return GateNone
}
func (s GapStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	result, err := agents.RunGapAnalysis(ctx, in.Client, in.BookDir, in.Cfg, in.ChapterNum, s.Level)
	if err != nil {
		return StageOutput{}, err
	}
	if err := agents.WriteGapReport(in.BookDir, result); err != nil {
		log.Warn("pipeline: gap report write failed", "chapter", in.ChapterNum, "err", err)
	}
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

// PipelineResult holds per-stage outputs for a completed pipeline run.
type PipelineResult struct {
	Stages []StageOutput
}

// TotalTokensIn returns the sum of input tokens across all stages.
func (r *PipelineResult) TotalTokensIn() int {
	var n int
	for _, s := range r.Stages {
		n += s.TokensIn
	}
	return n
}

// TotalTokensOut returns the sum of output tokens across all stages.
func (r *PipelineResult) TotalTokensOut() int {
	var n int
	for _, s := range r.Stages {
		n += s.TokensOut
	}
	return n
}

// TotalCost returns the sum of estimated costs across all stages.
func (r *PipelineResult) TotalCost() float64 {
	var c float64
	for _, s := range r.Stages {
		c += s.EstimatedCost
	}
	return c
}

// TotalDuration returns the sum of stage durations.
func (r *PipelineResult) TotalDuration() time.Duration {
	var d time.Duration
	for _, s := range r.Stages {
		d += s.Duration
	}
	return d
}

// Run executes a slice of stages in order for a chapter, committing after each.
// When in.JobID and in.DB are both set, stages already recorded in stage_progress
// are skipped, enabling resumption of failed jobs.
func Run(ctx context.Context, stages []Stage, in StageInput) (*PipelineResult, error) {
	total := len(stages)
	log.Info("pipeline start", "chapter", in.ChapterNum, "book", in.Cfg.Title, "stages", total)

	// Load completed stages for resumption.
	completedStages := make(map[string]bool)
	if in.JobID != "" && in.DB != nil {
		var err error
		completedStages, err = dbCompletedStages(in.DB, in.JobID)
		if err != nil {
			return nil, fmt.Errorf("pipeline: load stage progress for job %s: %w", in.JobID, err)
		}
	}

	graph := runtime.NewStateGraph[PipelineState]()
	for i, stage := range stages {
		graph.AddNode(stage.Name(), stageNode(stage, i, total))
	}
	for i := 0; i < len(stages)-1; i++ {
		graph.AddEdge(stages[i].Name(), stages[i+1].Name())
	}
	if len(stages) > 0 {
		graph.SetEntryPoint(stages[0].Name())
	}

	state := PipelineState{
		BookDir:         in.BookDir,
		Cfg:             in.Cfg,
		Client:          in.Client,
		ChapterNum:      in.ChapterNum,
		Out:             in.Out,
		DB:              in.DB,
		FantasyProvider: in.FantasyProvider,
		FantasyModelID:  in.FantasyModelID,
		JobID:           in.JobID,
		GatesPassed:     in.GatesPassed,
		Completed:       completedStages,
	}

	compiled := graph.Compile()
	finalState, err := compiled.Invoke(ctx, state)
	if err != nil {
		log.Error("pipeline stage failed", "chapter", in.ChapterNum, "err", err)
		return &PipelineResult{Stages: finalState.Stages}, err
	}

	log.Info("pipeline complete", "chapter", in.ChapterNum)
	return &PipelineResult{Stages: finalState.Stages}, nil
}

// dbMarkStageComplete wraps db.MarkStageComplete to avoid a package import
// in the pipeline package (db is already imported transitively via sql.DB).
func dbMarkStageComplete(sqlDB *sql.DB, jobID, stageName string) error {
	_, err := sqlDB.Exec(
		`INSERT OR IGNORE INTO stage_progress(job_id, stage_name) VALUES (?, ?)`,
		jobID, stageName,
	)
	return err
}

// dbCompletedStages loads completed stage names for a job.
func dbCompletedStages(sqlDB *sql.DB, jobID string) (map[string]bool, error) {
	rows, err := sqlDB.Query(
		`SELECT stage_name FROM stage_progress WHERE job_id = ?`, jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result[name] = true
	}
	return result, rows.Err()
}

// estimateCost calculates USD cost from token counts using the configured model.
func estimateCost(tokIn, tokOut int, cfg *config.BookConfig) float64 {
	// Try to find model in registry; fall back to zero.
	model := cfg.LLM.WriteModel
	if model == "" {
		model = llm.ModelDefault
	}
	caps, ok := llm.KnownModels[model]
	if !ok {
		return 0
	}
	inCost := float64(tokIn) / 1_000_000 * caps.InputCostPerMTok()
	outCost := float64(tokOut) / 1_000_000 * caps.OutputCostPerMTok()
	return inCost + outCost
}

// RunChapterPipeline runs the full stage set (context + write + qa + optional
// conflict + gap) for a chapter. Rules are loaded from config/rules.yaml.
func RunChapterPipeline(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, out io.Writer) (*PipelineResult, error) {
	rules, _ := config.LoadRules(bookDir)
	return Run(ctx, DefaultStagesFor(rules), StageInput{
		BookDir:    bookDir,
		Cfg:        cfg,
		Client:     client,
		ChapterNum: chapterNum,
		Out:        out,
	})
}
