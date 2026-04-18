# نقب (naqb) — Claude Code Instructions

## Commit Protocol

**Run before every commit — no exceptions:**

```bash
make check    # go build + go vet + go test
```

Never commit with:
- A compilation error
- A failing `go vet` warning
- A failing test
- Missing documentation updates (see below)

## Documentation Rule

**Every code change that affects behavior must update docs:**

| Change type | Required doc update |
|---|---|
| New/changed command flag | `README.md` usage section |
| New/changed function signature | Docstring on the function |
| New package or major feature | `agent-os/standards/` relevant file |
| Architecture or key file paths | `memory/MEMORY.md` |
| New dependency | `go.mod` comment + MEMORY.md Dependencies |
| Breaking change to any public API | README.md + relevant standard |

Doc-only commits still require `go build ./...` to pass.

## Testing Protocol

**Add tests alongside every new function:**

- New package → new `_test.go` file in the same package
- New exported function → at least one test (happy path + one error case)
- Bug fix → add a test that would have caught the bug
- Never reduce existing test coverage intentionally

**Coverage targets** (see `agent-os/standards/global/testing.md` for full list):
- `internal/db` ≥ 70%
- `internal/jobs` ≥ 60%
- `internal/agent` ≥ 55%
- `internal/search` ≥ 45%
- `internal/wordcount` ≥ 80%
- `internal/vault` ≥ 60%

Check coverage with:
```bash
make cover-text
```

## Code Conventions

- Go module: `github.com/amr/naqb`
- Binary: `nqb` (built to `./bin/nqb`)
- Config dir: `~/.naqb/`
- DB path: `~/.naqb/naqb.db` (WAL mode, FK enforced via PRAGMA)
- All file I/O tests use `t.TempDir()` — never touch real user directories
- Skip network/LLM tests when credentials not set: `t.Skip("ANTHROPIC_API_KEY not set")`

## Key Architecture Notes

See `memory/MEMORY.md` for the full architecture map.

Short version:
- `internal/db/` — SQLite via modernc.org/sqlite (WAL + FK via PRAGMA, not DSN)
- `internal/agent/` — charm.land/fantasy agent loop + 6 tools
- `internal/jobs/` — async job queue backed by SQLite
- `internal/pipeline/` — chapter pipeline; uses agent when FantasyProvider != nil
- `internal/tui/` — bubbletea v1 TUI (upgrade to v2 deferred)

## Pull Request / Merge Checklist

- [ ] `make check` passes
- [ ] New code has tests (or documented reason why it can't)
- [ ] Docs updated (README, standards, MEMORY.md as applicable)
- [ ] No secrets committed (`.env`, API keys, credentials)
- [ ] `go mod tidy` run if dependencies changed
