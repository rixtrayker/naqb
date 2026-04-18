---
name: naqb-add-tool
description: "Add a new runtime tool to the naqb (نقب) agent system. Use when extending internal/booktools/ with new capabilities for the fantasy agent loop, MCP server, or pipeline stages."
user-invocable: true
license: MIT
compatibility: Designed for Claude Code or similar AI coding agents working on the github.com/amr/naqb project.
metadata:
  author: naqb
  version: "1.0.0"
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Agent
---

**Persona:** You are a platform engineer extending naqb's tool registry. You implement `runtime.Tool` interfaces and wire them into the agent and MCP server.

## How to Add a New Tool

### Step 1: Choose the Right File

Tools are organized by domain in `internal/booktools/`:

| File | Tools |
|------|-------|
| `file.go` | `read_file`, `write_file`, `list_chapters` |
| `research.go` | `search_research`, `web_fetch`, `grep_chunks` |
| `knowledge.go` | `knowledge_search` |
| `qa.go` | `run_qa` |
| `spawn.go` | `spawn_write`, `spawn_qa`, `spawn_pipeline`, `spawn_research` |

Add your tool to the most relevant file, or create a new one if it doesn't fit.

### Step 2: Implement `runtime.Tool`

Every tool must implement:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() jsonschema.Schema
    Invoke(ctx context.Context, input string) (string, error)
}
```

**Best practices:**
- Return clear, actionable error messages
- Keep descriptions concise (used in LLM system prompts)
- Use simple JSON schema types
- If the tool needs to stream or spawn background work, implement `runtime.TaskSpawner` too

### Step 3: Implement `FantasyToolProvider` (optional but recommended)

If you need precise JSON schema control for `charm.land/fantasy`, also implement:

```go
type FantasyToolProvider interface {
    FantasyTool() fantasy.AgentTool
}
```

The `internal/agent/adapter.go` duck-types this to preserve custom schema.

### Step 4: Register the Tool

Tools are wired in `internal/tui/screen_agentchat.go` and `internal/jobs/worker.go`. Search for `booktools.New` calls and add your tool to the slice passed to `agent.WithTools()`.

### Step 5: Add Tests

Create or extend `internal/booktools/[file]_test.go`. Test the `Invoke` method directly without needing a full agent runtime.

### Step 6: Update Docs

If the tool is user-facing in agent chat, add it to the system prompt in `internal/agent/agent.go` (`buildAnalysisSystemPrompt` or `buildSystemPrompt`).
