# UI Refactoring & Future Modularity Plan

## Current State (2026-04-18)

Since the original UI refactor, the project has undergone a major structural shift from a monolithic `internal/` layout to a hybrid `internal/` + `pkg/*` module architecture.

### What Happened

- **UI refactoring (Part 1)** — `internal/tui/` was reorganized into `theme/`, `keys/`, `components/`, screens, etc. ✅
- **11 packages extracted to `pkg/*`** as standalone Go modules with their own `go.mod` files:
  - `pkg/agent`, `pkg/agents`, `pkg/booktools`, `pkg/config`, `pkg/llm`, `pkg/log`, `pkg/pipeline`, `pkg/research`, `pkg/runtime`, `pkg/search`, `pkg/wordcount`, `pkg/youtube`
- **`go.work` + root `replace` directives** enable local development across all workspace modules.
- **LangGraph runtime extracted** — `pkg/runtime` is fully functional and provides the core `Runnable`, `Tool`, `StateGraph`, and `Checkpointer` abstractions used by `pkg/agent`.
- **`pkg/agent` now uses interfaces** — `SessionStore` and `EpistemicStore` are defined in `pkg/runtime` and injected into the agent, making the package testable without a real database.
- **CI/CD pipeline** — GitHub Actions workflow exists and `make check` covers all workspace modules.

### What's Deferred

- **`internal/app` extraction (Phase A)** — not done; `internal/commands` is still the main orchestrator, though some logic was cleaned up.
- **`config` split into `domain`/`persistence`/`bootstrap` (Phase C)** — not done; `pkg/config` still handles structs, file I/O, keychain, provider resolution, and templates.
- **Eliminating global mutable state (Phase D)** — not done; `pkg/llm` still contains `SessionBudget` and `ActivePricingTier` as package-level globals.
- **`pkg/agents` vs `pkg/agent` duality (Phase B)** — partially resolved with interfaces in `pkg/agent`, but `pkg/agents` still exists alongside it and `pkg/pipeline.WriteStage` still branches on `in.FantasyProvider != nil`.
- **Sidebar data access cleanup (Phase E)**, **search layer consolidation (Phase F)**, and **wizard component improvement (Phase G)** — not started.

---

## Part 1: What Was Done (Immediate UI Refactor) ✅

### 1.1 New Package Structure

The monolithic `internal/tui` package has been reorganized into a layered structure:

```
internal/tui/
├── theme/              # Centralized colors, styles, render helpers
│   ├── colors.go
│   ├── styles.go
│   └── render.go
├── keys/               # Canonical keybindings and help overlays
│   ├── bindings.go
│   └── help.go
├── components/         # Reusable Bubble Tea widgets
│   ├── spinner.go      # CLI spinner for long-running tasks
│   ├── tracker.go      # Background task tracker
│   ├── chat.go         # Shared streaming chat primitive
│   └── wizard.go       # Shared multi-step form primitive
├── screen_home.go
├── screen_book.go
├── screen_chat.go
├── screen_agentchat.go
├── screen_outline.go
├── screen_preview.go
├── screen_init.go
├── screen_import.go
├── sidebar.go
├── palette.go          # Slash-command dispatch (decoupled from handlers)
├── handlers.go         # Built-in command handlers
└── app.go              # Public API and orchestration helpers
```

### 1.2 Key Improvements

| Area | Before | After |
|------|--------|-------|
| **Styles** | Scattered per-file inline styles | Centralized in `theme/` package |
| **Keybindings** | Single `keys.go` with mixed concerns | Split into `keys/bindings.go` (data) and `keys/help.go` (rendering) |
| **Chat** | ~80% duplication between `chat.go` and `agent_chat.go` | Shared `components.ChatBase` for viewport/textarea/stream management |
| **Forms** | `init_chat.go` and `import_form.go` duplicated step logic | Shared `components.WizardModel` for multi-step navigation |
| **Spinner** | In `tui/spinner.go`, imported by `commands/` | Moved to `components/spinner.go`; `commands/` now imports `components` |
| **Task Tracker** | In `tui/tasktracker.go` | Moved to `components/tracker.go` |
| **Palette** | Business logic (`agents` calls) mixed in `commands.go` | `palette.go` is pure dispatch; `handlers.go` contains the business logic |

### 1.3 Decoupling Wins

- **PaletteHandler interface**: `BookViewModel` now depends on an interface rather than hardcoded command handlers. This makes the book view testable without importing `pkg/agents`.
- **Default wiring preserved**: `RunBookView()` automatically wires `DefaultCommandRegistry` so existing callers in `commands/open.go` continue to work without changes.
- **Commands updated**: `commands/export.go`, `fix.go`, `qa.go`, and `write.go` now import `tui/components` instead of reaching through the entire `tui` package for `RunWithSpinner`.

### 1.4 What Was Preserved

- All public APIs (`RunHome`, `RunBookView`, `RunChat`, `RunAgentChat`, `RunOutlineEditor`, `RunPreview`, `RunInitForm`, `RunImportForm`) keep their original signatures.
- All existing tests pass without modification.
- User-facing behavior (keybindings, layouts, colors) is unchanged.

---

## Part 2: Recommended Next Steps (Architecture & Modularity)

### Step A — Extract a Pure Application Layer (`internal/app`) 🚧

**Status**: Partially done. Some logic in `internal/commands` was cleaned up, but `internal/commands` remains the main orchestrator.

**Problem**: `internal/commands` is a "god package" that imports 15+ internal packages. It mixes CLI flag parsing, provider resolution, TUI launching, and direct business logic calls.

**Solution**: Create `internal/app` as the single orchestration layer between CLI and core logic.

```
internal/app/
├── app.go            # App struct holding all services
├── books.go          # Book-scoped operations (open, write, qa, export)
├── vault.go          # Vault management
├── providers.go      # LLM provider resolution (moved from commands/provider.go)
└── background.go     # Background runner closures
```

**Migration path**:
1. Move `providerFor`, `RunPreflight`, and `buildBackgroundRunners` from `commands/` to `app/`.
2. Move `OpenDot`, `LaunchHome`, `openProject`, `openBookAt`, `openBookAtAgentChat` from `commands/open.go` to `app/`.
3. Have `commands/` delegate to `app/` methods instead of inline logic.
4. `mcpserver/` should also delegate to `app/` instead of duplicating provider resolution.

**Benefit**: `commands` becomes thin Cobra wrappers; `app` becomes testable without a CLI harness.

---

### Step B — Resolve the `pkg/agent` vs `pkg/agents` Duality 🚧

**Status**: Partially done. `pkg/agent` now uses interfaces (`runtime.SessionStore`, `runtime.EpistemicStore`), but `pkg/agents` still exists alongside `pkg/agent` and `pkg/pipeline.WriteStage` still branches on `in.FantasyProvider != nil`.

**Problem**: There are two overlapping agent packages:
- `pkg/agents` — legacy single-shot orchestration (`WriteChapter`, `RunQA`, etc.)
- `pkg/agent` — Fantasy loop agent with tools

`pkg/pipeline.WriteStage` branches on `in.FantasyProvider != nil`, indicating an incomplete migration.

**Solution**: Unify behind a single `Writer` interface.

```go
package writer

type ChapterWriter interface {
    WriteChapter(ctx context.Context, bookDir string, cfg *config.BookConfig, chNum int, onDelta func(string)) error
}

type LegacyWriter struct { client llm.Provider }
type FantasyWriter struct { provider fantasy.Provider; db *sql.DB }
```

**Migration path**:
1. Define `Writer`, `QAer`, `Researcher` interfaces in `pkg/pipeline` (or a new `internal/ops` package).
2. Implement both legacy and fantasy adapters.
3. Have `pkg/pipeline.WriteStage` depend on the interface, not the packages directly.
4. Gradually migrate callers from `pkg/agents.WriteChapter` to the interface.
5. Eventually deprecate and remove `pkg/agents`.

**Benefit**: Clear domain boundary; pipeline doesn't care which framework delivers the chapter.

---

### Step C — Break Up the `pkg/config` God Package 🚧

**Status**: Not done. `pkg/config` still handles everything.

**Problem**: `pkg/config` handles:
- Domain structs (`BookConfig`, `Chapter`, `LLMSettings`)
- File I/O (`LoadBook`, `SaveBook`, `LoadGlobal`, `SaveGlobal`)
- Keychain integration
- Provider resolution helpers
- Template management
- Rules loading

**Solution**: Split into three packages:

```
internal/domain/         # Pure structs, no I/O, no third-party deps
├── book.go              # BookConfig, Chapter, LLMSettings
├── global.go            # GlobalConfig, ProviderConfig
└── rules.go             # Rules, Template

internal/persistence/    # File I/O, YAML, SQLite setup
├── book_yaml.go
├── global_yaml.go
└── keychain.go

internal/bootstrap/      # Provider resolution, template registry
├── providers.go
└── templates.go
```

**Migration path**:
1. Start by moving pure structs to `internal/domain`.
2. Update all packages to import `domain` instead of `config` for structs.
3. Move I/O functions to `persistence`.
4. Move provider/template logic to `bootstrap`.
5. Delete `pkg/config` once all references are migrated.

**Benefit**: Domain model becomes importable by any layer without dragging in YAML parsers and keychain code.

---

### Step D — Eliminate Global Mutable State 🚧

**Status**: Not done.

**Problem**: `pkg/llm` contains:
- `SessionBudget` (global budget tracker)
- `ActivePricingTier` (global pricing mode)

These make concurrent operations and tests fragile.

**Solution**: Make them fields on a `Session` or `ClientConfig` struct.

```go
type Session struct {
    Budget       *Budget
    PricingTier  PricingTier
    Provider     Provider
}
```

**Migration path**:
1. Create `Session` struct in `pkg/llm`.
2. Change functions that touch `SessionBudget` to accept a `*Session`.
3. Have `commands/` create one `Session` per invocation and pass it down.
4. Remove the package-level vars.

**Benefit**: Testability and thread safety.

---

### Step E — Clean Up Sidebar Data Access 🚧

**Status**: Not started.

**Problem**: `sidebar.go` still directly calls `wordcount.CountFile`, `config.LoadRules`, `os.ReadFile`, and `exec.Command("git", ...)`.

**Solution**: Introduce a `BookInfoProvider` interface.

```go
type BookInfoProvider interface {
    WordCounts(bookDir string, chapters []config.Chapter) []ChapterWordCount
    QASummary(bookDir string) string
    GitLog(bookDir string, limit int) []string
    Notes(bookDir string) []string
    Todos(bookDir string) []string
}
```

**Migration path**:
1. Define the interface in `tui/`.
2. Create `defaultBookInfoProvider` in `app/` or `tui/`.
3. Inject it into `BookViewModel` instead of letting the view reach out to the filesystem.

**Benefit**: Sidebar becomes purely presentational; no file I/O or exec calls in the UI layer.

---

### Step F — Consolidate Search Layers 🚧

**Status**: Not started.

**Problem**: Both `pkg/search` and `internal/store` exist and are used independently. The architecture doc says `search` is the "routing layer," but in practice `pkg/agent` (and formerly `agent/tools.go`) imports both.

**Solution**: Make `pkg/search` a thin facade over `internal/store`.

**Migration path**:
1. Have `pkg/search.Store` wrap `internal/store.HybridStore` instead of owning chromem directly.
2. Redirect `pkg/search.Open`, `pkg/search.IndexChapter`, etc. to the store backend.
3. Remove the duplicate chromem logic from `pkg/search`.
4. Eventually delete `pkg/search` and have callers use `internal/store` directly.

**Benefit**: One search abstraction, no duplicate backends.

---

### Step G — Improve the Shared Wizard Component 🚧

**Status**: Not started.

**Problem**: The `components.WizardModel` is a good start, but `screen_import.go` originally had a *dynamic* third step whose label and validation depended on the first step's answer. The current wizard doesn't support conditional steps well.

**Solution**: Extend `WizardModel` to support dynamic step injection.

```go
type WizardModel struct {
    Steps      []WizardStep
    Dynamic    func(currentType string) []WizardStep
    // ...
}
```

**Migration path**:
1. Add `OnStepComplete(step int, value string) []WizardStep` hook to `WizardModel`.
2. Re-implement `screen_import.go` on top of the extended wizard.
3. Remove the simplified fallback in `screen_import.go`.

**Benefit**: `init` and `import` forms share 100% of their navigation logic.

---

## Part 3: Target Architecture Vision

```
┌─────────────────────────────────────────────────────────────────────┐
│                         cmd/                                        │
│  nqb / nqb-mcp / naqb-style  ← thin entry points                   │
├─────────────────────────────────────────────────────────────────────┤
│                         commands/   mcpserver/                      │
│  CLI parsing, flag binding, help text  ← no business logic          │
├─────────────────────────────────────────────────────────────────────┤
│                         app/                                        │
│  Orchestration: provider resolution, session mgmt, TUI launching    │
├─────────────────────────────────────────────────────────────────────┤
│                         tui/                                        │
│  Pure presentation: screens, components, theme, keys                │
│  Depends only on domain/ for structs                                │
├─────────────────────────────────────────────────────────────────────┤
│                         pkg/pipeline                                │
│  Stage interface, DAG engine, debt/gate management                  │
├─────────────────────────────────────────────────────────────────────┤
│                         pkg/agent                                   │
│  Fantasy loop + tools  ← unified writer interface                   │
├─────────────────────────────────────────────────────────────────────┤
│                         internal/domain/                            │
│  BookConfig, Chapter, Rules, LLMSettings  ← zero external deps      │
├─────────────────────────────────────────────────────────────────────┤
│  pkg/llm  pkg/store  pkg/knowledge  pkg/context  pkg/style          │
│  pkg/exporter  pkg/research  pkg/runtime                            │
│  Each depends on domain/; no cycles; no globals                     │
└─────────────────────────────────────────────────────────────────────┘
```

**Key**: `pkg/*` modules are standalone Go modules with their own `go.mod`, versioned and importable independently. `internal/*` packages are implementation details private to the root module.

---

## Part 4: Quick Wins Checklist

If you want to continue refactoring incrementally, tackle these in order:

1. [ ] **Create `internal/app`** and move `providerFor` + `openBookAt*` functions there.
2. [ ] **Move `SessionBudget` and `ActivePricingTier`** off package globals and onto a `Session` struct in `pkg/llm`.
3. [ ] **Introduce `BookInfoProvider`** and stop `sidebar.go` from doing I/O directly.
4. [ ] **Unify `pkg/agent` vs `pkg/agents`** behind a `ChapterWriter` interface in `pkg/pipeline`.
5. [ ] **Split `pkg/config`** into `internal/domain/` + `internal/persistence/` + `internal/bootstrap/`.
6. [ ] **Make `pkg/search` a facade** over `internal/store` and remove duplicate chromem code.
7. [ ] **Extend `WizardModel`** to support dynamic steps and fully rewrite `screen_import.go`.

---

## Appendix: Files Changed in the UI Refactor

### New files
- `internal/tui/theme/colors.go`
- `internal/tui/theme/styles.go`
- `internal/tui/theme/render.go`
- `internal/tui/keys/bindings.go`
- `internal/tui/keys/help.go`
- `internal/tui/components/spinner.go`
- `internal/tui/components/tracker.go`
- `internal/tui/components/chat.go`
- `internal/tui/components/wizard.go`
- `internal/tui/screen_*.go` (8 screen files)
- `internal/tui/palette.go`
- `internal/tui/handlers.go`
- `internal/tui/app.go`
- `docs/refactoring-plan.md`

### Deleted files
- `internal/tui/styles.go`
- `internal/tui/keys.go`
- `internal/tui/spinner.go`
- `internal/tui/tasktracker.go`
- `internal/tui/home.go`
- `internal/tui/book_view.go`
- `internal/tui/chat.go`
- `internal/tui/agent_chat.go`
- `internal/tui/outline_editor.go`
- `internal/tui/preview.go`
- `internal/tui/init_chat.go`
- `internal/tui/import_form.go`
- `internal/tui/commands.go`

### Updated files
- `internal/commands/export.go`
- `internal/commands/fix.go`
- `internal/commands/qa.go`
- `internal/commands/write.go`
