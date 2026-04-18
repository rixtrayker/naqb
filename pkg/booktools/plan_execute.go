package booktools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/runtime"
)

// PlanAndExecuteInput is the input for the plan_and_execute tool.
type PlanAndExecuteInput struct {
	Goal        string   `json:"goal" jsonschema:"description=High-level goal to accomplish via a multi-step plan"`
	GatesPassed []string `json:"gates_passed,omitempty" jsonschema:"description=Step IDs already approved (for resumption)"`
}

// PlanStep is a single step in an LLM-generated plan.
type PlanStep struct {
	ID          string                 `json:"id"`
	Tool        string                 `json:"tool"`
	Description string                 `json:"description"`
	Params      map[string]interface{} `json:"params"`
}

// PlanAndExecuteTool generates a plan and executes it step-by-step.
type PlanAndExecuteTool struct {
	registry *runtime.ToolRegistry
	client   llm.Provider
	db       *sql.DB
}

// NewPlanAndExecuteTool creates a plan-and-execute tool backed by a tool registry.
func NewPlanAndExecuteTool(registry *runtime.ToolRegistry, client llm.Provider, db *sql.DB) runtime.Tool {
	return &PlanAndExecuteTool{registry: registry, client: client, db: db}
}

func (t *PlanAndExecuteTool) Name() string        { return "plan_and_execute" }
func (t *PlanAndExecuteTool) Description() string { return "Break a complex goal into steps, then execute each step using available tools. Returns a trace of the plan and results. Supports checkpoint resumption if interrupted." }
func (t *PlanAndExecuteTool) Schema() any         { return nil }

func (t *PlanAndExecuteTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args PlanAndExecuteInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Goal == "" {
		return "goal is required", nil
	}

	graph := runtime.NewStateGraph[planExecuteState]()
	graph.AddNode("plan", t.planNode(args.Goal))
	graph.AddNode("execute", t.executeNode())
	graph.AddEdge("plan", "execute")
	graph.SetEntryPoint("plan")

	state := planExecuteState{
		Goal:        args.Goal,
		GatesPassed: args.GatesPassed,
	}

	compiled := graph.Compile()
	if t.db != nil {
		cp := &runtime.DBCheckpointer[planExecuteState]{
			DB:          t.db,
			Serialize:   serializePlanState,
			Deserialize: deserializePlanState,
		}
		compiled.SetCheckpointer(cp)
	}

	final, err := compiled.Invoke(ctx, state, runtime.WithThreadID("plan:"+args.Goal))
	if err != nil {
		return "", fmt.Errorf("plan_and_execute failed: %w", err)
	}
	return final.Trace, nil
}

type planExecuteState struct {
	Goal        string
	Steps       []PlanStep
	Trace       string
	GatesPassed []string
}

func (t *PlanAndExecuteTool) planNode(goal string) runtime.Node[planExecuteState] {
	return func(ctx context.Context, state planExecuteState, cfg *runtime.RunConfig) (planExecuteState, error) {
		if len(state.Steps) > 0 {
			return state, nil // already planned (resumption)
		}

		log.Info("plan_and_execute: generating plan", "goal", goal)

		var toolList strings.Builder
		for _, tool := range t.registry.List() {
			fmt.Fprintf(&toolList, "- %s: %s\n", tool.Name(), tool.Description())
		}

		systemPrompt := fmt.Sprintf(`You are an expert planner. Break the user's goal into a short sequence of steps.
Each step must use one of the available tools.

Available tools:
%s

Respond with ONLY a JSON array of steps. Each step must have:
- id: unique step identifier
- tool: exact tool name from the list above
- description: what this step does
- params: object of parameters for the tool
`, toolList.String())

		response, err := t.client.Complete(ctx, agents.ModelFor(agents.StagePlan, nil), systemPrompt, []llm.Message{
			{Role: "user", Content: goal},
		}, 0)
		if err != nil {
			return state, fmt.Errorf("plan generation failed: %w", err)
		}

		response = stripMarkdownFencesPlan(response)
		if err := json.Unmarshal([]byte(response), &state.Steps); err != nil {
			return state, fmt.Errorf("plan parse failed: %w", err)
		}
		return state, nil
	}
}

func (t *PlanAndExecuteTool) executeNode() runtime.Node[planExecuteState] {
	return func(ctx context.Context, state planExecuteState, cfg *runtime.RunConfig) (planExecuteState, error) {
		var trace strings.Builder
		fmt.Fprintf(&trace, "Plan: %s\n%d step(s)\n\n", state.Goal, len(state.Steps))

		for _, step := range state.Steps {
			tool, ok := t.registry.Resolve(step.Tool)
			if !ok {
				fmt.Fprintf(&trace, "✗ %s — unknown tool %q\n", step.ID, step.Tool)
				continue
			}

			params, _ := json.Marshal(step.Params)
			fmt.Fprintf(&trace, "→ %s: %s (%s)\n", step.ID, step.Description, step.Tool)
			result, err := tool.Invoke(ctx, string(params))
			if err != nil {
				fmt.Fprintf(&trace, "  ✗ Error: %v\n", err)
				continue
			}
			summary := result
			if len(summary) > 500 {
				summary = summary[:500] + "..."
			}
			fmt.Fprintf(&trace, "  ✓ Result: %s\n", summary)
		}

		state.Trace = trace.String()
		return state, nil
	}
}

func (t *PlanAndExecuteTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input PlanAndExecuteInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

func stripMarkdownFencesPlan(s string) string {
	s = strings.TrimPrefix(s, "```yaml")
	s = strings.TrimPrefix(s, "```yml")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func serializePlanState(state planExecuteState) ([]byte, error) {
	return json.Marshal(state)
}

func deserializePlanState(data []byte) (planExecuteState, error) {
	var state planExecuteState
	err := json.Unmarshal(data, &state)
	return state, err
}
