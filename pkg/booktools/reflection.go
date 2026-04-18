package booktools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// RunReflectionInput is the input for the run_reflection tool.
type RunReflectionInput struct {
	ChapterNum int `json:"chapter_num" jsonschema:"description=Chapter number to write with reflection"`
}

// RunReflectionTool runs the reflection pipeline in the background.
type RunReflectionTool struct {
	spawner runtime.TaskSpawner
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
}

// NewRunReflectionTool creates a reflection spawn tool.
func NewRunReflectionTool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, client llm.Provider) runtime.Tool {
	return &RunReflectionTool{spawner: spawner, bookDir: bookDir, cfg: cfg, client: client}
}

func (t *RunReflectionTool) Name() string        { return "run_reflection" }
func (t *RunReflectionTool) Description() string { return "Write a chapter using an iterative reflection loop: write → review → rewrite until quality passes or max attempts reached. Runs in the background." }
func (t *RunReflectionTool) Schema() any         { return nil }

func (t *RunReflectionTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args RunReflectionInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}

	label := fmt.Sprintf("Reflective Write Chapter %d", args.ChapterNum)
	id := t.spawner.Spawn(label, "reflection", args.ChapterNum, func(bgCtx context.Context) (string, error) {
		res, err := pipeline.RunReflectionPipeline(bgCtx, t.client, t.bookDir, t.cfg, args.ChapterNum, nil)
		if err != nil {
			return "", err
		}
		status := "passed"
		if len(res.Issues) > 0 {
			status = fmt.Sprintf("%d issue(s) remaining", len(res.Issues))
		}
		return fmt.Sprintf("Chapter %d written — %d words, %d attempt(s), review: %s",
			args.ChapterNum, res.WordCount, res.Attempts, status), nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *RunReflectionTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input RunReflectionInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

// ReflectionInput is the input for the reflection tool.
type ReflectionInput struct {
	ChapterNum    int `json:"chapter_num" jsonschema:"description=Chapter number to write with reflection"`
	MaxIterations int `json:"max_iterations,omitempty" jsonschema:"description=Maximum rewrite attempts (default 3)"`
}

// ReflectionTool runs the reflection pipeline inline with checkpointing support.
type ReflectionTool struct {
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
	db      *sql.DB
}

// NewReflectionTool creates an inline reflection tool with optional DB checkpointing.
func NewReflectionTool(bookDir string, cfg *config.BookConfig, client llm.Provider, db *sql.DB) runtime.Tool {
	return &ReflectionTool{bookDir: bookDir, cfg: cfg, client: client, db: db}
}

func (t *ReflectionTool) Name() string        { return "reflection" }
func (t *ReflectionTool) Description() string { return "Write a chapter using an iterative reflection loop: write → review → rewrite until quality passes or max attempts reached. Runs inline and can resume from a checkpoint if interrupted." }
func (t *ReflectionTool) Schema() any         { return nil }

func (t *ReflectionTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args ReflectionInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}
	maxIter := args.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	state := pipeline.ReflectionState{
		BookDir:     t.bookDir,
		Cfg:         t.cfg,
		Client:      t.client,
		ChapterNum:  args.ChapterNum,
		Out:         io.Discard,
		MaxAttempts: maxIter,
	}

	graph := pipeline.NewReflectionGraph(t.client, t.bookDir, t.cfg)
	compiled := graph.Compile()

	if t.db != nil {
		cp := &runtime.DBCheckpointer[pipeline.ReflectionState]{
			DB:          t.db,
			Serialize:   serializeReflectionState,
			Deserialize: deserializeReflectionState,
		}
		compiled.SetCheckpointer(cp)
	}

	final, err := compiled.Invoke(ctx, state, runtime.WithThreadID(fmt.Sprintf("reflect:%d", args.ChapterNum)))
	if err != nil {
		return "", fmt.Errorf("reflection failed: %w", err)
	}

	status := "passed"
	if len(final.Issues) > 0 {
		status = fmt.Sprintf("%d issue(s) remaining", len(final.Issues))
	}
	return fmt.Sprintf("Chapter %d — reflection complete after %d attempt(s). Review: %s. Draft: %d words.",
		args.ChapterNum, final.Attempts, status, wordcount.Count(final.Draft)), nil
}

func (t *ReflectionTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input ReflectionInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

// reflectionSnapshot is the JSON-serializable subset of ReflectionState.
type reflectionSnapshot struct {
	ChapterNum  int      `json:"chapter_num"`
	JobID       string   `json:"job_id"`
	GatesPassed []string `json:"gates_passed"`
	Draft       string   `json:"draft"`
	Issues      []string `json:"issues"`
	Attempts    int      `json:"attempts"`
	MaxAttempts int      `json:"max_attempts"`
	Done        bool     `json:"done"`
}

func serializeReflectionState(state pipeline.ReflectionState) ([]byte, error) {
	snap := reflectionSnapshot{
		ChapterNum:  state.ChapterNum,
		JobID:       state.JobID,
		GatesPassed: state.GatesPassed,
		Draft:       state.Draft,
		Issues:      state.Issues,
		Attempts:    state.Attempts,
		MaxAttempts: state.MaxAttempts,
		Done:        state.Done,
	}
	return json.Marshal(snap)
}

func deserializeReflectionState(data []byte) (pipeline.ReflectionState, error) {
	var snap reflectionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return pipeline.ReflectionState{}, err
	}
	return pipeline.ReflectionState{
		ChapterNum:  snap.ChapterNum,
		Out:         io.Discard,
		JobID:       snap.JobID,
		GatesPassed: snap.GatesPassed,
		Draft:       snap.Draft,
		Issues:      snap.Issues,
		Attempts:    snap.Attempts,
		MaxAttempts: snap.MaxAttempts,
		Done:        snap.Done,
	}, nil
}
