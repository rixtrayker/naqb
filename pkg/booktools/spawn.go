package booktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/agent"
	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/research"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// SpawnWriteInput is the input for the spawn_write tool.
type SpawnWriteInput struct {
	ChapterNum int `json:"chapter_num" jsonschema:"description=Chapter number to write"`
}

// SpawnQAInput is the input for the spawn_qa tool.
type SpawnQAInput struct {
	ChapterNum int `json:"chapter_num" jsonschema:"description=Chapter number to run QA on"`
}

// SpawnPipelineInput is the input for the spawn_pipeline tool.
type SpawnPipelineInput struct {
	ChapterNum int `json:"chapter_num" jsonschema:"description=Chapter number to run the full pipeline on"`
}

// SpawnResearchInput is the input for the spawn_research tool.
type SpawnResearchInput struct {
	ChapterNum int  `json:"chapter_num" jsonschema:"description=Chapter number to research"`
	Deep       bool `json:"deep,omitempty" jsonschema:"description=Use deep research with Gemini (default false)"`
}

// SpawnTools bundles the background-work tools.
type SpawnTools struct {
	Spawner      runtime.TaskSpawner
	BookDir      string
	Cfg          *config.BookConfig
	WriterFactory runtime.WriterFactory
	LLMClient    llm.Provider
}

// All returns the enabled spawn tools based on available dependencies.
func (s *SpawnTools) All() []runtime.Tool {
	tools := []runtime.Tool{NewSpawnWriteTool(s.Spawner, s.BookDir, s.Cfg, s.WriterFactory)}
	if s.LLMClient != nil {
		tools = append(tools,
			NewSpawnQATool(s.Spawner, s.BookDir, s.Cfg, s.LLMClient),
			NewSpawnPipelineTool(s.Spawner, s.BookDir, s.Cfg, s.LLMClient),
			NewSpawnResearchTool(s.Spawner, s.BookDir, s.Cfg, s.LLMClient),
		)
	}
	return tools
}

// SpawnWriteTool starts a chapter write in the background.
type SpawnWriteTool struct {
	spawner      runtime.TaskSpawner
	bookDir      string
	cfg          *config.BookConfig
	writerFactory runtime.WriterFactory
}

func NewSpawnWriteTool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, factory runtime.WriterFactory) runtime.Tool {
	return &SpawnWriteTool{spawner: spawner, bookDir: bookDir, cfg: cfg, writerFactory: factory}
}

func (t *SpawnWriteTool) Name() string { return "spawn_write" }
func (t *SpawnWriteTool) Description() string {
	return "Start writing a chapter in the background using the agent writer. Returns immediately with a task ID. A system message will appear when writing is complete."
}
func (t *SpawnWriteTool) Schema() any { return nil }

func (t *SpawnWriteTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SpawnWriteInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}
	label := fmt.Sprintf("Write Chapter %d: %s", args.ChapterNum, chapterTitle(t.cfg, args.ChapterNum))
	id := t.spawner.Spawn(label, "write", args.ChapterNum, func(bgCtx context.Context) (string, error) {
		writeModel := ""
		if t.cfg != nil && t.cfg.LLM.WriteModel != "" {
			writeModel = t.cfg.LLM.WriteModel
		}
		runnable := t.writerFactory.NewWriter(t.bookDir, t.cfg, writeModel)
		task := agent.BuildChapterTask(t.bookDir, t.cfg, args.ChapterNum, nil)
		rawResult, err := runnable.Invoke(bgCtx, task)
		if err != nil {
			return "", fmt.Errorf("write chapter %d: %w", args.ChapterNum, err)
		}
		result, ok := rawResult.(*runtime.AgentRunResult)
		if !ok {
			return "", fmt.Errorf("write chapter %d: unexpected result type %T", args.ChapterNum, rawResult)
		}
		wc := wordcount.Count(result.Output)
		fileWC := agent.ChapterWordCount(t.bookDir, args.ChapterNum)
		if fileWC > wc {
			wc = fileWC
		}
		return fmt.Sprintf("Chapter %d written — %d words, %d tokens used", args.ChapterNum, wc, result.TokensIn+result.TokensOut), nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *SpawnWriteTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input SpawnWriteInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

// SpawnQATool starts QA in the background.
type SpawnQATool struct {
	spawner runtime.TaskSpawner
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
}

func NewSpawnQATool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, client llm.Provider) runtime.Tool {
	return &SpawnQATool{spawner: spawner, bookDir: bookDir, cfg: cfg, client: client}
}

func (t *SpawnQATool) Name() string { return "spawn_qa" }
func (t *SpawnQATool) Description() string {
	return "Run quality assurance (deterministic + LLM audit) on a chapter in the background. Returns immediately with a task ID."
}
func (t *SpawnQATool) Schema() any { return nil }

func (t *SpawnQATool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SpawnQAInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}
	label := fmt.Sprintf("QA Chapter %d: %s", args.ChapterNum, chapterTitle(t.cfg, args.ChapterNum))
	id := t.spawner.Spawn(label, "qa", args.ChapterNum, func(bgCtx context.Context) (string, error) {
		result, err := agents.RunQA(bgCtx, t.client, t.bookDir, t.cfg, args.ChapterNum)
		if err != nil {
			return "", err
		}
		summary := result.DeterministicMsg
		if result.LLMReport != "" && result.LLMReport != "(LLM audit skipped)" {
			report := result.LLMReport
			if len(report) > 500 {
				report = report[:500] + "..."
			}
			summary += "\n\nLLM Audit:\n" + report
		}
		return summary, nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *SpawnQATool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input SpawnQAInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

// SpawnPipelineTool starts the full pipeline in the background.
type SpawnPipelineTool struct {
	spawner runtime.TaskSpawner
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
}

func NewSpawnPipelineTool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, client llm.Provider) runtime.Tool {
	return &SpawnPipelineTool{spawner: spawner, bookDir: bookDir, cfg: cfg, client: client}
}

func (t *SpawnPipelineTool) Name() string { return "spawn_pipeline" }
func (t *SpawnPipelineTool) Description() string {
	return "Run the full chapter pipeline (context → write → QA) in the background. Returns immediately with a task ID."
}
func (t *SpawnPipelineTool) Schema() any { return nil }

func (t *SpawnPipelineTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SpawnPipelineInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}
	label := fmt.Sprintf("Pipeline Chapter %d: %s", args.ChapterNum, chapterTitle(t.cfg, args.ChapterNum))
	id := t.spawner.Spawn(label, "pipeline", args.ChapterNum, func(bgCtx context.Context) (string, error) {
		var buf strings.Builder
		result, err := pipeline.RunChapterPipeline(bgCtx, t.client, t.bookDir, t.cfg, args.ChapterNum, &buf)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Pipeline complete — %d stages, %d tokens", len(result.Stages), result.TotalTokensIn()+result.TotalTokensOut()), nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *SpawnPipelineTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input SpawnPipelineInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

// SpawnResearchTool starts the research pipeline in the background.
type SpawnResearchTool struct {
	spawner runtime.TaskSpawner
	bookDir string
	cfg     *config.BookConfig
	client  llm.Provider
}

func NewSpawnResearchTool(spawner runtime.TaskSpawner, bookDir string, cfg *config.BookConfig, client llm.Provider) runtime.Tool {
	return &SpawnResearchTool{spawner: spawner, bookDir: bookDir, cfg: cfg, client: client}
}

func (t *SpawnResearchTool) Name() string { return "spawn_research" }
func (t *SpawnResearchTool) Description() string {
	return "Run the research pipeline (Scout → Explorer → Scribe) for a chapter in the background. Returns immediately with a task ID."
}
func (t *SpawnResearchTool) Schema() any { return nil }

func (t *SpawnResearchTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args SpawnResearchInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}
	label := fmt.Sprintf("Research Chapter %d: %s", args.ChapterNum, chapterTitle(t.cfg, args.ChapterNum))
	id := t.spawner.Spawn(label, "research", args.ChapterNum, func(bgCtx context.Context) (string, error) {
		rules, _ := config.LoadRules(t.bookDir)
		result, err := research.Run(bgCtx, t.client, t.bookDir, t.cfg, args.ChapterNum, rules, nil)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Research complete — %d queries, %d results, %d notes", result.Queries, result.Results, len(result.Notes)), nil
	})
	return fmt.Sprintf("Started: %s [task: %s]", label, id), nil
}

func (t *SpawnResearchTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(t.Name(), t.Description(),
		func(ctx context.Context, input SpawnResearchInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		})
}

func chapterTitle(cfg *config.BookConfig, num int) string {
	if cfg == nil {
		return ""
	}
	for _, ch := range cfg.Chapters {
		if ch.Number == num {
			return ch.Title
		}
	}
	return ""
}
