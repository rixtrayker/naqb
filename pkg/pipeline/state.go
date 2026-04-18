package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/runtime"
)

// PipelineState is the mutable state carried through StateGraph execution.
type PipelineState struct {
	BookDir         string
	Cfg             *config.BookConfig
	Client          llm.Provider
	ChapterNum      int
	Out             io.Writer
	DB              *sql.DB
	FantasyProvider fantasy.Provider
	FantasyModelID  string
	JobID           string
	GatesPassed     []string

	SessionStore   runtime.SessionStore
	EpistemicStore runtime.EpistemicStore

	Stages    []StageOutput
	Completed map[string]bool
}

// toInput reconstructs a StageInput from the state.
func (s PipelineState) toInput() StageInput {
	return StageInput{
		BookDir:         s.BookDir,
		Cfg:             s.Cfg,
		Client:          s.Client,
		ChapterNum:      s.ChapterNum,
		Out:             s.Out,
		DB:              s.DB,
		FantasyProvider: s.FantasyProvider,
		FantasyModelID:  s.FantasyModelID,
		JobID:           s.JobID,
		GatesPassed:     s.GatesPassed,
		SessionStore:    s.SessionStore,
		EpistemicStore:  s.EpistemicStore,
	}
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// stageNode wraps a Stage into a StateGraph node.
func stageNode(stage Stage, index, total int) runtime.Node[PipelineState] {
	return func(ctx context.Context, state PipelineState, cfg *runtime.RunConfig) (PipelineState, error) {
		name := stage.Name()

		// Resumption: skip stages that were already completed in a previous attempt.
		if state.Completed[name] {
			fmt.Fprintf(state.Out, "  [%d/%d] %s (chapter %d) — already done, skipping\n",
				index+1, total, name, state.ChapterNum)
			return state, nil
		}

		// Human-in-the-loop gate check.
		if gate := stage.Gate(); gate != GateNone {
			if !containsString(state.GatesPassed, name) {
				if gate == GateAlways {
					return state, &runtime.InterruptedError{
						NodeID: name,
						Reason: fmt.Sprintf("human approval required for stage %s", name),
					}
				}
				if gate == GateAuto && len(state.Stages) > 0 {
					prev := state.Stages[len(state.Stages)-1]
					if strings.Contains(prev.Message, "issue") || strings.Contains(prev.Message, "gaps found") ||
						strings.Contains(prev.Message, "conflicts found") || strings.Contains(prev.Message, "⚠") {
						return state, &runtime.InterruptedError{
							NodeID: name,
							Reason: fmt.Sprintf("auto-gate triggered for %s due to previous warnings", name),
						}
					}
				}
			}
		}

		fmt.Fprintf(state.Out, "  [%d/%d] %s (chapter %d)...\n",
			index+1, total, name, state.ChapterNum)

		start := time.Now()
		in := state.toInput()
		out, err := stage.Run(ctx, in)
		out.Duration = time.Since(start)
		out.StageName = name

		// Token capture fallback for non-agent stages.
		if out.TokensIn == 0 && out.TokensOut == 0 {
			if tr, ok := state.Client.(llm.TokenReporter); ok {
				out.TokensIn, out.TokensOut = tr.LastTokens()
			}
		}
		if out.TokensIn > 0 || out.TokensOut > 0 {
			out.EstimatedCost = estimateCost(out.TokensIn, out.TokensOut, in.Cfg)
			model := in.Cfg.LLM.WriteModel
			if model == "" {
				model = llm.ModelDefault
			}
			llm.SessionBudget.Record(out.TokensIn, out.TokensOut, model)
		}

		state.Stages = append(state.Stages, out)

		if err != nil {
			return state, fmt.Errorf("%s stage failed: %w", name, err)
		}

		if out.Message != "" {
			fmt.Fprintf(state.Out, "        %s\n", out.Message)
		}
		if out.TokensIn > 0 || out.TokensOut > 0 {
			fmt.Fprintf(state.Out, "        %d in / %d out tok | $%.4f | %.1fs\n",
				out.TokensIn, out.TokensOut, out.EstimatedCost, out.Duration.Seconds())
		}
		log.Info("pipeline stage done", "stage", name, "chapter", state.ChapterNum,
			"duration", out.Duration, "tokensIn", out.TokensIn, "tokensOut", out.TokensOut)

		if msg := stage.CommitMessage(state.ChapterNum); msg != "" {
			if err := GitCommit(state.BookDir, msg); err != nil {
				log.Warn("pipeline: git commit skipped", "stage", name, "chapter", state.ChapterNum, "err", err)
				fmt.Fprintf(state.Out, "        (git commit skipped: %v)\n", err)
			}
		}

		if state.JobID != "" && state.DB != nil {
			if err := dbMarkStageComplete(state.DB, state.JobID, name); err != nil {
				log.Warn("pipeline: could not record stage progress", "job", state.JobID, "stage", name, "err", err)
			}
		}

		state.Completed[name] = true
		return state, nil
	}
}
