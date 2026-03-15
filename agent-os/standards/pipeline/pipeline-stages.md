# Pipeline Stage Patterns

## Stage Interface

A new piece of work should be a `Stage` if it:
- Has its own git commit (independent audit trail entry)
- Can be toggled on/off via `config/rules.yaml`

Otherwise, make it a helper function inside an existing stage.

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, in StageInput) (StageOutput, error)
    CommitMessage(chapterNum int) string // empty string = skip commit
}
```

Stages are stateless structs. All shared state passes through `StageInput`.

## Commit Message Prefixes

Commit messages reflect QA state. Use these prefixes:

| Prefix | Meaning |
|---|---|
| `context(NN):` | Context file assembled |
| `draft(NN):` | Chapter written, not yet QA'd |
| `reviewed(NN):` | QA stage passed |
| `conflict(NN):` | Cross-chapter conflict check done |
| `gap(NN):` | Outline gap analysis done |

```go
func (WriteStage) CommitMessage(n int) string {
    return fmt.Sprintf("draft(%02d): Chapter %d first draft", n, n)
}
func (QAStage) CommitMessage(n int) string {
    return fmt.Sprintf("reviewed(%02d): Chapter %d QA complete", n, n)
}
```

- Git commit is best-effort: pipeline continues if commit fails
- Every stage commits; advisory stages (conflict, gap) commit too as audit entries

## Conditional Stage Composition

Pipeline shape is data-driven. Use `DefaultStagesFor(rules)` not `DefaultStages`:

```go
// rules.yaml controls which stages run
func DefaultStagesFor(rules *config.Rules) []Stage {
    stages := []Stage{ContextStage{}, WriteStage{}, QAStage{}}
    if rules.QA.ConflictLevel != "off" {
        stages = append(stages, ConflictStage{Level: rules.QA.ConflictLevel})
    }
    // ...
    return stages
}
```

- New toggleable stages must be controlled by a field in `QARules` (rules.yaml)
- Levels follow: `off | light | moderate | max`
