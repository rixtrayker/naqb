# نقب — Naqb

> **نقب** (naqb) — *to excavate, to tunnel through, to investigate deeply.*
> An archaeologist نقّب عن الآثار. A writer نقّب في الأفكار.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/rixtrayker/naqb/actions/workflows/ci.yml/badge.svg)](https://github.com/rixtrayker/naqb/actions/workflows/ci.yml)
[![Security](https://github.com/rixtrayker/naqb/actions/workflows/security.yml/badge.svg)](https://github.com/rixtrayker/naqb/actions/workflows/security.yml)

> From spark to shelf — CLI that orchestrates LLMs through the full book lifecycle:
> interview → outline → write → QA → export. Built for depth thinkers.

`Naqb` (نقب) is an LLM-powered CLI tool for writing structured, ADHD-friendly books.
It manages the full lifecycle: interview → outline → context → write → QA → export.
Primary focus: **Arabic RTL books**, with full support for English/technical books too.

**Default LLM:** MiniMax M2.5 via OpenRouter (free tier available). Also supports Anthropic and AWS Bedrock.

---

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Providers](#providers)
- [Vault System](#vault-system)
- [TUI — Home Screen](#tui--home-screen)
- [TUI — Book View](#tui--book-view)
- [CLI Commands](#cli-commands)
- [Keybinding Cheatsheet](#keybinding-cheatsheet)
- [Templates](#templates)
- [Export](#export)
- [Shell Completion](#shell-completion)
- [Architecture](#architecture)
- [Roadmap](#roadmap)

---

## Install

**Requirements:** Go 1.26+, pandoc (for export), XeLaTeX (for PDF)

```bash
# Clone and build
git clone https://github.com/rixtrayker/naqb
cd naqb

# Build all binaries (nqb, naqb-style, nqb-mcp)
make build

# Or build individually
go build -o nqb ./cmd/nqb
go build -o naqb-style ./cmd/naqb-style
go build -o nqb-mcp ./cmd/nqb-mcp

# Move to PATH
mv nqb /usr/local/bin/nqb

# Set your API key (OpenRouter/MiniMax default)
nqb config --set-key
# or: export OPENROUTER_API_KEY=sk-or-v1-...

# Alternative: use Anthropic directly
export ANTHROPIC_API_KEY=sk-ant-...
```

### Export dependencies

| Format | Requirement |
|--------|-------------|
| PDF    | `pandoc` + `xelatex` (TeX Live) + `Amiri` font (Arabic) |
| EPUB   | `pandoc` |
| DOCX   | `pandoc` |
| Web    | Built-in (no deps) |

```bash
# macOS
brew install pandoc
brew install --cask mactex   # includes xelatex

# Ubuntu
apt install pandoc texlive-xetex fonts-arabeyes
```

---

## Quick Start

```bash
# Launch the interactive project picker (TUI home screen)
nqb

# Create a new book in the current directory
nqb init

# Create a book inside your vault (~/Projects/naqb/)
nqb init --vault

# Open interactive agent chat for the current project
nqb .

# Open a specific book in the chapter list TUI
nqb open my-arabic-book
```

### Typical workflow for a new book

```bash
# 1. Initialize (LLM interview → book.yaml + outline.md)
nqb init --vault

# 2. Open the book in the TUI
nqb open my-book-name

# 3. Inside the TUI: press w to write chapter 1, q to QA it, p to preview

# 4. Export when ready
nqb export --format pdf
nqb export --format all
```

---

## Providers

nqb supports multiple LLM providers:

| Provider | Type | Default Model | Setup |
|----------|------|---------------|-------|
| OpenRouter (default) | OpenAI-compatible | MiniMax M2.5 | `OPENROUTER_API_KEY` |
| Anthropic | Native SDK | Claude Sonnet 4-6 | `ANTHROPIC_API_KEY` |
| AWS Bedrock | Converse API | MiniMax M2.1 | IAM credentials |

### Configuration

```yaml
# ~/.naqb/config.yaml
default_provider: openrouter

providers:
  openrouter:
    type: openrouter
    api_key: "sk-or-v1-..."
    base_url: "https://openrouter.ai/api/v1"

  anthropic:
    type: anthropic
    api_key: "sk-ant-..."

  bedrock:
    type: bedrock
    api_key: "AKIAIOSFODNN7EXAMPLE"
    secret_access_key: "wJalrXUtnFEMI/..."
    region: "us-east-1"
```

Override per-book in `book.yaml`:
```yaml
llm:
  write_provider: openrouter
  write_model: minimax/minimax-m2.5
  qa_provider: anthropic
  qa_model: claude-sonnet-4-6
```

---

## Vault System

`nqb` organizes books into **vaults** — directories that contain book projects.

| Path | Purpose |
|------|---------|
| `~/.naqb/` | Global config directory |
| `~/.naqb/vault.yaml` | Vault registry + recent projects |
| `~/.naqb/projects/` | **Default vault** — books created with `--vault` go here |
| Any directory | Can be used as a vault after `nqb vault add` |

### Vault commands

```bash
nqb vault list              # Show all registered vaults
nqb vault add work ~/work/books   # Register ~/work/books as vault "work"
nqb vault remove work       # Unregister (does NOT delete files)
```

### `nqb .` — interactive agent chat

```bash
cd ~/my-arabic-research
nqb .         # Opens agent chat if book.yaml exists + API key available
              # Falls back to BookView if no Fantasy provider
              # Asks "Initialize here?" if no book.yaml
```

The agent chat is an interactive session where you can:
- Ask questions about your book project (status, word counts, chapter summaries)
- Say "write chapter N" — the agent calls `spawn_write` to run it in the background
- Say "run QA on chapter N" — background QA with results streamed back
- Run multiple background tasks in parallel (write + research)
- Press **Ctrl+T** to toggle the task panel showing active/completed tasks

---

## TUI — Home Screen

Launch with `nqb` (no arguments):

```
  نقب  nqb
  probe your ideas. give them depth.

  Search: [_____________]

  Projects
  ─────────────────────────────────────────────────────
  ▶ كتاب التاريخ العربي          ar  8/12 ch  2h ago
    CS Algorithms Book             en  3/10 ch  1d ago
    Philosophy Draft               ar  0/6  ch  3d ago

  [Enter] Open · [n] New book · [v] Vaults · [q] Quit
```

**Search** filters by title, language (`ar`/`en`), domain, and recency.
Pressing `n` launches `nqb init` inline and returns to the home screen when done.

---

## TUI — Book View

Opens when you select a project from home or run `nqb open <name>`:

```
  نقب  كتاب التاريخ العربي
  ┌ ACTIONS ──────────┬──── CHAPTERS ──────────────────────────────────┐
  │  w  Write chapter │  ○  Ch01  المقدمة                              │
  │  q  QA            │  ●  Ch02  العصر الجاهلي          (2,847 words)  │
  │  e  Export        │  ▶  Ch03  صدر الإسلام             ← selected    │
  │  p  Preview       │  ○  Ch04  العصر الأموي                          │
  │  o  Outline       │                                                  │
  │  ~  Chat (Opus)   │  Selected: Chapter 3 — صدر الإسلام              │
  │  s  Status        │  Overview of the early Islamic period and...     │
  │  W  Watch         │                                                  │
  │  ?  Help          │                                                  │
  └───────────────────┴──────────────────────────────────────────────────┘

  [↑/↓] Chapter · [/] Command palette · [w] Write · [q] QA · [?] Help · [Ctrl+C] Back
```

### Command Palette (`/`)

Press `/` to open the command palette and type any command:

```
  Command palette
  > /write --chapter 3
  > /qa --chapter 3 --deterministic-only
  > /export --format epub
  > /pipeline --chapter 3
  > /preview --chapter 3
  > /context --chapter 3
  > /status
  > /help
```

### Help Overlay (`?`)

Press `?` at any time in the book view for the full keybinding reference.

---

## CLI Commands

All commands work directly from the terminal. Commands are organized into groups —
run `nqb --help` to see the full grouped output. Most commands have short aliases
(shown in parentheses).

### Global Flags

| Flag | Description |
|------|-------------|
| `--verbose` / `-v` | Show debug info (LLM requests, token counts, timing) |
| `--quiet` / `-q` | Suppress non-essential output |
| `--no-color` | Disable color output (also respects `NO_COLOR` env var) |

### Writing

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb init` (`i`, `new`) | Initialize new book via LLM interview |
| `nqb write` (`w`) `--chapter N` | Write chapter N with streaming LLM |
| `nqb context` (`ctx`) `--chapter N` | Build context file for chapter N |
| `nqb pipeline` (`pipe`, `p`) `--chapter N` | Run context→write→qa for chapter N |
| `nqb pipeline --all` | Run full pipeline for all chapters |
| `nqb fix` (`f`) `--chapter N` | Rewrite chapter to fix QA issues |
| `nqb chat` (`c`) `[--chapter N]` | Open chat REPL with book context |

### Quality

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb qa` (`check`) `--chapter N` | Deterministic + LLM QA audit |
| `nqb qa --deterministic-only` | Skip LLM audit |
| `nqb research` (`res`) `--chapter N` | Run Scout→Explorer→Scribe pipeline |
| `nqb research --deep` | Deep research with Gemini grounding |
| `nqb index` (`idx`) | Index chapters into local vector store |

### Publishing

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb export` (`exp`) `--format pdf` | Export to PDF (RTL Arabic via XeLaTeX) |
| `nqb export --format all` | Export all formats (pdf, epub, docx, web) |
| `nqb watch` | Watch for changes, auto-rebuild exports |
| `nqb status` (`st`, `info`) | Chapter progress table + git log |

### Management

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb open` (`o`) `<name>` | Open book from vault by name or path |
| `nqb list` (`ls`) | List all books in the vault |
| `nqb list chapters` (`ls ch`) | List chapters with status and word count |
| `nqb batch` (`b`) `enqueue --chapter N` | Add a chapter job to the queue |
| `nqb batch status` (`b st`) | Show job queue status |
| `nqb batch run [--workers N]` | Start background job worker |
| `nqb batch cancel --job-id <id>` | Cancel a queued job |
| `nqb session` (`sess`) `list` | List recent agent sessions |
| `nqb session show <id>` | Show session messages |
| `nqb session delete <id>` | Delete a session |
| `nqb vault list` (`ls`) | Show all registered vaults |
| `nqb vault add <name> <path>` | Register a directory as a vault |
| `nqb vault remove` (`rm`) `<name>` | Unregister a vault |
| `nqb sync gdocs` | Push chapters to Google Doc |
| `nqb import` | Import wizard (notes, drafts, templates) |
| `nqb import gdoc --url <URL>` | Import from Google Docs |

### Configuration

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb config` (`cfg`) | Show global config |
| `nqb config --set-key` | Set API key |
| `nqb keys` (`k`) | Show all API key statuses (set/missing, source) |
| `nqb keys --set <NAME>` | Save an API key to macOS Keychain |
| `nqb setup` | Run the first-time setup wizard |
| `nqb models` (`m`) | List available models and pricing |
| `nqb mcp` | Start MCP server |

### Utility

| Command (aliases) | Description |
|-------------------|-------------|
| `nqb version` (`v`) | Print version, Go runtime, and build info |
| `nqb doctor` (`doc`) | Check system health: API keys, deps, book, DB |
| `nqb completion bash\|zsh\|fish` | Shell completion scripts |

---

## Keybinding Cheatsheet

### Home Screen

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate projects |
| `j` / `k` | Navigate (vim) |
| `Enter` | Open selected project |
| `n` | New book |
| `v` | Vault manager |
| `q` | Quit |
| Type anything | Filter/search |
| `Esc` | Clear search |

### Book View

| Key | Action |
|-----|--------|
| `↑` / `↓` / `j` / `k` | Navigate chapters |
| `w` | Write selected chapter |
| `q` | QA selected chapter |
| `e` | Export (PDF) |
| `p` | Preview chapter (glamour renderer) |
| `o` | Outline editor |
| `~` | Chat with Claude Opus |
| `s` | Status summary |
| `W` | Watch mode |
| `/` | Open command palette |
| `?` | Toggle help overlay |
| `Ctrl+C` | Back / quit |

### Command Palette

| Key | Action |
|-----|--------|
| Type | Enter command |
| `Enter` | Execute |
| `Esc` | Close |
| `Tab` | Complete (cobra) |

### Outline Editor

| Key | Action |
|-----|--------|
| `↑` / `↓` / `j` / `k` | Navigate chapters |
| `t` / `Enter` | Edit chapter title |
| `s` | Edit chapter summary |
| `U` | Move chapter up |
| `D` | Move chapter down |
| `Ctrl+S` | Save to book.yaml + outline.md |
| `q` / `Esc` | Back (prompts if unsaved) |

### Preview

| Key | Action |
|-----|--------|
| `↑` / `↓` / `j` / `k` | Scroll |
| `PgUp` / `PgDn` | Page scroll |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `q` / `Esc` | Back |

### Chat REPL

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Alt+Enter` | Insert newline in message |
| `↑` / `↓` | Scroll history |
| `Ctrl+C` | Quit |

---

## Templates

Choose a template during `nqb init`:

| # | Template | Language | Font | Use Case |
|---|----------|----------|------|----------|
| 1 | Arabic Research (كتاب بحثي) | `ar` (RTL) | Amiri | Scholarly Arabic books, cultural research |
| 2 | CS / Technical Book | `en` | IBM Plex Sans + JetBrains Mono | Programming, computer science |
| 3 | General | `en` or `ar` | Configurable | Anything else |

Each template pre-configures:
- `config/rules.yaml` (fonts, callouts, word count targets, RTL)
- `config/prompts/write.md` (writer system prompt)
- `config/prompts/qa.md` (QA reviewer system prompt)

---

## ADHD-Friendly Formatting

All templates use these callout conventions:

| Callout | Markdown | Meaning | Color |
|---------|----------|---------|-------|
| Note | `[!] text` | Important note | Yellow |
| Deep dive | `[?] text` | Further exploration | Blue |
| Warning | `[X] text` | Caution / common mistake | Red-pink |

The QA stage checks that code blocks have language tags, heading hierarchy is valid (no h1→h3 skips), and word count is within target range.

---

## Export

```bash
nqb export --format pdf    # Arabic RTL PDF via pandoc + XeLaTeX
nqb export --format epub   # EPUB with optional CSS theme
nqb export --format docx   # Word document
nqb export --format web    # Static HTML (dark mode, RTL-aware)
nqb export --format all    # All four formats
```

Output goes to `output/` in your book directory (gitignored by default).

### Arabic PDF (RTL)

The PDF exporter uses `polyglossia` + `Amiri` font for correct right-to-left rendering:

```bash
# Required: Amiri font
# macOS: download from https://fonts.google.com/specimen/Amiri
# Ubuntu: apt install fonts-arabeyes
```

---

## Shell Completion

```bash
# Bash
nqb completion bash > /etc/bash_completion.d/nqb

# Zsh
nqb completion zsh > "${fpath[1]}/_nqb"

# Fish
nqb completion fish > ~/.config/fish/completions/nqb.fish

# Carapace (recommended)
nqb completion carapace zsh   # prints install instructions
```

Dynamic completions:
- `nqb open <TAB>` → lists all vault project names
- `nqb write --chapter <TAB>` → lists chapter numbers with titles
- `nqb export --format <TAB>` → `pdf`, `epub`, `docx`, `web`, `all`

---

## Project Layout

After `nqb init`, your book directory looks like:

```
my-book/
├── book.yaml               ← manifest: title, author, chapters, LLM settings
├── outline.md              ← chapter-by-chapter breakdown (LLM-generated)
├── config/
│   ├── rules.yaml          ← tone, formatting rules, word count targets
│   └── prompts/
│       ├── init.md         ← system prompt for planner agent
│       ├── write.md        ← system prompt for writer agent
│       └── qa.md           ← system prompt for QA agent
├── chapters/
│   ├── ch-01.md
│   └── ch-02.md
├── contexts/               ← assembled single-shot context per chapter (gitignored)
├── research/              ← LLM-generated research notes (gitignored)
├── .naqb/                 ← vector store index (gitignored)
├── assets/
│   └── themes/
│       ├── light.css
│       └── dark.css
├── output/                ← generated files (gitignored)
│   ├── book.pdf
│   ├── book.epub
│   └── web/
└── pipeline-report.md    ← QA results and run summaries
```

---

## Architecture

`nqb` is a **Go workspace** (`go.work`) with a root module and 11 standalone `pkg/*` modules.
The root `go.mod` uses `replace` directives for local development; each `pkg/*` has its own `go.mod`.

```
cmd/
  nqb/main.go                Main CLI — cobra root + dynamic completions
  naqb-style/main.go         Style engine CLI (extract/apply/blend/diff/list/fork/fingerprint)
  nqb-mcp/main.go            Standalone MCP server

pkg/                         ← standalone Go modules (go.work members)
  runtime/                   LangGraph-style core: StateGraph, CompiledGraph, Checkpointer, Registry
  agent/                     Fantasy-based agent loop with SessionStore/EpistemicStore interfaces
  agents/                    Legacy single-shot orchestration (planner, writer, QA, conflict, gap)
  pipeline/                  Stage registry + DAG executor + swarm + reflection + debt tracking
  booktools/                 Concrete agent tools: file, research, knowledge, QA, spawn, plan/execute
  llm/                       Provider interface + OpenRouter/Anthropic/Bedrock implementations
  config/                    GlobalConfig, BookConfig, Template, Rules
  research/                  Scout→Explorer→Scribe research pipeline
  search/                    Vector + keyword store routing layer
  wordcount/                 Word counting utilities
  youtube/                   YouTube transcript fetching
  log/                       Structured logging wrapper

internal/                    ← root-module-only packages
  commands/                  One file per CLI subcommand
  tui/                       Bubble Tea screens: home, book view, chat, outline, preview
  db/                        SQLite persistence (sessions, messages, jobs, claims, knowledge graph)
  vault/                     Vault registry (~/.naqb/vault.yaml), project scanning
  exporter/                  Pandoc wrappers: PDF (RTL), EPUB, DOCX, Web
  store/                     VectorStore, KeywordStore, HybridStore interfaces + implementations
  knowledge/                 Claim (8 types) + Graph (8 relations) + EpistemicState
  context/                   Context stacks + BraidedField + Arabic analytical layers
  style/                     StyleImage engine: extract / apply / blend / diff / registry
  jobs/                      Async job queue (SQLite-backed) + worker pool
  chunker/                   Recursive text splitter with Arabic separators
  embedding/                 Embedder interface: OpenAI-compat, Voyage AI, Ollama
  rerank/                    NullReranker + CohereReranker
  searchutil/                NormalizeContent, TokenizeContent, JaccardSimilarity
  gdocs/                     Google Docs sync client
  mcpserver/                 MCP server implementation
  watcher/                   fsnotify with 500ms debounce → trigger rebuild
  keycheck/                  API key resolution (env → keychain → config)
  changelog/                 Session report → markdown changelog generator
```

### LLM Model Assignments (Default: OpenRouter)

| Stage | Default Model | Alternatives |
|-------|---------------|--------------|
| `nqb init` interview | MiniMax M2.5 | claude-haiku, claude-sonnet |
| `nqb write` drafting | MiniMax M2.5 | claude-sonnet, any OpenAI-compatible |
| `nqb qa` semantic audit | MiniMax M2.5 | claude-sonnet |
| `nqb chat` editing REPL | Claude Opus 4-5 (via OpenRouter) | claude-opus (native) |
| `nqb research` | MiniMax M2.5 | Any model |
| Context assembly | No LLM | Pure Go templating |

### Git Auto-Commits

After each meaningful stage, nqb creates a git commit (if the book dir is a git repo):

```
init: "كتاب التاريخ العربي" — book initialized
context(01): Chapter 1 context assembled
chapter(01): Chapter 1 first draft
qa(01): Chapter 1 QA complete
export(pdf): PDF generated
```

---

## Roadmap

### Phase 1 ✅ (current)
- [x] Vault system with default + custom vaults
- [x] TUI home screen (VSCode-style project picker)
- [x] Book TUI (sidebar + slash command palette + help overlay)
- [x] Outline editor (reorder, rename, save)
- [x] Chapter preview with glamour
- [x] Init form with template picker (Arabic research / CS / General)
- [x] Multi-provider LLM support (OpenRouter, Anthropic, Bedrock)
- [x] Write, QA, Export, Watch, Chat, Status, Config commands
- [x] Research pipeline (Scout→Explorer→Scribe)
- [x] Google Docs sync/import
- [x] Batch job queue
- [x] Agent sessions
- [x] Vector store indexing
- [x] MCP server
- [x] Carapace shell completions
- [x] Keybinding hints on all screens

### Phase 4 ✅ (WeKnora Integration — scholarly text intelligence)
- [x] BM25 keyword store via Bleve (Arabic `lang/ar` analyzer)
- [x] Vector store interface (Chroma HTTP client; LanceDB/Zilliz stubs)
- [x] HybridStore: concurrent dispatch + dedup by content signature + MMR diversity
- [x] Chunker: recursive splitter with Arabic separators and protected patterns (hadith chains)
- [x] Embedder: OpenAI-compatible (Voyage AI, Jina, Ollama), Bedrock stub
- [x] Cross-encoder reranker: composite score + NullReranker fallback
- [x] Knowledge graph: 8 claim types, 8 relation types, BFS shortest path
- [x] EpistemicState: thesis + research questions + established claims, injected into agent prompts
- [x] Ingestion pipeline: chunk → contextualize → embed → upsert vector + keyword stores
- [x] DAG pipeline engine: topological sort, parallel batch execution, HUMAN_GATE blocking
- [x] Context stacks: 11 Arabic analytical layers, BraidedField AGREEMENT/CONFLICT/RESONANCE/SILENCE
- [x] Style engine: extract / apply (prompt + postprocess modes) / blend / diff / registry
- [x] `naqb-style` CLI binary
- [x] 2 new agent tools: `knowledge_search`, `grep_chunks`

### Phase 5 ✅ (Modularization & Runtime)
- [x] Extracted 11 `pkg/*` standalone Go modules with `go.mod`
- [x] Go workspace (`go.work`) for local cross-module development
- [x] LangGraph-style runtime (`pkg/runtime`): StateGraph, CompiledGraph, Invoke, InvokeParallel
- [x] `pkg/agent` refactored with `SessionStore` + `EpistemicStore` interfaces (decoupled from db/knowledge)
- [x] `pkg/booktools` plan/execute pipeline with checkpointing
- [x] GitHub Actions CI/CD: test matrix, lint, security (govulncheck, CodeQL), GoReleaser
- [x] `make check` covers root + all `pkg/*` modules

### Phase 2 (planned)
- [ ] Word count progress bars per chapter (NaNoWriMo-style)
- [ ] MkDocs Material web export (RTL, light/dark)
- [ ] VS Code / Zed extension for inline chapter editing
- [ ] Chapter diff view (before/after edits)
- [ ] Export themes (custom CSS for EPUB/Web)

---

## License

MIT

---

*نقب في أفكارك. اكتشف أعماقها. اكتبها.*
*Excavate your ideas. Discover their depths. Write them.*
