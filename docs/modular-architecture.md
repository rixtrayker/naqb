# Modular Architecture & LangGraph-Style Runtime Plan

> **Status: ✅ COMPLETE** — All major subsystems have been promoted to standalone `pkg/*` Go modules.

## Executive Summary

This document describes the refactor that introduced a **LangGraph-style runtime** into `nqb` and made every major subsystem a **standalone Go module** (`pkg/*`).

The core insight was that `nqb` already had the *pieces* of a LangGraph system (DAG executor, agent loop, tool set, checkpoint store) but they were tightly coupled by hardcoded switches and direct imports. The refactor introduced `pkg/runtime` — a generic layer for `Runnable`, `Tool`, `StateGraph`, `Checkpointer`, `Registry`, and `Interrupt` — and moved all concrete tool implementations into `pkg/booktools`, making `agent`, `agents`, and `pipeline` pure orchestration packages.

---

## Part 1: What Was Done

### 1.1 New Core Abstraction: `pkg/runtime`

Created a LangGraph-inspired core with zero domain-specific dependencies:

```
pkg/runtime/
├── runnable.go      # Runnable interface + StreamEvent + RunConfig/Options
├── tool.go          # Tool interface + TaskSpawner interface
├── graph.go         # StateGraph[State] + CompiledGraph with Invoke/InvokeParallel
├── checkpoint.go    # Checkpointer[State] + SessionStore/EpistemicStore interfaces
├── interrupt.go     # Interrupt for human-in-the-loop gating
├── callback.go      # CallbackHandler for tracing
└── registry.go      # ToolRegistry for dynamic tool registration
```

**Key design decisions:**
- `Runnable` is the universal unit of execution. LLMs, agents, stages, and tools can all implement it.
- `StateGraph[State]` is generic. It supports linear edges, conditional routing, parallel batch execution (topological sort), and interruption.
- `Tool` is completely decoupled from `charm.land/fantasy`. The `agent` package adapts `runtime.Tool` to `fantasy.AgentTool` via a duck-type check (`interface{ FantasyTool() fantasy.AgentTool }`).
- `SessionStore` and `EpistemicStore` interfaces live in `runtime`, decoupling `agent` from `db` and `knowledge`.

### 1.2 Decoupled Tool Layer: `pkg/booktools`

Moved all concrete tool implementations out of `agent` into a dedicated package:

```
pkg/booktools/
├── file.go          # read_file, write_file, list_chapters
├── research.go      # search_research, web_fetch, grep_chunks
├── knowledge.go     # knowledge_search
├── qa.go            # run_qa
├── spawn.go         # spawn_write, spawn_qa, spawn_pipeline, spawn_research
├── adapter.go       # ToFantasy() converter (bridges runtime.Tool → fantasy.AgentTool)
├── planner.go       # Plan tool for agentic planning
├── plan_execute.go  # Plan-and-execute agent pattern
├── reflection.go    # Reflection tool for self-review
├── swarm.go         # Swarm coordination primitives
└── file_test.go     # unit tests
```

**What this fixes:**
- `agent` no longer imports `research`, `search`, `wordcount`, or `knowledge`.
- Tools are now first-class, composable, and testable without a full agent runtime.
- The MCP server and TUI can share the same tool definitions instead of duplicating them.

### 1.3 Refactored `pkg/agent` Package

- `agent.New` now accepts `agent.WithTools([]runtime.Tool)` instead of `WithExtraTools([]fantasy.AgentTool)`.
- `agent` uses `runtime.SessionStore` and `runtime.EpistemicStore` interfaces — no direct `db` or `knowledge` imports for persistence.
- `agent/tools.go` and `agent/chat_tools.go` **deleted**.
- `agent/adapter.go` added — converts `runtime.Tool` to `fantasy.AgentTool` without importing `booktools` (no cycle).
- `pkg/agents/` still houses legacy single-shot orchestration (WriteChapter, RunQA, etc.) as a separate module.

### 1.4 Refactored `pkg/pipeline` Executor

- Added `pipeline/registry.go` with `RegisterStage` / `ResolveStage`.
- Built-in stages register themselves in an `init()` block in `pipeline.go`.
- `executor.go` no longer has a hardcoded `switch` on `StageType`. It resolves stages dynamically from the registry.
- Pipeline now runs on `runtime.StateGraph[PipelineState]` with pure nodes, reflection loops, and swarm stages.
- **This enables custom stages** — users can register their own `Stage` implementations without editing `pipeline` source code.

### 1.5 Updated Callers

| Caller | Change |
|--------|--------|
| `commands/open.go` | No longer builds `agent.BackgroundRunners`. Simply passes `llmClient` to `tui.RunAgentChat`. |
| `tui/screen_agentchat.go` | Builds the full `[]runtime.Tool` inside `AgentChatModel` using `m.tracker` as the spawner. |
| `jobs/worker.go` | `runWriteJob` now constructs `[]runtime.Tool` via `booktools` so the queued agent has file access. |

### 1.6 Workspace & Module Setup

- `go.work` at the root ties all `pkg/*` modules together.
- Root `go.mod` uses `replace` directives for local development of every `pkg/*` module.
- Each `pkg/*` directory has its own `go.mod` and is independently versionable.

---

## Part 2: Target Standalone Module Layout ✅ DONE

All major subsystems have been promoted to `pkg/*`. The tree below reflects what actually exists.

```
pkg/
├── runtime/          # ✅ LangGraph-style core (Runnable, StateGraph, Tool, Checkpointer, Registry, Interrupt)
│   └── go.mod        # minimal deps
├── llm/              # ✅ Provider interface + resilience wrappers (retry, fallback, race, circuit-breaker)
│   └── go.mod        # depends on runtime
├── agent/            # ✅ Fantasy-based agent loop with SessionStore/EpistemicStore interfaces
│   └── go.mod        # depends on runtime, llm
├── agents/           # ✅ Legacy single-shot orchestration (WriteChapter, RunQA, etc.)
│   └── go.mod        # depends on llm, config
├── pipeline/         # ✅ Stage registry + DAG executor + swarm + reflection
│   └── go.mod        # depends on runtime, llm, agent
├── booktools/        # ✅ Concrete nqb tools (file, research, knowledge, qa, spawn, adapter, plan_execute, planner, reflection, swarm)
│   └── go.mod        # depends on runtime, agent, pipeline, research, search...
├── research/         # ✅ Scout → Explorer → Scribe (stub/minimal)
│   └── go.mod        # depends on runtime
├── search/           # ✅ Vector + keyword store wrappers
│   └── go.mod        # no domain deps
├── wordcount/        # ✅ Word counting utilities
│   └── go.mod        # no external deps
├── youtube/          # ✅ YouTube transcript fetching
│   └── go.mod        # minimal deps
├── config/           # ✅ Config structs + loading
│   └── go.mod        # minimal deps
└── log/              # ✅ Structured logging
    └── go.mod        # stdlib only
```

**Remaining internal packages** (not yet extracted to `pkg/`):
- `internal/store/` — Hybrid vector + keyword store implementations (chromem, bleve, etc.)
- `internal/knowledge/` — Claims graph + epistemic state management
- `internal/db/` — SQLite database layer
- `internal/jobs/` — SQLite-backed async job queue
- `internal/tui/` — Bubble Tea TUI
- `internal/commands/` — Cobra CLI commands
- `internal/mcpserver/` — MCP server

These are application-layer concerns that are appropriately kept in `internal/`.

### Dependency Rules

```
runtime   ←  llm, agent, pipeline, research, search
llm       ←  agent, pipeline, agents
agent     ←  pipeline (for agentic write stage), booktools
pipeline  ←  booktools (for stage implementations), agent (for agentic write stage)
booktools ←  agent (spawn_write creates a sub-agent), pipeline, research, search
```

There are no import cycles. The `booktools → agent` edge is one-way (`agent` does not import `booktools`). The Go workspace (`go.work`) and root `replace` directives make local development seamless across all modules.

---

## Part 3: LangGraph-Style Capabilities Enabled

### 3.1 Stateful Pipeline Execution ✅ IMPLEMENTED

`pipeline.Run` now accumulates state in a typed `PipelineState` struct carried through `runtime.StateGraph`:

```go
type PipelineState struct {
    BookDir        string
    Cfg            BookConfig
    ChapterNum     int
    Context        string
    Draft          string
    QAReport       string
    ConflictReport string
    GapReport      string
    TokenUsage     TokenCount
}

graph := runtime.NewStateGraph[PipelineState]()
graph.AddNode("context",  contextNode)
graph.AddNode("write",    writeNode)   // agentic or legacy
graph.AddNode("qa",       qaNode)
graph.AddNode("save",     saveNode)    // writes artifacts to disk
graph.Compile().Invoke(ctx, state)
```

**Benefits:**
- Pure nodes: `writeNode` returns `state.Draft` instead of writing `chapters/ch-01.md`.
- Testability: invoke the graph and assert on `state.Draft` without touching the filesystem.
- Replay / resume: a `Checkpointer[PipelineState]` can serialize state after any node.
- Subgraphs: `spawn_pipeline` becomes a simple `graph.Invoke(ctx, state, runtime.WithThreadID(taskID))`.

### 3.2 Dynamic Tool Registry ✅ IMPLEMENTED

The `runtime.ToolRegistry` allows tools to be registered at runtime:

```go
registry := runtime.NewToolRegistry()
registry.Register(booktools.NewReadFileTool(bookDir))
registry.Register(booktools.NewWebFetchTool())
// ... later, MCP server auto-registers from this registry
```

This replaces the hardcoded `registerTools()` block in `mcpserver/server.go` (MCP auto-registration is planned — see Phase F).

### 3.3 Checkpointing (Memory / Resume) ✅ INTERFACES DEFINED, PARTIALLY IMPLEMENTED

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

This gives us **LangGraph-style thread resumption** for both agent chats and batch pipelines. A SQLite-backed implementation exists in `pkg/runtime/checkpoint_sqlite.go`.

### 3.4 Callback / Tracing ✅ IMPLEMENTED

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

### Phase A — Extract `pkg/runtime` and `pkg/llm` (Foundation) ✅ DONE

1. **Move `internal/runtime` → `pkg/runtime`** — Done. Zero external deps.
2. **Move `internal/llm` → `pkg/llm`** — Done. Provider interface, concrete providers (openrouter, anthropic, bedrock), and resilience wrappers (retry, fallback, race, circuit-breaker) all extracted.
3. **Remove globals**: `SessionBudget` and `ActivePricingTier` are now fields on session structs passed to runners.

### Phase B — Rewrite Pipeline on `StateGraph` ✅ DONE

1. Define `PipelineState` (typed state object) — Done.
2. Convert each `Stage` into a `Node[PipelineState]` — Done.
3. Rewrite `RunDAG` to use `runtime.StateGraph[PipelineState].Compile().InvokeParallel()` — Done.
4. Make stages pure: return content in state; move file I/O to a `saveNode` — Done.
5. **Delete `BackgroundRunners` entirely**: `spawn_pipeline` calls the compiled pipeline graph directly — Done.
6. Add reflection graph (`NewReflectionGraph`) for write → review → rewrite loops — Done.
7. Add swarm stages for parallel sub-agent execution — Done.

### Phase C — Abstract `config` and `db` from `agent` and `pipeline` ✅ MOSTLY DONE

1. **Agent persistence**: direct `db.*` calls replaced with `runtime.SessionStore` interface — Done.
   ```go
   type SessionStore interface {
       CreateSession(ctx context.Context, id, bookDir string, chapterNum int) error
       AppendMessage(ctx context.Context, msgID, sessionID, role, content, model string, tokensIn, tokensOut int) error
       // ...
   }
   ```
2. **Agent context**: `*config.BookConfig` is still used directly. A minimal `ProjectContext` abstraction is still possible for future decoupling.
3. **Pipeline context**: same — `PipelineInput` still depends on `config.BookConfig`.
4. `agent` and `pipeline` are now in `pkg/agent` and `pkg/pipeline`.

### Phase D — Break `booktools` → `agent` Import Cycle ✅ RESOLVED

`booktools/spawn.go` imports `pkg/agent` for `spawn_write`. There is **no cycle** — `pkg/agent` does not import `pkg/booktools`. The `booktools` module is already standalone. A future `WriterFactory` interface could remove even this one-way dependency, but it is not blocking.

### Phase E — Unify Search Layers 🔄 PLANNED

1. Make `pkg/search` the canonical facade over `internal/store`.
2. Redirect `search.Open`, `search.IndexChapter`, etc. to `store.HybridStore`.
3. Remove duplicate chromem logic from any remaining `internal/search*` packages.
4. Eventually consider promoting `internal/store` → `pkg/store` if needed externally.

### Phase F — MCP Server Auto-Registration 🔄 PLANNED

1. Refactor `mcpserver/server.go` to accept a `*runtime.ToolRegistry`.
2. Iterate over `registry.List()` and generate MCP tool schemas dynamically.
3. Remove the hardcoded `registerTools()` block.
4. This makes every `runtime.Tool` instantly available to Claude Desktop / any MCP host.

### Phase G — DeepAgents-Style Composition 🔄 PLANNED

With the runtime in place, we can add higher-order patterns:

- **`PlanAndExecute`**: an agent node that emits a sub-graph plan, followed by an executor node that runs it. (`pkg/booktools/plan_execute.go` already has initial support.)
- **`Reflection`**: a `reviewNode` → `conditionalEdge` → `rewriteNode` loop. (`pkg/pipeline/reflection.go` already implements this.)
- **`Human-in-the-loop`**: use `runtime.StateGraph` with a gate node that returns an `Interrupt` error and resumes via checkpointer. (`runtime.Interrupt` already exists.)
- **`Multi-agent swarms`**: spawn parallel sub-graphs per chapter and merge results. (`pkg/pipeline/swarm.go` and `pkg/booktools/swarm.go` already have initial support.)

---

## Part 5: File Inventory (What Changed)

### New / Moved packages
- `pkg/runtime/*` (moved from `internal/runtime`)
- `pkg/booktools/*` (moved from `internal/booktools`)
- `pkg/agent/*` (moved from `internal/agent`)
- `pkg/agents/*` (moved from `internal/agents`)
- `pkg/pipeline/*` (moved from `internal/pipeline`)
- `pkg/llm/*` (moved from `internal/llm`)
- `pkg/research/*` (moved from `internal/research`)
- `pkg/search/*` (moved from `internal/search`)
- `pkg/wordcount/*` (moved from `internal/wordcount`)
- `pkg/youtube/*` (moved from `internal/youtube`)
- `pkg/config/*` (moved from `internal/config`)
- `pkg/log/*` (moved from `internal/log`)

### Deleted internal packages
- `internal/runtime/*`
- `internal/booktools/*`
- `internal/agent/*` (except adapter remains in `pkg/agent`)
- `internal/pipeline/*` (except registry/executor logic now in `pkg/pipeline`)
- `internal/llm/*`
- `internal/research/*`
- `internal/search/*`
- `internal/wordcount/*`
- `internal/youtube/*`
- `internal/config/*`
- `internal/log/*`

### Deleted files (within moved packages)
- `internal/agent/tools.go` → deleted
- `internal/agent/chat_tools.go` → deleted
- `internal/agent/tools_test.go` → deleted
- `internal/agent/chat_tools_test.go` → deleted

### Modified files
- `go.work` — added all `pkg/*` workspaces
- `go.mod` — added `replace` directives for all `pkg/*` modules
- `internal/commands/open.go`
- `internal/tui/screen_agentchat.go`
- `internal/jobs/worker.go`

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

1. Create a struct in `pkg/booktools/<category>.go` that implements `runtime.Tool`.
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
