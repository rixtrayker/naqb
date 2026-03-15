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
