# DAG Pipeline Engine

## The DAG Model

A pipeline is a **directed acyclic graph**, not a linear list of stages.

- Stages are nodes; dependencies are edges (`depends_on` field in template YAML)
- Fan-out (one stage → multiple parallel branches) and fan-in (multiple branches → one merge stage) are first-class constructs
- A project selects one template; the template defines the DAG shape
- The engine resolves execution order via topological sort at startup

The existing `Stage` interface (see `pipeline/pipeline-stages.md`) is extended with
dependency declarations. Stages remain stateless structs; all shared state passes
through `StageInput`.

## Stage Types

Every stage declares exactly one type. The type controls model routing, git commit
prefix, and HUMAN_GATE eligibility.

| Type | Purpose |
|---|---|
| `DECOMPOSE` | Break input into atomic units (paragraphs → claims, outline → sections) |
| `COMPARE` | Diff two or more inputs (edition variants, translation candidates) |
| `VERIFY` | Check a claim or segment against a reference (corpus, known facts) |
| `CORRECT` | Apply fixes to input (OCR errors, tashkeel normalization) |
| `TASHKEEL` | Add or critique Arabic diacritical marks |
| `CRITIQUE` | Analytical evaluation at one or more levels (internal / corpus / analytical) |
| `SYNTHESIZE` | Combine inputs into a new unified output (fan-in merge, corpus synthesis) |
| `COMPOSE` | Generate new text grounded in epistemic state |
| `QA` | Deterministic and LLM checks against quality rules |
| `HUMAN_GATE` | Pause point — blocking or advisory (see below) |

## HUMAN_GATE Rules

Each stage that can pause declares its gate type in the template:

| Gate type | Behavior |
|---|---|
| `blocking` | Pipeline stops. Emits `stage.blocked`. Resumes only when human provides input and calls `resume`. |
| `advisory` | Pipeline continues on the optimistic path. Human can override the output. Emits `stage.done` with an advisory flag. |

Rules:
- A stage cannot be both types simultaneously
- Stages that have neither gate type run fully automatically
- If a blocking gate is skipped (e.g., `--no-gates` flag), it becomes advisory for that run only — it must be logged

## Events

Every stage emits these events (to the job queue and any attached TUI/API consumers):

```
stage.started    { stage_id, run_id, timestamp }
stage.progress   { stage_id, run_id, pct float64, message string }
stage.done       { stage_id, run_id, output StageOutput, duration_ms int64 }
stage.blocked    { stage_id, run_id, prompt string }  // HUMAN_GATE blocking only
```

Events are written to the `jobs` table (`internal/db/`) and streamed over the API.

## Stage Declaration (Go)

```go
type StageDecl struct {
    ID          string        // matches template YAML id
    Type        StageType     // DECOMPOSE, COMPARE, etc.
    DependsOn   []string      // upstream stage IDs
    Model       string        // overrides template default; "" = use routing table
    HumanGate   GateType      // GateBlocking | GateAdvisory | GateNone
    Concurrency int           // max parallel executions; 0 = unlimited
}

type Stage interface {
    Name()          string
    Decl()          StageDecl
    Run(ctx context.Context, in StageInput) (StageOutput, error)
    CommitMessage(chapterNum int) string // "" = skip commit
}
```

`StageInput` carries: project epistemic state, upstream stage outputs (keyed by stage ID),
resolved context stack, model config, and a `DeclaredDebt` accumulator.

## Model Routing Per Stage Type

Default routing when a stage does not override `Model`:

| Stage types | Default model |
|---|---|
| `DECOMPOSE`, `CORRECT`, `TASHKEEL` (generation), `QA` (deterministic pass) | Haiku |
| `COMPARE`, `VERIFY`, `COMPOSE`, `QA` (LLM pass) | Sonnet |
| `CRITIQUE`, `SYNTHESIZE`, complex `COMPOSE`, نقد الإعراب | Opus |

Override at the stage level in template YAML. Override at the project level in
`book.yaml` LLM settings. See `agents/llm-agents.md` for the full routing precedence.

## Template Definition (YAML)

Templates live in `~/.naqb/templates/<name>.yaml` or shipped as embedded files.

```yaml
name: classical-tahqeeq
description: Critical edition pipeline with apparatus
inherits: null          # or another template name

defaults:
  model: sonnet         # stage-level override wins
  concurrency: 2

stages:
  - id: ocr-correct
    type: CORRECT
    model: haiku
    human_gate: advisory
    concurrency: 4

  - id: tashkeel-generate
    type: TASHKEEL
    depends_on: [ocr-correct]
    model: haiku
    human_gate: none

  - id: irab-critique
    type: CRITIQUE
    depends_on: [tashkeel-generate]
    model: opus
    human_gate: blocking
    label: "Review نقد الإعراب before tahqeeq"

  - id: tahqeeq
    type: COMPOSE
    depends_on: [irab-critique]
    model: opus
    human_gate: blocking

  - id: compare-editions
    type: COMPARE
    depends_on: [tahqeeq]
    concurrency: 8

  - id: apparatus-generate
    type: SYNTHESIZE   # fan-in: tahqeeq + compare-editions
    depends_on: [tahqeeq, compare-editions]
    model: sonnet
    human_gate: none
```

Fan-in is implicit: any stage with multiple `depends_on` entries receives all
upstream outputs in `StageInput.Upstreams`.

## Context Debt Rule

If a stage cannot satisfy its required context layers, it must choose one of four
declared behaviors — never silently proceed without disclosure:

| Behavior | When to use |
|---|---|
| `FAIL` | Context is mandatory and no substitute exists |
| `DEGRADE` | Proceed with reduced capability; add uncertainty markers to output; log warning |
| `SUBSTITUTE` | Use a declared fallback layer and note the substitution |
| `HUMAN_GATE` | Escalate: ask the human to supply the missing context |

The chosen behavior is declared in the stage's context stack configuration, not
hardcoded in stage logic. Accumulated debt across a pipeline run is reported in
the run summary and appended to `processing_log` in the epistemic state.

See `context/context-stacks.md` for how stacks declare debt resolution policies.
