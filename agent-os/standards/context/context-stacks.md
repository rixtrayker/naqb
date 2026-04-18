# Context Stacks

## What a Context Stack Is

A **named, versioned, composable unit** — not a system prompt.

A stack declares:
- An ordered set of layers (positions 0–5)
- A merge policy
- A conflict resolution policy
- A preferred processing unit (`word` | `sentence` | `paragraph` | `chapter`)

Stacks are never injected in full. See `context/retrieval-patterns.md` Rule 7 for
the progressive disclosure pattern that governs how stacks are selected and loaded.

## Layer Positions

| Position | Role | Purpose |
|---|---|---|
| 0–1 | Foundational | Prerequisite knowledge, background, provenance |
| 2–3 | Interpretive | How we see: grammar, semantics, rhetoric, intertextuality |
| 4–5 | Analytical | What we ask: consistency, authenticity, coherence, style match |

Higher positions may depend on lower positions being satisfied. A layer at position 4
that requires a position-2 layer absent from the stack must declare that dependency
explicitly — it cannot assume it.

## Layer Types

```go
type LayerType string
const (
    LayerKnowledge    LayerType = "knowledge"    // factual background
    LayerRules        LayerType = "rules"         // grammatical or editorial constraints
    LayerTool         LayerType = "tool"          // a callable tool or microservice
    LayerTemplate     LayerType = "template"      // an output template or format
    LayerExample      LayerType = "example"       // few-shot examples
    LayerValidation   LayerType = "validation"    // post-generation checks
    LayerStyle        LayerType = "style"         // style image (see style/style-engine.md)
)
```

## Stack Declaration (YAML)

```yaml
name: tahqeeq-detailed
version: "1.0"
description: "Full critical edition stack with apparatus support"
inherits: tahqeeq       # optional; overrides + extends parent
processing_unit: paragraph
conflict_resolution: ANNOTATE_BOTH

layers:
  - position: 0
    type: knowledge
    id: manuscript-census
  - position: 1
    type: knowledge
    id: isnād-chain
  - position: 2
    type: rules
    id: classical-grammar-rules
  - position: 3
    type: knowledge
    id: diacritical-variants
  - position: 4
    type: knowledge
    id: allusion-recognition
  - position: 5
    type: validation
    id: qa-grounding
```

## Stack Composition Rules

- Stacks can **inherit** from a parent stack; child overrides specific positions and
  may extend with additional layers
- Stacks are **versioned**; projects pin a specific version in `book.yaml`
- **Fork** a stack to create a mutation that tracks its parent; forks carry a
  `forked_from` field
- Two stacks with the same `name` at different versions are distinct artifacts

```go
// Pin in book.yaml
type StackPin struct {
    Name    string `yaml:"name"`
    Version string `yaml:"version"`  // "" = latest
}
```

## The Braided Field

When multiple stacks process the same paragraph, they run as a **braided field**.

**Execution:**
- Each stack runs as an independent strand — parallel, no shared state during execution
- Strands do not see each other's intermediate outputs

**After all strands complete — braid analysis:**
- Scan strand outputs for **braid points**: positions in the text where strands interact

| Braid point type | Definition |
|---|---|
| `AGREEMENT` | Two or more strands reach the same conclusion independently — strong signal |
| `CONFLICT` | Strands contradict — surface both, never hide, never auto-merge |
| `RESONANCE` | Strands amplify the same theme from different angles |
| `SILENCE` | One strand speaks; others are silent on this point |

**Synthesis pass:**
A final LLM call receives ALL strand outputs plus the interference map. It never
receives a pre-collapsed version. The synthesis call must preserve CONFLICT markers
in its output.

```go
type BraidPoint struct {
    Position    int           // character offset in paragraph
    Type        BraidType     // AGREEMENT | CONFLICT | RESONANCE | SILENCE
    Strands     []StrandID
    Quote       string        // relevant excerpt from paragraph
}
```

## Conflict Resolution Policy

Declared per stack. Applied during the synthesis pass.

| Policy | Behavior |
|---|---|
| `FAIL_FAST` | Stop on first contradiction — do not proceed to synthesis |
| `ANNOTATE_BOTH` | Include both interpretations in output; mark with `[CONFLICT: ...]` |
| `WEIGHTED_MERGE` | Merge by `Claim.Confidence` score; log the merge decision |
| `HUMAN_REVIEW` | Flag conflict for human; emit `stage.blocked`; do not auto-resolve |

## Context Debt Rule

When a required layer cannot be satisfied, the stack must declare and act on debt.
The same four behaviors as the DAG engine apply here:

| Behavior | Declaration field |
|---|---|
| `FAIL` | `on_missing: fail` |
| `DEGRADE` | `on_missing: degrade` — adds uncertainty markers to output |
| `SUBSTITUTE` | `on_missing: substitute` + `fallback_layer: <id>` |
| `HUMAN_GATE` | `on_missing: human_gate` |

Accumulated debt is written to `EpistemicState.ProcessingLog` and the pipeline run
summary. See `pipeline/dag-engine.md` for the pipeline-level debt handling.

## Standard Stack Library

These stacks ship with the system as embedded YAML files:

| Stack name | Purpose |
|---|---|
| `tashkeel-basic` | Generate tashkeel for modern Arabic text |
| `tashkeel-critique` | Audit existing tashkeel; flag errors |
| `tahqeeq` | Core critical edition (base stack) |
| `tahqeeq-detailed` | Full apparatus support; inherits `tahqeeq` |
| `ocr-correction` | Fix OCR artifacts in scanned manuscripts |
| `stylometric` | Authorship and style fingerprinting |
| `comparative` | Cross-tradition idea comparison |
| `rhetorical` | Rhetoric and balāgha analysis |
| `translation-prep` | Pre-translation analysis layer |
| `summarize` | Layered summarization |
| `qa-grounding` | Factual consistency checks |
| `style-apply` | Apply a style image to rewritten text |

## Arabic-Specific Layers

Available as named layers in any stack. These are loaded from the embedded layer
library; a stack references them by ID.

| Layer ID | What it provides |
|---|---|
| `isnād-chain` | Transmission chain analysis for hadith and classical texts |
| `manuscript-census` | Known manuscript witnesses and their variants |
| `diacritical-variants` | Variant readings introduced by different tashkeel |
| `classical-grammar-rules` | Basran/Kufan grammar rules for نقد الإعراب |
| `etymological-field` | Root-based semantic field mapping |
| `rhetorical-formulae` | Balāgha patterns (سجع، جناس، طباق، مقابلة) |
| `ijāza-chain` | Authorization chain for transmitted texts |
| `semantic-shift` | Historical meaning drift detection |
| `allusion-recognition` | Qur'anic and classical intertextual allusions |
| `stylometric-fingerprint` | Statistical style signature for authorship attribution |

## Retrieval Rule

Never inject all stacks into a stage. The selection process is mandatory:

1. Store stack **descriptions** (~1 sentence each) in the vector store
2. Store full stack **definitions** in the docstore
3. At stage execution: embed `(paragraph_text + task_description)` → retrieve top-K description matches
4. Fetch full definitions for the top-K matches only → inject into stage context

This keeps the stack library arbitrarily large while only paying token cost for
activated stacks. See `context/retrieval-patterns.md` Rule 7 for the full pattern.
