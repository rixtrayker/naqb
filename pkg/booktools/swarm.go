package booktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/runtime"
)

// SpawnSwarmInput is the input for the spawn_swarm tool.
type SpawnSwarmInput struct {
	Chapters    []int `json:"chapters" jsonschema:"description=List of chapter numbers to process in parallel"`
	Concurrency int   `json:"concurrency,omitempty" jsonschema:"description=Max parallel chapters (0 = unlimited)"`
}

// SpawnSwarmTool runs multiple chapter pipelines in parallel.
type SpawnSwarmTool struct {
	spawner runtime.TaskSpawner
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
}

// NewSpawnSwarmTool creates a swarm spawn tool.
func NewSpawnSwarmTool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, client llm.Provider) runtime.Tool {
	return &SpawnSwarmTool{spawner: spawner, bookDir: bookDir, cfg: cfg, client: client}
}

func (t *SpawnSwarmTool) Name() string        { return "spawn_swarm" }
func (t *SpawnSwarmTool) Description() string { return "Run the full pipeline for multiple chapters in parallel. Returns immediately with a task ID." }
func (t *SpawnSwarmTool) Schema() any         { return nil }

func (t *SpawnSwarmTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SpawnSwarmInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if len(args.Chapters) == 0 {
		return "chapters list is required", nil
	}

	label := fmt.Sprintf("Swarm %d chapters", len(args.Chapters))
	id := t.spawner.Spawn(label, "swarm", 0, func(bgCtx context.Context) (string, error) {
		var buf strings.Builder
		in := pipeline.SwarmInput{
			BookDir:     t.bookDir,
			Cfg:         t.cfg,
			Client:      t.client,
			ChapterNums: args.Chapters,
			Out:         &buf,
			Concurrency: args.Concurrency,
		}
		res, err := pipeline.RunSwarmWithRules(bgCtx, in)
		if err != nil {
			return "", err
		}
		summary := fmt.Sprintf("Swarm complete — %d succeeded, %d failed", len(res.Results), len(res.Errors))
		if len(res.Errors) > 0 {
			var errs []string
			for ch, e := range res.Errors {
				errs = append(errs, fmt.Sprintf("ch%d: %v", ch, e))
			}
			summary += "\n" + strings.Join(errs, "\n")
		}
		return summary, nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *SpawnSwarmTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input SpawnSwarmInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}
