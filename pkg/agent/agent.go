package agent

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/google/uuid"
)

// RunResult is returned by Agent.Run.
type RunResult struct {
	// Output is the final text response from the agent.
	Output string
	// SessionID is the database session ID (new or continuation of existing).
	SessionID string
	// TokensIn is the total input token count across all steps.
	TokensIn int64
	// TokensOut is the total output token count across all steps.
	TokensOut int64
	// Steps is the number of agentic steps taken.
	Steps int
}

// Agent wraps a fantasy provider+model, a book project context, and
// optional session/epistemic stores for persistence.
type Agent struct {
	provider  fantasy.Provider
	modelID   string
	sessions  runtime.SessionStore
	epistemic runtime.EpistemicStore
	bookDir   string
	cfg       *config.BookConfig
	analysis  *ProjectAnalysis
	tools     []runtime.Tool
}

// AgentOption configures optional Agent behaviour.
type AgentOption func(*Agent)

// WithAnalysis injects a ProjectAnalysis into the agent's system prompt.
func WithAnalysis(a *ProjectAnalysis) AgentOption {
	return func(ag *Agent) { ag.analysis = a }
}

// WithTools replaces the agent's tool set with the given runtime tools.
func WithTools(tools []runtime.Tool) AgentOption {
	return func(ag *Agent) { ag.tools = tools }
}

// WithSessionStore injects a session store for chat persistence.
func WithSessionStore(store runtime.SessionStore) AgentOption {
	return func(ag *Agent) { ag.sessions = store }
}

// WithEpistemicStore injects an epistemic store for knowledge-graph summaries.
func WithEpistemicStore(store runtime.EpistemicStore) AgentOption {
	return func(ag *Agent) { ag.epistemic = store }
}

// New creates an Agent.
func New(provider fantasy.Provider, modelID string, bookDir string, cfg *config.BookConfig, opts ...AgentOption) *Agent {
	a := &Agent{
		provider: provider,
		modelID:  modelID,
		bookDir:  bookDir,
		cfg:      cfg,
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Run executes an agentic loop for the given task, optionally continuing an
// existing session. Pass an empty sessionID to start a new session.
//
// onDelta is called with each streamed text chunk (can be nil for non-interactive use).
//
// If a session store is wired in, all messages and token counts are persisted.
func (a *Agent) Run(ctx context.Context, task, sessionID string, onDelta func(string)) (*RunResult, error) {
	// ── Model ────────────────────────────────────────────────────────────────
	model, err := a.provider.LanguageModel(ctx, a.modelID)
	if err != nil {
		return nil, fmt.Errorf("agent: language model %q: %w", a.modelID, err)
	}

	// ── Session ──────────────────────────────────────────────────────────────
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
	if a.sessions != nil {
		if err := a.sessions.CreateSession(ctx, sessionID, a.bookDir, 0); err != nil {
			// Ignore duplicate key (session already exists for continuation).
			errMsg := strings.ToUpper(err.Error())
			if !strings.Contains(errMsg, "UNIQUE") && !strings.Contains(errMsg, "CONSTRAINT") {
				return nil, fmt.Errorf("agent: create session: %w", err)
			}
		}
		// Persist user message
		msgID := uuid.NewString()
		if err := a.sessions.AppendMessage(ctx, msgID, sessionID, "user", task, "", 0, 0); err != nil {
			return nil, fmt.Errorf("agent: persist user message: %w", err)
		}
	}

	// ── System prompt ────────────────────────────────────────────────────────
	systemPrompt := a.buildEnrichedSystemPrompt()

	// ── Fantasy agent ────────────────────────────────────────────────────────
	ftools := toFantasyTools(a.tools)
	fantasyAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(systemPrompt),
		fantasy.WithTools(ftools...),
		fantasy.WithMaxRetries(2),
	)

	// ── Streaming ────────────────────────────────────────────────────────────
	result := &RunResult{SessionID: sessionID}
	var outputBuf strings.Builder
	var stepCount int

	streamCall := fantasy.AgentStreamCall{
		Prompt: task,
		OnTextDelta: func(id, text string) error {
			outputBuf.WriteString(text)
			if onDelta != nil {
				onDelta(text)
			}
			return nil
		},
		OnStepFinish: func(stepResult fantasy.StepResult) error {
			stepCount++
			result.TokensIn += stepResult.Usage.InputTokens
			result.TokensOut += stepResult.Usage.OutputTokens

			// Persist assistant message to DB
			if a.sessions != nil {
				content := stepResult.Content.Text()
				msgID := uuid.NewString()
				if err := a.sessions.AppendMessage(ctx, msgID, sessionID, "assistant", content,
					a.modelID, int(stepResult.Usage.InputTokens), int(stepResult.Usage.OutputTokens)); err != nil {
					log.Warn("agent: persist assistant message failed", "session", sessionID, "err", err)
				}
				if err := a.sessions.TouchSession(ctx, sessionID); err != nil {
					log.Warn("agent: touch session failed", "session", sessionID, "err", err)
				}
			}
			return nil
		},
		OnError: func(err error) {
			log.Warn("agent step error", "session", sessionID, "err", err)
			if a.sessions != nil {
				msgID := uuid.NewString()
				if dbErr := a.sessions.AppendMessage(ctx, msgID, sessionID, "error", err.Error(),
					a.modelID, 0, 0); dbErr != nil {
					log.Warn("agent: persist error message failed", "session", sessionID, "err", dbErr)
				}
			}
		},
	}

	agentResult, err := fantasyAgent.Stream(ctx, streamCall)
	if err != nil {
		return nil, fmt.Errorf("agent: stream: %w", err)
	}

	// Collect final totals from the agent result (includes all steps)
	if agentResult != nil {
		result.TokensIn = agentResult.TotalUsage.InputTokens
		result.TokensOut = agentResult.TotalUsage.OutputTokens
		result.Steps = len(agentResult.Steps)
		if agentResult.Response.Content != nil {
			result.Output = agentResult.Response.Content.Text()
		}
	} else {
		result.Output = outputBuf.String()
		result.Steps = stepCount
	}

	return result, nil
}

// buildEnrichedSystemPrompt constructs the system prompt, optionally enriching
// it with the ProjectAnalysis and background tool descriptions.
func (a *Agent) buildEnrichedSystemPrompt() string {
	hasSpawn := false
	for _, t := range a.tools {
		if t.Name() == "spawn_write" || t.Name() == "spawn_qa" || t.Name() == "spawn_pipeline" || t.Name() == "spawn_research" {
			hasSpawn = true
			break
		}
	}
	if a.analysis != nil {
		return buildAnalysisSystemPrompt(a.analysis, hasSpawn)
	}
	return buildSystemPrompt(a.bookDir, a.cfg)
}

// Invoke implements runtime.Runnable for non-streaming agent execution.
// input must be a string (the task prompt).
func (a *Agent) Invoke(ctx context.Context, input any, opts ...runtime.Option) (any, error) {
	task, _ := input.(string)
	res, err := a.Run(ctx, task, "", nil)
	if err != nil {
		return nil, err
	}
	return &runtime.AgentRunResult{
		Output:    res.Output,
		SessionID: res.SessionID,
		TokensIn:  res.TokensIn,
		TokensOut: res.TokensOut,
		Steps:     res.Steps,
	}, nil
}

// Stream implements runtime.Runnable (not yet supported; use Run with onDelta).
func (a *Agent) Stream(ctx context.Context, input any, opts ...runtime.Option) (<-chan runtime.StreamEvent, error) {
	return nil, fmt.Errorf("agent streaming via Stream() not yet implemented; use Run() with onDelta")
}

// buildAnalysisSystemPrompt builds a rich system prompt using project analysis data.
func buildAnalysisSystemPrompt(analysis *ProjectAnalysis, hasSpawnTools bool) string {
	var sb strings.Builder
	sb.WriteString("You are نقب (naqb), an expert AI book-writing assistant.\n\n")
	sb.WriteString(analysis.SystemPromptSection())

	sb.WriteString("\n## Your Role\n")
	sb.WriteString("You are an expert author and editor helping to write, refine, and manage this book.\n")
	sb.WriteString("Use the available tools to read files, write chapters, search research notes, and fetch web content as needed.\n")
	sb.WriteString("Always work within the book directory — never access files outside the project.\n")

	sb.WriteString("\n## File Tools\n")
	sb.WriteString("- **read_file**: Read any file in the project\n")
	sb.WriteString("- **write_file**: Write content to a file (creates backup)\n")
	sb.WriteString("- **search_research**: Search research notes by query\n")
	sb.WriteString("- **run_qa**: Run quality checks on a chapter\n")
	sb.WriteString("- **web_fetch**: Fetch a URL for research enrichment\n")
	sb.WriteString("- **list_chapters**: List chapters with word counts\n")
	sb.WriteString("- **knowledge_search**: Search knowledge graph and indexed chunks\n")
	sb.WriteString("- **grep_chunks**: Keyword search across indexed chunks\n")

	if hasSpawnTools {
		sb.WriteString("\n## Background Work Tools\n")
		sb.WriteString("Use these to run heavy operations in the background. They return immediately with a task ID.\n")
		sb.WriteString("When a task completes, a system message will appear with the results.\n\n")
		sb.WriteString("- **spawn_write**: Write a chapter using the agent writer\n")
		sb.WriteString("- **spawn_qa**: Run QA (deterministic + LLM audit) on a chapter\n")
		sb.WriteString("- **spawn_pipeline**: Run the full pipeline (context → write → QA)\n")
		sb.WriteString("- **spawn_research**: Run research pipeline (Scout → Explorer → Scribe)\n")
		sb.WriteString("\nYou can start multiple background tasks in parallel.\n")
	}

	return sb.String()
}

// buildSystemPrompt constructs the system prompt injecting book context.
func buildSystemPrompt(bookDir string, cfg *config.BookConfig) string {
	if cfg == nil {
		return "You are نقب (naqb), an expert AI book-writing assistant. You have access to tools for reading/writing files, searching research notes, and fetching web content."
	}

	return fmt.Sprintf(`You are نقب (naqb), an expert AI book-writing assistant.

## Book Project
- **Title:** %s
- **Author:** %s
- **Domain:** %s
- **Language:** %s
- **Synopsis:** %s

## Your Role
You are an expert author and editor helping to write, refine, and manage this book.
Use the available tools to read files, write chapters, search research notes, and fetch web content as needed.
Always work within the book directory — never access files outside the project.

## Tools Available
- **read_file**: Read any file in the project (chapters, contexts, outline, research notes)
- **write_file**: Write content to a file (creates backup automatically)
- **search_research**: Search research notes by semantic/keyword query
- **run_qa**: Run quality checks on a chapter
- **web_fetch**: Fetch a URL for research enrichment
- **list_chapters**: List all chapters with word counts and draft status`,
		cfg.Title, cfg.Author, cfg.Domain, cfg.Language, cfg.Synopsis,
	)
}
