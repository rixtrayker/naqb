# Contributing to nqb

Thank you for your interest in contributing to **نقب (nqb)**!

---

## Getting Started

```bash
git clone https://github.com/rixtrayker/naqb
cd naqb
go mod tidy
make build
```

Requirements: **Go 1.21+**, `pandoc` (for export tests), `git`.

---

## Development Workflow

```bash
make build      # compile to ./bin/nqb
make test       # run all tests
make vet        # static analysis
make test-race  # race detector
make cover      # coverage report

# Debug mode: logs to stderr + ~/.naqb/nqb.log
make debug
# or:
NQB_DEBUG=1 ./bin/nqb

# Tail logs
make log
```

---

## Project Structure

```
cmd/nqb/            Entry point (cobra root command)
internal/
  agents/           LLM pipeline stages (planner, context, writer, qa)
  commands/         One file per CLI subcommand
  config/           Global config + book.yaml CRUD + templates
  exporter/         Pandoc wrappers (pdf, epub, docx, web)
  llm/              Anthropic SDK wrapper (streaming + non-streaming)
  log/              Leveled logger (→ ~/.naqb/nqb.log)
  pipeline/         Stage orchestrator + git auto-commits
  tui/              All Bubble Tea screens
  vault/            Vault registry (~/.naqb/vault.yaml)
  watcher/          fsnotify file watcher
```

---

## Guidelines

- **Keep packages focused** — each package has one job.
- **No TUI in agent packages** — agents must be callable from both CLI and TUI.
- **Log, don't print** — use `log.Info/Warn/Error` inside packages; `fmt.Printf` only in `commands/`.
- **Test pure logic** — unit test functions that don't require a live LLM or terminal. Mock the LLM client in integration tests.
- **Arabic-first** — RTL rendering, Amiri font, and MSA language quality are first-class concerns.
- **Keep the binary small** — avoid heavy dependencies; prefer stdlib where possible.

---

## Adding a New Template

1. Open `internal/config/templates.go`
2. Add a new `Template` entry to the `templates` slice
3. Add a case in `internal/tui/init_chat.go` → `stepLabels`/`stepDefaults` if it needs a UI hint
4. Add a test in `internal/config/book_test.go`

---

## Adding a New CLI Command

1. Create `internal/commands/mycommand.go`
2. Export `MyCmd() *cobra.Command`
3. Register it in `cmd/nqb/main.go` → `rootCmd.AddCommand(...)`
4. Add completions in `internal/commands/completion.go` if it has flags

---

## Submitting a PR

- One logical change per PR
- `make test && make vet` must pass
- Describe **why**, not just what
- For UX changes, include a short terminal recording or screenshot

---

## Reporting Issues

Open an issue at https://github.com/rixtrayker/naqb/issues with:
- `nqb` version (`nqb --version` once implemented, or git SHA)
- OS + Go version
- Steps to reproduce
- Contents of `~/.naqb/nqb.log` (set `NQB_DEBUG=1` first)
