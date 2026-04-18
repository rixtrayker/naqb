package booktools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/runtime"
)

// RunPlannerInput is the input for the run_planner tool.
type RunPlannerInput struct {
	Goal        string   `json:"goal" jsonschema:"description=High-level goal for the planner (e.g., 'Write chapter 3 with deep research and full QA')"`
	ChapterNum  int      `json:"chapter_num,omitempty" jsonschema:"description=Chapter number to target (optional)"`
	GatesPassed []string `json:"gates_passed,omitempty" jsonschema:"description=Stage IDs that have already cleared human-in-the-loop gates"`
}

// RunPlannerTool generates and executes a custom pipeline DAG.
type RunPlannerTool struct {
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
	db      *sql.DB
}

// NewRunPlannerTool creates a planner tool.
func NewRunPlannerTool(bookDir string, cfg *config.BookConfig, client llm.Provider, db *sql.DB) runtime.Tool {
	return &RunPlannerTool{bookDir: bookDir, cfg: cfg, client: client, db: db}
}

func (t *RunPlannerTool) Name() string        { return "run_planner" }
func (t *RunPlannerTool) Description() string { return "Generate a custom pipeline plan for a goal and execute it automatically. Returns a summary of the planned stages and their results." }
func (t *RunPlannerTool) Schema() any         { return nil }

func (t *RunPlannerTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args RunPlannerInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Goal == "" {
		return "goal is required", nil
	}

	topicID := fmt.Sprintf("planner-%s-%d", t.bookDir, args.ChapterNum)

	// Build initial state (may be overwritten by checkpoint below).
	state := plannerState{
		Goal:        args.Goal,
		ChapterNum:  args.ChapterNum,
		BookDir:     t.bookDir,
		GatesPassed: args.GatesPassed,
	}

	var cp runtime.Checkpointer[plannerState]
	if t.db != nil {
		cp = &runtime.DBCheckpointer[plannerState]{DB: t.db}
		if loaded, err := cp.Get(ctx, topicID); err == nil {
			loaded.GatesPassed = mergeStrings(loaded.GatesPassed, state.GatesPassed)
			state = loaded
		}
	}

	graph := runtime.NewStateGraph[plannerState]()
	graph.AddNode("plan", t.planNode(args.Goal))
	graph.AddNode("execute", t.executeNode(args.ChapterNum, topicID))
	graph.AddEdge("plan", "execute")
	graph.SetEntryPoint("plan")

	compiled := graph.Compile()
	if cp != nil {
		compiled.SetCheckpointer(cp)
	}

	// If user provided new gate approvals, skip auto-loading so our merged state wins.
	skipLoad := len(args.GatesPassed) > 0
	var invokeOpts []runtime.Option
	invokeOpts = append(invokeOpts, runtime.WithThreadID(topicID))
	if skipLoad {
		invokeOpts = append(invokeOpts, runtime.WithSkipCheckpointLoad())
	}

	final, err := compiled.Invoke(ctx, state, invokeOpts...)
	if err != nil {
		if final.Error != "" {
			return "", fmt.Errorf("planner workflow failed: %s: %w", final.Error, err)
		}
		return "", fmt.Errorf("planner workflow failed: %w", err)
	}

	return final.Result, nil
}

type plannerState struct {
	Goal        string
	ChapterNum  int
	BookDir     string
	Template   *pipeline.Template
	Result     string
	Error      string
	GatesPassed []string
}

func (t *RunPlannerTool) planNode(goal string) runtime.Node[plannerState] {
	return func(ctx context.Context, state plannerState, cfg *runtime.RunConfig) (plannerState, error) {
		if state.Template != nil {
			return state, nil // already planned (resumption)
		}
		log.Info("planner: generating plan", "goal", goal)
		res, err := pipeline.RunDAGPlanner(ctx, t.client, goal)
		if err != nil {
			state.Error = err.Error()
			return state, fmt.Errorf("plan generation failed: %w", err)
		}
		state.Template = res.Template
		return state, nil
	}
}

func (t *RunPlannerTool) executeNode(chapterNum int, topicID string) runtime.Node[plannerState] {
	return func(ctx context.Context, state plannerState, cfg *runtime.RunConfig) (plannerState, error) {
		if state.Template == nil {
			state.Error = "no template to execute"
			return state, fmt.Errorf("no template to execute")
		}

		dag, err := state.Template.ToDAG()
		if err != nil {
			state.Error = err.Error()
			return state, fmt.Errorf("invalid DAG: %w", err)
		}

		var buf strings.Builder
		fmt.Fprintf(&buf, "Executing plan: %s (%d stages)\n", state.Template.Name, len(state.Template.Stages))

		input := pipeline.StageInput{
			BookDir:         t.bookDir,
			Cfg:             t.cfg,
			Client:          t.client,
			ChapterNum:      chapterNum,
			Out:             &buf,
			DB:              t.db,
			JobID:           fmt.Sprintf("%s-execute", topicID),
			GatesPassed:     state.GatesPassed,
		}

		err = pipeline.RunDAG(ctx, dag, input, func(ev pipeline.Event) {
			switch ev.Type {
			case "start":
				fmt.Fprintf(&buf, "  → %s\n", ev.StageID)
			case "done":
				fmt.Fprintf(&buf, "  ✓ %s: %s\n", ev.StageID, ev.Output.Message)
			case "error":
				fmt.Fprintf(&buf, "  ✗ %s: %v\n", ev.StageID, ev.Err)
			}
		})
		if err != nil {
			state.Error = err.Error()
			return state, fmt.Errorf("plan execution failed: %w", err)
		}

		state.Result = buf.String()
		return state, nil
	}
}

func (t *RunPlannerTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input RunPlannerInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

func mergeStrings(a, b []string) []string {
	m := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		m[s] = true
	}
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	return out
}
