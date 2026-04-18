# Project Epistemic State

## What It Is

A persistent, growing, queryable object that lives for the duration of a project.
It is the shared foundation for every pipeline stage, every paragraph operation,
and every LLM call. It is never manually edited — it is built and updated by the
pipeline.

## Schema

```go
type EpistemicState struct {
    // Set at project init — rarely change
    ResearchQuestions []string              // what the book tries to answer
    Thesis            string                // the central argument
    Outline           []ChapterNode         // chapter structure with semantic annotations
    AuthorProfile     AuthorProfile         // known positions, StyleImagePath string

    // Grows as pipeline runs
    EstablishedClaims []Claim               // extracted claims (see knowledge-graph.md)
    CorpusView        CorpusView            // agreements, disagreements, gaps from external corpus
    AnalyticalStance  AnalyticalStance      // perspective, KnownDisagreements, OpenQuestions

    // Audit trail — append-only
    ProcessingLog     []LogEntry            // what was done to what paragraph, with outputs
}

type ChapterNode struct {
    Number      int
    Title       string
    Establishes []string  // semantic annotations: what this chapter proves or introduces
    DependedOnBy []int    // chapter numbers that rely on this one
}

type CorpusView struct {
    Agreements    []CorpusRelation
    Disagreements []CorpusRelation
    Gaps          []string          // questions the corpus doesn't address
}

type LogEntry struct {
    Paragraph   ParagraphRef
    Lens        LensType
    StageID     string
    Timestamp   time.Time
    Output      string    // the lens output, stored verbatim
    ClaimsAdded []string  // Claim IDs extracted during this operation
}
```

## How It Is Built

The epistemic state is **never** populated manually.

| Source | What it populates |
|---|---|
| `nqb init` / project init | `ResearchQuestions`, `Thesis`, `Outline` skeleton |
| `corpus-builder` template | `CorpusView` — runs before any writing stages |
| `DECOMPOSE` stage | `Outline[N].Establishes`, grows `EstablishedClaims` |
| Any paragraph lens operation | Appends to `ProcessingLog`, may add new `EstablishedClaims` |

## The Five Lenses

A lens is a typed operation on a single paragraph, executed through the full epistemic
context. Each lens has a defined input contract, LLM prompt pattern, and output storage target.

### EXPLAIN
Situates the paragraph within the full epistemic context. References what prior chapters
established (`Outline[N].Establishes`). Does not evaluate — only contextualizes.
- Input: paragraph text + full Outline + relevant EstablishedClaims (top-K)
- Output: stored in `ProcessingLog` as `lens=explain`

### SUMMARIZE
Delta summary — what is genuinely new in this paragraph relative to `EstablishedClaims`?
Suppresses restatements of already-established points.
- Input: paragraph text + top-K EstablishedClaims (semantic match)
- Output: stored in `ProcessingLog` as `lens=summarize`

### CRITIQUE
Three-level analytical evaluation:

| Level | Against what |
|---|---|
| Internal | The book's own argument (`Thesis` + `EstablishedClaims`) |
| Corpus | External sources (`CorpusView.Disagreements`) |
| Analytical | The project's declared `AnalyticalStance` |

- Output: stored in `ProcessingLog` as `lens=critique`; critique flags are available to REWRITE

### REWRITE
Rewrites the paragraph to a target style while preserving argumentative integrity.
- Requires: `AuthorProfile.StyleImagePath` (style image must exist — see `style/style-engine.md`)
- Checks: rewritten text must still advance `ResearchQuestions`; must not contradict `EstablishedClaims`
- Output: stored in `ProcessingLog` as `lens=rewrite`; reads prior `lens=critique` flags if present

### CORPUS-ADD
Extracts atomic claims from the paragraph, deduplicates against `EstablishedClaims`,
and links to `CorpusView`. Used during corpus ingestion, not during writing.
- Deduplication: embed new claim → search existing claims → if cosine similarity > 0.92: UPDATE or NOOP
- Output: new/updated claims written to knowledge store; `ClaimsAdded` appended to `LogEntry`

## Lenses Accumulate (Critical Rule)

Output of one lens is stored in `ProcessingLog` and **must be injected** into
subsequent lens calls on the same paragraph. Never discard lens output mid-paragraph.

```
EXPLAIN → CRITIQUE  (critique reads the explanation)
CRITIQUE → REWRITE  (rewrite reads the critique flags)
SUMMARIZE → CORPUS-ADD  (dedup uses the summary to detect overlap)
```

The pipeline stage that runs multiple lenses on one paragraph is responsible for
passing prior `LogEntry` outputs via `StageInput`.

## Token Architecture

Static fields (rarely change within a session):
- `ResearchQuestions`, `Outline`, `AuthorProfile` → always inject as cached prefix
- Cost: 10% of face value after the first call in a session
- Target: ≤ 2,000 cached tokens for this block

Dynamic fields (retrieved per paragraph):
- `EstablishedClaims` → top-K semantic match to current paragraph (never full list)
- Corpus passages → retrieved from `CorpusView` via three-tier retrieval
- Target: ≤ 1,500 retrieved tokens for this block

Total effective token budget per lens call: **~3,500 tokens**.
Calls exceeding this must be justified in code comments. Never inject the full
epistemic state as a raw dump.
