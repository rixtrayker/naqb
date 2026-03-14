# Architecture

## Overview

`nqb` is a single Go binary. All functionality lives in `internal/` packages;
`cmd/nqb/main.go` is just a Cobra root that wires them together.

```
┌──────────────────────────────────────────────────────────┐
│                      nqb (binary)                        │
│                                                          │
│  cmd/nqb/main.go   ← cobra root + dynamic completions   │
│                                                          │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────────┐  │
│  │  commands/   │  │    tui/       │  │   vault/     │  │
│  │  (CLI cmds)  │  │  (Bubble Tea) │  │  (registry)  │  │
│  └──────┬───────┘  └──────┬────────┘  └──────────────┘  │
│         │                 │                              │
│  ┌──────▼─────────────────▼──────────────────────────┐  │
│  │                   agents/                          │  │
│  │  planner → context_builder → writer → qa           │  │
│  └──────────────────────┬────────────────────────────┘  │
│                         │                               │
│  ┌──────────┐  ┌────────▼──────┐  ┌──────────────────┐  │
│  │   llm/   │  │  pipeline/    │  │   exporter/      │  │
│  │ (Anthropic│  │ (orchestrate +│  │ (pandoc wrappers)│  │
│  │  SDK)    │  │  git commits) │  │                  │  │
│  └──────────┘  └───────────────┘  └──────────────────┘  │
│                                                          │
│  ┌──────────┐  ┌───────────────┐                        │
│  │  config/ │  │   watcher/    │                        │
│  │(yaml CRUD)│  │ (fsnotify)    │                        │
│  └──────────┘  └───────────────┘                        │
└──────────────────────────────────────────────────────────┘
```

---

## Package Responsibilities

### `cmd/nqb`
Entry point. `nqb` alone → TUI home. `nqb .` → open current dir. Registers all subcommands.

### `internal/vault`
Obsidian-style vault registry stored at `~/.naqb/vault.yaml`.
Scans vault directories for subdirs containing `book.yaml`.
`ListProjects()` returns all projects sorted by modification time.

### `internal/config`
- `GlobalConfig` — API key, loaded from `~/.naqb/config.yaml` or `ANTHROPIC_API_KEY`
- `BookConfig` — per-project `book.yaml` CRUD
- `Template` — three built-in templates: `arabic-research`, `cs-book`, `general`

### `internal/llm`
Thin wrapper over the Anthropic Go SDK.
- `Complete()` — single non-streaming call
- `Stream()` — streaming call with delta callback
- Uses `.AsAny()` on `MessageStreamEventUnion` (not `.AsUnion()`)

### `internal/agents`
The four pipeline stages, each independently callable:

| File | Stage | Input | Output |
|------|-------|-------|--------|
| `planner.go` | 0 — Plan | `InterviewAnswers` | `BookConfig` + `outline.md` |
| `context_builder.go` | 1 — Context | `BookConfig` + chapter N | `contexts/ch-NN-context.md` |
| `writer.go` | 2 — Write | context file | `chapters/ch-NN.md` |
| `qa.go` | 3 — QA | chapter file | `QAResult` + `pipeline-report.md` |

### `internal/pipeline`
`RunChapterPipeline` orchestrates stages 1→2→3 and calls `GitCommit` between each.

### `internal/exporter`
Pandoc wrappers for PDF (XeLaTeX + polyglossia for Arabic RTL), EPUB, DOCX, and static HTML.

### `internal/tui`
All Bubble Tea screens:

| File | Screen |
|------|--------|
| `home.go` | VSCode-style project picker |
| `book_view.go` | Sidebar + slash command palette |
| `outline_editor.go` | Visual chapter outline editor |
| `preview.go` | Glamour markdown renderer |
| `chat.go` | Streaming Opus chat REPL |
| `init_chat.go` | Multi-step init form |
| `keys.go` | Central keybinding definitions |
| `spinner.go` | Spinner wrapper for long tasks |

### `internal/log`
Leveled logger (`DEBUG/INFO/WARN/ERROR`) → `~/.naqb/nqb.log`.
`NQB_DEBUG=1` drops to DEBUG and echoes to stderr.

---

## LLM Model Assignments

| Stage | Model | Reason |
|-------|-------|--------|
| `nqb init` interview | `claude-haiku-4-5` | Fast, interactive |
| `nqb write` drafting | `claude-sonnet-4-6` | Best long-form quality |
| `nqb qa` semantic audit | `claude-sonnet-4-6` | Good reasoning |
| `nqb chat` editing REPL | `claude-opus-4-6` | Deep edits, nuanced |
| Context assembly | No LLM | Pure Go templating |

---

## Data Flow: `nqb init`

```
RunInitForm() [TUI]
    │
    ▼
InterviewAnswers{title, author, language, domain, synopsis, chapters, template}
    │
    ▼
RunPlanner(ctx, client, answers) [Haiku]
    │  → LLM generates YAML chapter list + outline.md
    ▼
PlannerResult{BookConfig, OutlineMD}
    │
    ▼
InitBookDir(bookDir, cfg)
    │  → creates directories, book.yaml, rules.yaml, default prompts, .gitignore
    ▼
writeTemplateFiles() [if template selected]
    │  → overrides rules.yaml, write.md, qa.md
    ▼
GitInit() + GitCommit("init: ...")
    │
    ▼
vault.RecordRecent()
```

## Data Flow: `nqb write --chapter N`

```
config.LoadBook(bookDir)
    │
    ▼
agents.WriteContextFile(bookDir, cfg, N)
    │  → reads outline.md section, previous chapter summaries, research/ notes
    │  → executes Go text/template → contexts/ch-NN-context.md
    ▼
agents.WriteChapter(ctx, client, bookDir, cfg, N, onDelta)
    │  → reads context file
    │  → streams Sonnet → chapters/ch-NN.md
    ▼
chapter file on disk
```

---

## Git Auto-Commits

After each meaningful pipeline stage:

```
init:          "My Book" — book initialized
context(01):   Chapter 1 context assembled
chapter(01):   Chapter 1 first draft
qa(01):        Chapter 1 QA complete
export(pdf):   PDF generated
```
