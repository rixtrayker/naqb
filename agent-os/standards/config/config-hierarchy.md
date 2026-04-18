# Configuration Hierarchy

## Three-Tier Model

| File | Scope | Required | Purpose |
|---|---|---|---|
| `~/.naqb/config.yaml` | Global | No | API keys, named providers |
| `book.yaml` | Project | **Yes** | Book identity: title, author, chapters, LLM model overrides |
| `config/rules.yaml` | Project | No | Editorial behavior: word counts, QA levels, research settings |

**book.yaml = what the book is. rules.yaml = how to process it.**

- `book.yaml` is set once at `nqb init` and rarely changes
- `rules.yaml` is tuned during writing
- Never add processing/behavior config to `book.yaml`; never add identity fields to `rules.yaml`

## Fail-Safe Defaults

`config/rules.yaml` is optional. `LoadRules()` always returns a valid `*Rules`:

```go
func LoadRules(bookDir string) (*Rules, error) {
    r := &Rules{
        WordCount: WordCountRules{Min: 1500, Max: 5000, Target: 3000},
    }
    // returns defaults if file is missing, fills zero values if file is partial
}
```

- `book.yaml` is required. `LoadBook()` returns an error if missing.
- Only optional config (rules.yaml) uses the fail-safe default pattern.
- Zero values in a partial `rules.yaml` are filled in: `Target=3000`, `Min=Target/2`, `Max=Target*3`

## Storage Format Rules

Book content stays in plain files, git-tracked. Agent state lives in SQLite.

| Format | Used for | Who writes it |
|---|---|---|
| YAML | Config, metadata, book manifest, vault registry | Humans + `nqb init` |
| Markdown | Chapters, research notes, context files | LLM + Scribe |
| SQLite | Agent sessions, job queue (`~/.naqb/naqb.db`) | `internal/db/` only |

- Book files (chapters, contexts, research notes) → always plain Markdown
- SQLite is the single exception: sessions + job queue need atomic state transitions
- `~/.naqb/naqb.db` is **not** per-book — it is global (shared across all books)
- Per-book data that fits in a file uses a file; only volatile/indexed state goes to DB
- Vault stays YAML until 50+ projects make it slow (not yet)
- `modernc.org/sqlite` — PRAGMA WAL + FK must be set via `db.Exec()` after `sql.Open()`, not DSN

## Provider Override Levels

LLM provider resolution order (most specific wins):

```
CLI --provider flag
  → book.yaml LLM.WriteProvider / QAProvider / ChatProvider
    → GlobalConfig.DefaultProvider
      → Legacy ANTHROPIC_API_KEY env var
```

```go
// Always use providerFor(), never call llm.New(key) directly in commands
client, err := providerFor(providerFlag, cfg.LLM.WriteProvider)
```
