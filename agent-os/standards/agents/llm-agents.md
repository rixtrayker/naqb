# LLM Agent Patterns

## Go Template Prompts

Use `text/template` for any prompt with a structured context (4+ dynamic fields). Plain string formatting for simple one-off calls.

```go
// Structured context — use template
var contextTemplate = `...{{.Title}}...{{if .FinishedSummaries}}...{{end}}`

// One-off call — plain string
userMsg := fmt.Sprintf(`Book: %s | Chapter: %d\n\n%s`, cfg.Title, chNum, rawText)
```

- `contextTemplate` is the golden prompt for chapter writing — do not inline it
- Each chapter may also have a custom context override file at `contexts/ch-XX-context.md`
- Prompts live as constants in the same file as the agent that owns them

## Graceful LLM Output Fallbacks

Never return an error when the LLM returns malformed output. Degrade gracefully:

```go
// Parse expected format, build fallback if parsing fails
config, outline, err := parseResponse(resp)
if err != nil {
    config = buildFallbackConfig(answers)
    outline = buildFallbackOutline(config)
}
```

- A partial/synthetic result is always better than crashing the pipeline
- Log a warning when falling back: `log.Warn("planner: falling back to synthetic outline")`
- Fallback functions must be named `buildFallback<Thing>()`

## Per-Stage Model Routing

Match model cost to task stakes:

| Task type | Model | Rationale |
|---|---|---|
| Utility/scaffolding (query gen, classification) | `llm.ModelHaiku` | Fast, cheap, structured |
| Content quality (writing, QA, editing) | `llm.ModelSonnet` | Balance of quality/cost |
| Interactive / open-ended (chat) | `llm.ModelOpus` | Best reasoning, user is waiting |

```go
// Always read from config first, fall back to constant
model := cfg.LLM.WriteModel
if model == "" {
    model = llm.ModelSonnet
}
```

- book.yaml LLM settings always override agent defaults
- Add new model constants to `internal/llm/models.go`, not inline strings

## Semantic Output Validation

For advisory agents (conflict, gap analysis), use keyword heuristics to classify LLM output rather than structured output:

```go
func looksLikeConflict(findings string) bool {
    lower := strings.ToLower(findings)
    for _, n := range []string{"no conflict", "no contradiction"} {
        if strings.Contains(lower, n) { return false }
    }
    // ... check positive keywords
}
```

- Check negative keywords first ("no conflicts") before positive ones
- Used only for advisory outputs that don't block the pipeline
- Never use heuristics for outputs that gate further processing
