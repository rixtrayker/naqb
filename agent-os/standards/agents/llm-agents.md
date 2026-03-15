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

## Research Note Format

Every note saved by `Scribe` must have a YAML frontmatter header:

```markdown
---
title: "The Topic Title"
chapter: 1
tags: [research, ch-01]
source: "https://..."
date: "2026-03-15"
---

## The Topic Title
Content...
```

- Title: extracted from the first `##` heading in the note body
- Source: extracted from a `Source:` line or bare `http(s)://` URL in the body
- Tags: always `["research", "ch-NN"]` — enables future filtering by chapter
- Date: `time.Now().Format("2006-01-02")` at write time

This format is owned by `buildFrontmatter()` in `internal/research/scribe.go`.
Do not inline frontmatter construction elsewhere.

## Research Note Retrieval (Two-Tier)

`buildResearchNotes()` in `internal/agents/context_builder.go` uses two tiers:

| Tier | Condition | Implementation |
|---|---|---|
| 1 — Semantic | `OPENAI_API_KEY` or `MISTRAL_API_KEY` set | `store.QueryResearch()` via chromem-go embeddings |
| 2 — Keyword | Always available | `store.QueryResearch()` → `keywordSearch()` file scan |

`store.QueryResearch()` routes automatically: semantic when an embedder is
configured, file-scan keyword otherwise. The caller does not need to branch.

Keyword scoring in `keywordSearch()`:
- Heading match (`# …`): **3 points per query word**
- Body occurrence: **1 point per query word**
- Returns top-K by score descending

Do not add a Tier 3 or any other fallback without updating this standard.

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
