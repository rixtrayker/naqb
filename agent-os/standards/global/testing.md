# Testing

## Commit Gate Rule

Before every commit, ALL of the following must pass:

```bash
go build ./...          # no compilation errors
go vet ./...            # no suspicious constructs
go test ./...           # all tests green
```

- **Never commit** with a failing build, failing vet, or failing tests
- **Docs must update** alongside code changes — see below
- Use `make check` to run the full gate in one command

## Documentation Rule

Every code change that affects behavior, APIs, or user-facing commands **must** include:
- Updated docstring/comment in the changed function(s)
- Updated `README.md` (if command output, flags, or config changes)
- Updated relevant `agent-os/standards/` file (if the pattern itself changed)
- Updated `MEMORY.md` key facts (if architecture or key file paths changed)

Doc-only changes do NOT require tests, but still must pass `go build ./...`.

## Coverage Targets

Coverage is measured per package. Run `make cover-text` to see current numbers.

| Package | Target | Rationale |
|---|---|---|
| `internal/db` | ≥ 70% | Critical: all persistence goes through here |
| `internal/jobs` | ≥ 60% | Critical: job lifecycle (enqueue/next/complete/fail) |
| `internal/agent` | ≥ 55% | Critical: tool path traversal, provider factory |
| `internal/search` | ≥ 45% | Critical: keyword search fallback, open/close |
| `internal/wordcount` | ≥ 80% | Pure logic, fully testable |
| `internal/vault` | ≥ 60% | Core data access, no external deps |
| `internal/config` | ≥ 30% | Partial: keychain/file I/O hard to unit test |
| `internal/tui` | ≥ 5% | Bubbletea models: unit-test pure helpers only |
| `internal/agents` | ≥ 20% | LLM calls excluded; test deterministic helpers |
| `internal/research` | ≥ 15% | Test parsers/formatters; skip network calls |
| `internal/pipeline` | ≥ 15% | Integration tests deferred; test stage inputs |
| `internal/llm` | ≥ 10% | Test model constants, budget tracker |
| `internal/commands` | 0% | CLI glue — covered by integration/manual |
| `internal/exporter` | 0% | Pandoc wrappers — covered by manual |
| `internal/watcher` | 0% | fsnotify — covered by manual |
| `cmd/nqb` | 0% | Entry point — covered by build |

Packages at 0% that **should** remain 0%: commands, exporter, watcher, cmd — all are thin glue over
external tools (pandoc, fsnotify, cobra). Unit testing them provides no value; they are exercised by
manual `nqb` runs and will eventually get integration tests.

## What to Test

**Always test:**
- Pure functions: parsers, formatters, word counters, URL sanitizers
- Data access: CRUD round-trips (db, vault, config)
- Security boundaries: path traversal blocks in tools, key lookup order
- State machines: job status transitions, agent session lifecycle
- Error paths: missing file → graceful error; wrong input → IsError=true

**Skip in unit tests (use manual/integration):**
- Functions that make real network calls (LLM, HTTP fetch, Gemini)
- Functions that require a running external service (chromem-go embedder)
- TUI model rendering (use table-driven golden-file tests only when stable)

## Test Patterns

### Table-Driven Tests
Use for any function with multiple input/output cases:
```go
cases := []struct{ name, input, want string }{...}
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

### Temp Dirs
Use `t.TempDir()` for all file I/O tests. Never write to real user directories.

### Skip on Missing Credentials
```go
if os.Getenv("ANTHROPIC_API_KEY") == "" {
    t.Skip("ANTHROPIC_API_KEY not set")
}
```

### Error vs Fatal
- `t.Fatal` / `t.Fatalf` — test cannot continue (setup failed, nil pointer would panic)
- `t.Error` / `t.Errorf` — test can continue (wrong output but we can still check more)

### Naming Convention
```
TestTypeName_Scenario         → TestKeywordSearch_FindsMatch
TestFuncName_Input_Expected   → TestBuildChapterTask_WithContextFile
TestFuncName_EdgeCase         → TestOpen_Idempotent
```

## Running Tests

```bash
make check          # build + vet + test (gate)
make test           # tests only
make test-v         # tests verbose
make test-race      # with race detector
make cover          # HTML coverage report → coverage.html
make cover-text     # per-package coverage summary in terminal
```
