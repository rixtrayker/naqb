# Tool Interface

All agent tools live in `internal/booktools/` and implement `runtime.Tool`.

## Interface

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() jsonschema.Schema
    Invoke(ctx context.Context, input string) (string, error)
}
```

## Optional: Fantasy Schema Control

If precise JSON schema is needed for `charm.land/fantasy`, also implement:

```go
type FantasyToolProvider interface {
    FantasyTool() fantasy.AgentTool
}
```

`internal/agent/adapter.go` duck-types this to preserve custom schema.

## File Organization

| File | Tools |
|------|-------|
| `file.go` | `read_file`, `write_file`, `list_chapters` |
| `research.go` | `search_research`, `web_fetch`, `grep_chunks` |
| `knowledge.go` | `knowledge_search` |
| `qa.go` | `run_qa` |
| `spawn.go` | `spawn_write`, `spawn_qa`, `spawn_pipeline`, `spawn_research` |

## Wiring

Pass tools to `agent.WithTools([]runtime.Tool{...})` in:
- `internal/tui/screen_agentchat.go`
- `internal/jobs/worker.go`

## Testing

Test `Invoke` directly in `internal/booktools/[file]_test.go`. No full agent runtime needed.
