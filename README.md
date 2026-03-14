# نقب — nqb

> **نقب** (naqb) — *to excavate, to tunnel through, to investigate deeply.*
> An archaeologist نقّب عن الآثار. A writer نقّب في الأفكار.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)](#)

> From spark to shelf — CLI that orchestrates Claude through the full book lifecycle:
> interview → outline → write → QA → export. Built for depth thinkers.

`nqb` is an LLM-powered CLI tool for writing structured, ADHD-friendly books.
It manages the full lifecycle: interview → outline → context → write → QA → export.
Primary focus: **Arabic RTL books**, with full support for English/technical books too.

---

## Table of Contents

- [Install](#install)
- [Quick Start](#quick-start)
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

**Requirements:** Go 1.21+, pandoc (for export), XeLaTeX (for PDF)

```bash
# Clone and build
git clone https://github.com/rixtrayker/naqb
cd naqb
go build -o nqb ./cmd/nqb

# Move to PATH
mv nqb /usr/local/bin/nqb

# Set your Anthropic API key
nqb config --set-key
# or: export ANTHROPIC_API_KEY=sk-ant-...
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

# Open the current directory as a book project
nqb .

# Open a specific book from your vault by name
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

### `nqb .` — open current directory

```bash
cd ~/my-arabic-research
nqb .         # Opens book TUI if book.yaml exists
              # Asks "Initialize here?" if it doesn't
```

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

Opens when you select a project from home or run `nqb .` / `nqb open <name>`:

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

All commands also work directly from the terminal (without entering the TUI):

| Command | Description |
|---------|-------------|
| `nqb` | Open TUI home screen |
| `nqb .` | Open current dir as book (or prompt init) |
| `nqb open <name>` | Open book from vault by name or path |
| `nqb init` | Initialize new book via LLM interview |
| `nqb context --chapter N` | Build context file for chapter N |
| `nqb write --chapter N` | Write chapter N with Claude Sonnet (spinner) |
| `nqb write --chapter N --stream` | Write with live streaming output |
| `nqb qa --chapter N` | Deterministic + LLM QA audit |
| `nqb qa --chapter N --deterministic-only` | Skip LLM audit |
| `nqb pipeline --chapter N` | Run context→write→qa for chapter N |
| `nqb pipeline --all` | Run full pipeline for all chapters |
| `nqb export --format pdf` | Export to PDF (RTL Arabic via XeLaTeX) |
| `nqb export --format all` | Export all formats |
| `nqb watch` | Watch for changes, auto-rebuild exports |
| `nqb chat` | Open Opus chat REPL with book context |
| `nqb chat --chapter N` | Chat focused on chapter N |
| `nqb status` | Chapter progress table + git log |
| `nqb config` | Show global config |
| `nqb config --set-key` | Set Anthropic API key |
| `nqb vault list/add/remove` | Manage vaults |
| `nqb completion bash\|zsh\|fish` | Shell completions |

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
├── research/               ← drop your notes, PDFs, references here
├── assets/
│   └── themes/
│       ├── light.css
│       └── dark.css
├── output/                 ← generated files (gitignored)
│   ├── book.pdf
│   ├── book.epub
│   └── web/
└── pipeline-report.md      ← QA results and run summaries
```

---

## Architecture

```
cmd/nqb/main.go              Entry point — cobra root + dynamic completions
internal/
  vault/                     Vault registry (~/.naqb/vault.yaml), project scanning
  config/                    GlobalConfig, BookConfig, Template definitions
  llm/                       Anthropic SDK wrapper (streaming + non-streaming)
  agents/
    planner.go               Stage 0: interview answers → BookConfig + outline.md
    context_builder.go       Stage 1: golden prompt assembler → contexts/ch-XX.md
    writer.go                Stage 2: context file → LLM → chapters/ch-XX.md
    qa.go                    Stage 3: deterministic checks + LLM semantic audit
  pipeline/                  Orchestrator (stages 1-3) + git auto-commit
  exporter/                  Pandoc wrappers: PDF (RTL), EPUB, DOCX, Web
  watcher/                   fsnotify with 500ms debounce → trigger rebuild
  tui/
    keys.go                  Central keybinding definitions + hint renderer
    home.go                  VSCode-style project picker
    book_view.go             Book TUI: sidebar + slash command palette
    outline_editor.go        Visual chapter outline editor
    preview.go               glamour markdown renderer (scrollable viewport)
    chat.go                  Streaming Opus chat REPL
    init_chat.go             Multi-step init form with template picker
    spinner.go               Bubble Tea spinner wrapper
  commands/                  One file per CLI subcommand
```

### LLM Model Assignments

| Stage | Model | Reason |
|-------|-------|--------|
| `nqb init` interview | `claude-haiku-4-5` | Fast, interactive |
| `nqb write` drafting | `claude-sonnet-4-6` | Best long-form quality |
| `nqb qa` semantic audit | `claude-sonnet-4-6` | Good reasoning |
| `nqb chat` editing REPL | `claude-opus-4-6` | Deep edits, nuanced |
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
- [x] Write, QA, Export, Watch, Chat, Status, Config commands
- [x] Carapace shell completions
- [x] Keybinding hints on all screens

### Phase 2 (planned)
- [ ] Word count progress bars per chapter (NaNoWriMo-style)
- [ ] Research automation (Scout/Explorer/Scribe agents)
- [ ] MkDocs Material web export (RTL, light/dark)
- [ ] Multi-LLM option (opt-in Gemini, GPT-4o)
- [ ] VS Code / Zed extension for inline chapter editing
- [ ] Chapter diff view (before/after edits)
- [ ] Export themes (custom CSS for EPUB/Web)

---

## License

MIT

---

*نقب في أفكارك. اكتشف أعماقها. اكتبها.*
*Excavate your ideas. Discover their depths. Write them.*
