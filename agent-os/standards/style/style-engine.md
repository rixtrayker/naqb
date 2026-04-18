# Style Engine

## What a Style Image Is

A **portable, versioned YAML artifact** capturing an author's voice at a measurable,
reproducible level of fidelity. Stored in `~/.naqb/styles/<name>@<version>.yaml`.

Treat it like a Docker image: extractable from any text, applicable to any compatible
text, blendable with other images, forkable, and diffable.

## Five Dimensions (All Required)

A style image is invalid if any dimension is missing or empty. Validate with
`naqb-style validate` before first use.

### 1. `linguistic_profile`

```yaml
linguistic_profile:
  vocabulary_size: 12400        # unique lemma count in source corpus
  type_token_ratio: 0.61
  avg_sentence_length: 22.4     # words
  passive_voice_ratio: 0.18
  parallelism_score: 0.74       # 0–1; structural parallelism frequency
  # Arabic-only fields:
  arabic_binyan_freq:           # map from binyan (verb pattern) to frequency
    فَعَلَ: 0.31
    أَفْعَلَ: 0.14
  diacritical_density: 0.42     # diacritized chars / total chars in source
```

### 2. `structural_profile`

```yaml
structural_profile:
  heading_hierarchy: [h2, h3]   # heading levels actually used
  avg_paragraph_length: 4.8     # sentences
  topic_sentence_position: first  # first | last | middle | variable
  list_ratio: 0.08              # paragraphs that are lists / total paragraphs
  emphasis_preferences: [bold]  # bold | italic | underline | none
  callout_ratios:               # map from callout type to frequency
    note: 0.12
    warning: 0.02
```

### 3. `rhetorical_patterns`

A list of named patterns. Each pattern requires at minimum: name, confidence, description,
and 3–5 verbatim examples from the source text.

```yaml
rhetorical_patterns:
  - name: "threefold_enumeration"
    confidence: 0.88
    description: "Presents arguments in groups of three; third item carries the weight"
    examples:
      - "الأول... والثاني... أما الثالث فهو..."
      - "..."
  - name: "rhetorical_question_pivot"
    confidence: 0.71
    description: "Opens a new section with a rhetorical question then answers it immediately"
    examples:
      - "فهل يعني هذا...؟ لا، بل..."
```

### 4. `voice_descriptor`

```yaml
voice_descriptor:
  formality: 8          # 1–10
  authority: 7
  warmth: 4
  ornamental: 6         # 1=sparse, 10=maximally decorative
  technical_depth: 6
  explicitness: 8       # 1=implicit/allusive, 10=explicit/didactic
  narrative_summary: "Authoritative and measured. Favors explicit argumentation..."
  conditioning_prompt: |
    Write in a measured, authoritative register. Prefer three-part enumerations.
    Open sections with a direct statement of position before elaborating.
    [Additional extracted voice instructions...]
```

### 5. `formatting_rules`

```yaml
formatting_rules:
  punctuation_style: oxford_comma   # or: no_serial_comma
  quotation_handling: guillemets    # guillemets | double | single | arabic
  # Arabic-only fields:
  arabic_punctuation: eastern       # eastern | western
  diacritical_policy:
    density_target: 0.42            # matches linguistic_profile.diacritical_density
    strategy: meaning-critical-first  # meaning-critical-first | uniform
```

## The Standalone Binary: naqb-style

All style operations are exposed through `naqb-style`, a standalone CLI binary.

| Command | Description |
|---|---|
| `extract <source>` | Extract a style image from a text file or directory |
| `apply <image> <target>` | Rewrite target text using style image |
| `blend <img1> <img2> [--weight 0.6]` | Blend two images by dimension weights |
| `fork <image> <new-name>` | Create a fork; tracks `forked_from` |
| `diff <img1> <img2>` | Show dimension-by-dimension delta |
| `cherry-pick <image> <dims...>` | Copy specific dimensions from one image to another |
| `fingerprint <text>` | Compute style signature without saving |
| `search <text>` | Find closest style image in `~/.naqb/styles/` by embedding distance |
| `validate <image>` | Check all five dimensions present and internally consistent |
| `show <image>` | Human-readable style image summary |
| `list` | List all stored style images |
| `tui` | Interactive style image browser |
| `transfer <image> <target>` | Style transfer: apply image to target, preserving content |

## Arabic-Specific Rules

When `language: ar` is set in the style image, these rules become mandatory:

- `diacritical_policy` must be fully specified (both `density_target` and `strategy`)
- `arabic_binyan_freq` must be present in `linguistic_profile`
- Balāgha patterns (`سجع`, `جناس`, `طباق`, `مقابلة`, `التفات`) must be extracted as
  named entries in `rhetorical_patterns`
- Qur'anic vocabulary (`prestige_lexicon`) must be flagged as a `StyleFeature` with
  `application: advisory` — it is never force-applied; the LLM is advised only
- `register` is a mandatory top-level field: `classical` | `MSA` | `colloquial`
  - This is a **hard boundary**, not a style dimension
  - A classical-register style image must never be applied to modern technical content
  - `validate` enforces this; `apply` refuses to run across a register boundary without `--force`

## Integration Rule

A style image is a **layer** (type: `style`) in a context stack. It is never a
top-level prompt override and never overrides system-level behavior.

The `style-apply` stack (from the standard library) provides the canonical integration
point. It places the style layer at position 5 (analytical), so structural and
interpretive layers run first.

In the braided field, a `style-apply` strand runs alongside content-analysis strands.
Its output is a proposed rewrite; it participates in braid point detection like any
other strand. CONFLICT braid points between a style strand and a factual-accuracy
strand are resolved by `ANNOTATE_BOTH` by default.

## Application Modes

| Mode | How it works | When to use |
|---|---|---|
| `prompt` | Inject `voice_descriptor.conditioning_prompt` + top-3 `rhetorical_pattern` examples into LLM system prompt | First-pass generation; blank-slate writing |
| `postprocess` | Surgical LLM rewrites of existing text toward target metric values | Revision of existing drafts |

`postprocess` mode sends the text in segments, targeting one or two metric dimensions
per segment to avoid over-constraining the model. It never attempts to hit all
dimensions in a single pass.

## Validation Before Apply (Mandatory)

Run `naqb-style validate <image>` before any `apply` or `transfer` call. Validation
checks and **warns** on:

- Classical Arabic style image applied to `domain: technical` content
- `diacritical_density > 0.5` applied to `domain: social-media` or similar
- `ornamental > 7` applied to `domain: legal` or `domain: academic`
- `register` mismatch between image and target text
- High `code_block_ratio` in target applied to `domain: philosophy` (likely wrong target)

Warnings are non-blocking unless the stage declares `on_style_warning: fail`.
