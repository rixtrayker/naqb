# Modular Architecture & LangGraph-Style Runtime Plan

## Executive Summary

This document describes the refactor that introduces a **LangGraph-style runtime** into `nqb` and the path toward making every major subsystem a **standalone Go module** (`pkg/*`).

The core insight is that `nqb` already has the *pieces* of a LangGraph system (DAG executor, agent loop, tool set, checkpoint store) but they were tightly coupled by hardcoded switches and direct imports. The refactor introduces `internal/runtime` — a generic layer for `Runnable`, `Tool`, `StateGraph`, and `Checkpointer` — and moves all concrete tool implementations into `internal/booktools`, making `agent` and `pipeline` pure orchestration packages.

---

## Part 1: What Was Done

### 1.1 New Core Abstraction: `internal/runtime`

Created a LangGraph-inspired core with zero domain-specific dependencies:

```
internal/runtime/
├── runnable.go      # Runnable interface + StreamEvent + RunConfig/Options
├── tool.go          # Tool interface + TaskSpawner interface
├── graph.go         # StateGraph[State] + CompiledGraph with Invoke/InvokeParallel
├── checkpoint.go    # Checkpointer[State] for resume/save
├── callback.go      # CallbackHandler for tracing
└── registry.go      # ToolRegistry for dynamic tool registration
```

**Key design decisions:**
- `Runnable` is the universal unit of execution. LLMs, agents, stages, and tools can all implement it.
- `StateGraph[State]` is generic. It supports linear edges, conditional routing, and parallel batch execution (topological sort).
- `Tool` is completely decoupled from `charm.land/fantasy`. The `agent` package adapts `runtime.Tool` to `fantasy.AgentTool` via a duck-type check (`interface{ FantasyTool() fantasy.AgentTool }`).

### 1.2 Decoupled Tool Layer: `internal/booktools`

Moved all concrete tool implementations out of `agent` into a dedicated package:

```
internal/booktools/
├── file.go       # read_file, write_file, list_chapters
├── research.go   # search_research, web_fetch, grep_chunks
├── knowledge.go  # knowledge_search
├── qa.go         # run_qa
├── spawn.go      # spawn_write, spawn_qa, spawn_pipeline, spawn_research
├── adapter.go    # ToFantasy() converter (bridges runtime.Tool → fantasy.AgentTool)
└── file_test.go  # unit tests
```

**What this fixes:**
- `agent` no longer imports `research`, `search`, `wordcount`, or `knowledge`.
- Tools are now first-class, composable, and testable without a full agent runtime.
- The MCP server and TUI can share the same tool definitions instead of duplicating them.

### 1.3 Refactored `agent` Package

- `agent.New` now accepts `agent.WithTools([]runtime.Tool)` instead of `WithExtraTools([]fantasy.AgentTool)`.
- `agent/tools.go` and `agent/chat_tools.go` **deleted**.
- `agent/adapter.go` added — converts `runtime.Tool` to `fantasy.AgentTool` without importing `booktools` (no cycle).
- `agent` still imports `config` and `db` for session persistence. These will be abstracted in Phase C.

### 1.4 Refactored `pipeline` Executor

- Added `pipeline/registry.go` with `RegisterStage` / `ResolveStage`.
- Built-in stages register themselves in an `init()` block in `pipeline.go`.
- `executor.go` no longer has a hardcoded `switch` on `StageType`. It resolves stages dynamically from the registry.
- **This is the prerequisite for custom stages** — users will eventually be able to register their own `Stage` implementations without editing `pipeline` source code.

### 1.5 Updated Callers

| Caller | Change |
|--------|--------|
| `commands/open.go` | No longer builds `agent.BackgroundRunners`. Simply passes `llmClient` to `tui.RunAgentChat`. |
| `tui/screen_agentchat.go` | Builds the full `[]runtime.Tool` inside `AgentChatModel` using `m.tracker` as the spawner. |
| `jobs/worker.go` | `runWriteJob` now constructs `[]runtime.Tool` via `booktools` so the queued agent has file access. |

---

## Part 2: Target Standalone Module Layout

The end-state is to promote `internal/runtime`, `internal/booktools`, `internal/agent`, and `internal/pipeline` to top-level `pkg/` modules so they can be versioned and imported independently.

```
pkg/
├── runtime/          # LangGraph-style core (Runnable, StateGraph, Tool, Checkpointer)
│   └── go.mod        # no external deps except stdlib
├── llm/              # Provider interface + resilience wrappers
│   └── go.mod        # depends on runtime (for Runnable adapters)
├── agent/            # Fantasy-based agent loop
│   └── go.mod        # depends on runtime, llm
├── pipeline/         # Stage registry + DAG executor
│   └── go.mod        # depends on runtime, llm
├── booktools/        # Concrete nqb tools
│   └── go.mod        # depends on runtime, agent, pipeline, research, search...
├── research/         # Scout → Explorer → Scribe
│   └── go.mod        # depends on runtime (for future Runnable nodes)
├── store/            # Vector + keyword + hybrid stores
│   └── go.mod        # no domain deps
└── knowledge/        # Claims graph + epistemic state
    └── go.mod        # depends on runtime (for Checkpointer interface)
```

### Dependency Rules

```
runtime  ←  llm, agent, pipeline, knowledge, research
llm      ←  agent, pipeline
agent    ←  pipeline (for agentic write stage), booktools
pipeline ←  booktools (for stage implementations), agent (for agentic write stage)
booktools ← agent (spawn_write creates a sub-agent), pipeline, research, search
```

The only "messy" edge is `booktools` → `agent` because `spawn_write` creates a sub-agent. To break this, Phase D (below) will introduce a `WriterFactory` interface.

---

## Part 3: LangGraph-Style Capabilities Enabled

### 3.1 Stateful Pipeline Execution

Currently `pipeline.Run` passes `StageInput` to every stage, and stages communicate via **side effects** (writing files). With `runtime.StateGraph`, we can accumulate state in a typed struct:

```go
type BookState struct {
    BookDir      string
    Cfg          BookConfig
    ChapterNum   int
    Context      string
    Draft        string
    QAReport     string
    ConflictReport string
    GapReport    string
    TokenUsage   TokenCount
}

graph := runtime.NewStateGraph[BookState]()
graph.AddNode("context",  contextNode)
graph.AddNode("write",    writeNode)   // agentic or legacy
graph.AddNode("qa",       qaNode)
graph.AddNode("save",     saveNode)    // writes artifacts to disk
graph.Compile().Invoke(ctx, state)
```

**Benefits:**
- Pure nodes: `writeNode` returns `state.Draft` instead of writing `chapters/ch-01.md`.
- Testability: invoke the graph and assert on `state.Draft` without touching the filesystem.
- Replay / resume: a `Checkpointer[BookState]` can serialize state after any node.
- Subgraphs: `spawn_pipeline` becomes a simple `graph.Invoke(ctx, state, runtime.WithThreadID(taskID))`.

### 3.2 Dynamic Tool Registry

The `runtime.ToolRegistry` allows tools to be registered at runtime:

```go
registry := runtime.NewToolRegistry()
registry.Register(booktools.NewReadFileTool(bookDir))
registry.Register(booktools.NewWebFetchTool())
// ... later, MCP server auto-registers from this registry
```

This replaces the hardcoded `registerTools()` block in `mcpserver/server.go`.

### 3.3 Checkpointing (Memory / Resume)

The `runtime.Checkpointer[State]` interface maps directly onto the existing SQLite `db` package:

```go
type DBCheckpointer[State any] struct{ DB *sql.DB }

func (c *DBCheckpointer[State]) Get(ctx context.Context, threadID string) (State, error) {
    // load from sessions / stage_progress tables
}
func (c *DBCheckpointer[State]) Put(ctx context.Context, threadID string, state State) error {
    // serialize and upsert
}
```

This gives us **LangGraph-style thread resumption** for both agent chats and batch pipelines.

### 3.4 Callback / Tracing

`runtime.CallbackHandler` provides a unified observability hook:

```go
type TracingCallback struct{}
func (t *TracingCallback) OnNodeStart(ctx context.Context, nodeID string, state any) {
    slog.Info("node start", "node", nodeID)
}
func (t *TracingCallback) OnToolStart(ctx context.Context, toolName string, input string) {
    slog.Info("tool call", "tool", toolName, "input", input)
}
```

This replaces the ad-hoc `Event` struct in `pipeline/executor.go` and the `onDelta` callback in `agent.Run`.

---

## Part 4: Recommended Next Steps (Phased Roadmap)

### Phase A — Extract `pkg/runtime` and `pkg/llm` (Foundation)

1. **Move `internal/runtime` → `pkg/runtime`**.
   - It has zero external deps; this is the easiest standalone module.
2. **Move `internal/llm` → `pkg/llm`**.
   - Extract the `Provider` interface, `Message`, `StreamFunc`.
   - Move concrete providers to `pkg/llm/openrouter`, `pkg/llm/anthropic`, `pkg/llm/bedrock`.
   - Move resilience wrappers to `pkg/llm/wrap`.
   - **Remove globals**: `SessionBudget` and `ActivePricingTier` become fields on a `Session` struct passed to runners.

### Phase B — Rewrite Pipeline on `StateGraph`

1. Define `PipelineState` (typed state object).
2. Convert each `Stage` into a `Node[PipelineState]`.
3. Rewrite `RunDAG` to use `runtime.StateGraph[PipelineState].Compile().InvokeParallel()`.
4. Make stages pure: return content in state; move file I/O to a `saveNode` or `ProjectStore` adapter.
5. **Delete `BackgroundRunners` entirely**: `spawn_pipeline` calls the compiled pipeline graph directly.

### Phase C — Abstract `config` and `db` from `agent` and `pipeline`

1. **Agent persistence**: replace direct `db.*` calls with a `SessionStore` interface.
   ```go
   type SessionStore interface {
       CreateSession(id, bookDir string) error
       AppendMessage(msgID, sessionID, role, content string, tokensIn, tokensOut int) error
   }
   ```
2. **Agent context**: replace `*config.BookConfig` with a minimal `ProjectContext` struct (title, author, domain, language, synopsis, chapters).
3. **Pipeline context**: same abstraction — `PipelineInput` should not depend on the full `config` package.
4. Once these are done, `agent` and `pipeline` can move to `pkg/agent` and `pkg/pipeline`.

### Phase D — Break `booktools` → `agent` Import Cycle

`booktools/spawn.go` imports `agent` for `spawn_write`. To make `booktools` standalone:

1. Define a `WriterFactory` interface in `runtime` or `agent`:
   ```go
   type WriterFactory interface {
       NewWriter(bookDir string, cfg *config.BookConfig) runtime.Runnable
   }
   ```
2. `SpawnWriteTool` accepts a `WriterFactory` instead of calling `agent.New` directly.
3. `commands` wires the real `agent.WriterFactory` at startup.
4. Now `booktools` no longer imports `agent`. It can move to `pkg/booktools`.

### Phase E — Unify Search Layers

1. Make `internal/search` a thin facade over `internal/store`.
2. Redirect `search.Open`, `search.IndexChapter`, etc. to `store.HybridStore`.
3. Remove duplicate chromem logic from `search`.
4. Eventually delete `internal/search` and have callers import `store` directly.

### Phase F — MCP Server Auto-Registration

1. Refactor `mcpserver/server.go` to accept a `*runtime.ToolRegistry`.
2. Iterate over `registry.List()` and generate MCP tool schemas dynamically.
3. Remove the hardcoded `registerTools()` block.
4. This makes every `runtime.Tool` instantly available to Claude Desktop / any MCP host.

### Phase G — DeepAgents-Style Composition

With the runtime in place, we can add higher-order patterns:

- **`PlanAndExecute`**: an agent node that emits a sub-graph plan, followed by an executor node that runs it.
- **`Reflection`**: a `reviewNode` → `conditionalEdge` → `rewriteNode` loop.
- **`Human-in-the-loop`**: use `runtime.StateGraph` with a gate node that returns an `Interrupt` error and resumes via checkpointer.
- **`Multi-agent swarms`**: spawn parallel sub-graphs per chapter and merge results.

---

## Part 5: File Inventory (What Changed)

### New packages
- `internal/runtime/*` (6 files)
- `internal/booktools/*` (7 files)
- `internal/pipeline/registry.go`
- `internal/agent/adapter.go`

### Deleted files
- `internal/agent/tools.go`
- `internal/agent/chat_tools.go`
- `internal/agent/tools_test.go`
- `internal/agent/chat_tools_test.go`

### Modified files
- `internal/agent/agent.go`
- `internal/tui/screen_agentchat.go`
- `internal/commands/open.go`
- `internal/jobs/worker.go`
- `internal/pipeline/executor.go`
- `internal/pipeline/pipeline.go`

---

## Part 6: Alternative Considered — River Queue (Rejected for Now)

We evaluated [River](https://riverqueue.com/) as a potential backbone for durable job queues, parallel subagent workers, retries, and dead-lettering.

### Why River is attractive
- **Durable queues** with exactly-once semantics inside PostgreSQL transactions.
- **Built-in retries, exponential backoff, and dead-lettering**.
- **Worker pools** that give us parallel subagent execution with minimal code.
- **Observability** (dashboard, metrics, job-state inspection).
- **Sub-job / batching APIs** map naturally to "spawn N subagents in parallel".

### Why we are not adopting it now
1. **Hard dependency on PostgreSQL**. `nqb` is currently SQLite-native: `internal/db`, vector/keyword search (`sqlite-vec` / `sqlite-fts5`), session storage, and checkpoint tables all live in SQLite. Adding Postgres would introduce a second persistent store and complicate local installs, backups, and self-hosting.
2. **River is a job queue, not a state-graph engine**. We would still need `runtime.StateGraph` and `Checkpointer` for conditional routing, planner-generated DAGs, and inter-node checkpoints. River would sit *alongside* our runtime rather than replace it.
3. **Current scale does not justify the ops overhead**. The project is a local-first CLI; SQLite + a lightweight worker poller is sufficient for the foreseeable future.

### Decision
**Stay SQLite-only for now.** We will:
- Implement `DBCheckpointer` on SQLite.
- Build a lightweight job-queue worker (`internal/jobs`) on top of SQLite if parallel background execution is needed.
- Keep `runtime.StateGraph`, `Runnable`, and `ToolRegistry` queue-agnostic.

### Future migration path
If the project later requires River-level durability (e.g., multi-user hosted mode, massive parallel workloads), the migration is straightforward because our abstractions are decoupled:

```
Planner agent → emits DAG plan
River client  → inserts N sub-jobs (one per subagent/task)
River worker  → executes sub-job by calling runtime.StateGraph.Invoke()
DBCheckpointer (now backed by PG) → resumes intermediate state inside each sub-job
```

The only required change would be adding a `RiverAdapter` that implements the same `Spawner` or `TaskQueue` interface we already use in `runtime`. No rewrite of `agent`, `pipeline`, or `booktools` would be necessary.

---

## Appendix: Quick Migration Guide for Future Contributors

### Adding a new agent tool

1. Create a struct in `internal/booktools/<category>.go` that implements `runtime.Tool`.
2. Optionally implement `FantasyToolProvider` so the adapter preserves JSON schema.
3. Import it in `commands/open.go` (or your app-layer wire-up) and pass it to `agent.WithTools`.

### Adding a new pipeline stage

1. Implement the `pipeline.Stage` interface in a new file.
2. Call `pipeline.RegisterStage(StageTypeCustom, func() Stage { return &MyStage{} })` in an `init()`.
3. Reference `StageTypeCustom` in your DAG template YAML.

### Running a subgraph from a tool

```go
graph := runtime.NewStateGraph[MyState]()
// ... add nodes/edges
compiled := graph.Compile()
compiled.SetCheckpointer(myCheckpointer)
nextState, err := compiled.Invoke(ctx, state, runtime.WithThreadID("task-123"))
```

This is the pattern that replaces `BackgroundRunners` and enables arbitrarily complex, resumable, observable workflows.
