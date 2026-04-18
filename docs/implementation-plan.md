# Implementation Plan: Stability + Arabic + Pipeline Evolution

> **Status:** Draft — awaiting approval before implementation begins  
> **Estimated effort:** 2-3 days (Sprint 0: 2h, Sprint 2: 1 day, Sprint 3: 1-2 days)  
> **Risk:** Medium — touches core pipeline, export, and chat paths

---

## Sprint 0: Production Bug Fixes (2 hours)

**Goal:** Close 7 bugs that will bite users in production. All are surgical, one-line or few-line fixes.

### S0.1 — Fix Arabic UTF-8 truncation in `nqb chat`

**File:** `internal/commands/chat.go:93-96`

**Problem:** `len(content)` counts bytes, not runes. Slicing at byte 8000 cuts Arabic multi-byte characters in half, producing invalid UTF-8.

**Change:**
```go
// Before:
if len(content) > 8000 {
    content = content[:8000] + "\n... (truncated)"
}

// After:
runes := []rune(content)
if len(runes) > 4000 {
    content = string(runes[:4000]) + "\n... (truncated)"
}
```

**Test:** Add `TestBuildChatSystemPrompt_ArabicTruncation` in `internal/commands/commands_test.go`:
- Input: Arabic text > 4000 runes
- Verify: output is valid UTF-8, ends with "... (truncated)"

---

### S0.2 — Remove hardcoded Composio test user ID

**Files:**
- `internal/commands/sync.go:59-61`
- `internal/gdocs/client.go:19` (remove `defaultUserID` constant)

**Change:** Remove the fallback. If no user ID configured, return clear error:
```go
if userID == "" {
    return fmt.Errorf("composio user ID not configured — run 'nqb setup' or set sync.composio_user_id in book.yaml")
}
```

**Test:** Add `TestSyncCommand_MissingUserID` in `commands_test.go`.

---

### S0.3 — Remove `--reindex` no-op flag

**File:** `internal/commands/index.go`

**Decision:** Option B — remove the flag entirely. Re-indexing is fast; skip logic adds complexity for no benefit.

**Change:**
- Remove `--reindex` flag definition from Cobra command
- Remove any references to the flag in the command body
- Update help text if needed

**Test:** None needed (removing code).

---

### S0.4 — Return default model from `NewProviderFromGlobalConfig`

**File:** `pkg/agent/provider.go:68`

**Change:**
```go
// Before:
return provider, "", nil

// After:
model := agents.ModelFor(agents.StageChat, nil)
return provider, model, nil
```

**Test:** Update `TestNewProviderFromGlobalConfig` to assert non-empty model.

---

### S0.5 — Fix `readPrompt` wrong directory

**File:** `pkg/pipeline/reflection.go` (and any other file using `readPrompt`)

**Change:** Check both standard and legacy paths:
```go
func readPrompt(bookDir, name string) (string, error) {
    // Standard location (created by InitBookDir)
    path := filepath.Join(bookDir, "config", "prompts", name)
    if data, err := os.ReadFile(path); err == nil {
        return string(data), nil
    }
    // Legacy location
    path = filepath.Join(bookDir, "prompts", name)
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

**Test:** Add `TestReadPrompt_StandardLocation` and `TestReadPrompt_LegacyFallback` in `reflection_test.go`.

---

### S0.6 — Fix `fix.go` indentation artifact

**File:** `internal/commands/fix.go:64`

**Change:** Remove extra leading tabs. One-line change.

**Test:** None (cosmetic).

---

### S0.7 — Fix `write.go` misleading description

**File:** `internal/commands/write.go:27`

**Change:**
```go
// Before:
Short: "Write a chapter using Claude Sonnet (with spinner)"

// After:
Short: "Write a chapter draft using the configured LLM"
```

---

### S0.8 — Sanitize `import` file paths (gosec G703)

**Files:** `internal/commands/import.go:155-276`

**Problem:** User-supplied `src` paths flow directly into `os.ReadFile`, `os.WriteFile`, and `filepath.Join` without sanitization. A malicious `src` like `../../../etc/passwd` could read/write outside the book directory.

**Fix:** Validate that `src` resolves inside the book directory or user's home:
```go
absSrc, err := filepath.Abs(src)
if err != nil { return err }
absBook, _ := filepath.Abs(bookDir)
if !strings.HasPrefix(absSrc, absBook) && !strings.HasPrefix(absSrc, os.Getenv("HOME")) {
    return fmt.Errorf("import path must be inside book directory or home folder")
}
```

---

### S0.9 — Validate `envFile` path in `keycheck` (gosec G703)

**File:** `internal/keycheck/keycheck.go:165`

**Problem:** `envFile` is constructed from `bookDir` but `os.WriteFile` could write outside the intended scope if `bookDir` is malformed.

**Fix:** Ensure `envFile` is resolved and checked to be within the book directory before writing.

---

### S0.10 — Annotate acceptable `exec.Command` calls (gosec G204/G702)

**Files:**
- `internal/commands/config.go:119` — opens user-configured editor (standard CLI behavior)
- `internal/exporter/exporter.go:68` — runs `pandoc` with application-generated file list
- `internal/commands/status.go:113,122` — runs `git` in the book directory
- `internal/tui/sidebar.go:266` — runs `git` in the book directory
- `internal/keycheck/keycheck.go:175,196` — runs `security` with current `$USER`

**Problem:** gosec flags any `exec.Command` with dynamic arguments as high-risk.

**Fix:** Add `//nolint:gosec // <justification>` to each call after confirming the risk is acceptable:
- Editor: user-configured tool, standard pattern
- Pandoc: args are generated by the app, not raw user input
- Git: `bookDir` is validated on init
- Keychain: `USER` env var and `service` string are controlled values

---

### Sprint 0 Checklist

- [ ] S0.1 UTF-8 truncation fix + test
- [ ] S0.2 Remove hardcoded Composio user ID
- [ ] S0.3 Remove `--reindex` flag
- [ ] S0.4 Return default model from `NewProviderFromGlobalConfig`
- [ ] S0.5 Fix prompt directory lookup
- [ ] S0.6 Fix indentation
- [ ] S0.7 Fix misleading description
- [ ] S0.8 Sanitize `import` file paths (gosec G703)
- [ ] S0.9 Validate `envFile` path in `keycheck` (gosec G703)
- [ ] S0.10 Annotate acceptable `exec.Command` calls (gosec G204/G702)
- [ ] `make check` passes across all modules
- [ ] Commit: `fix: close 7 production bugs + gosec annotations (Sprint 0)`

---

## Sprint 2: Arabic RTL Export + Chapter Provenance (1 day)

**Goal:** Make the app production-ready for Arabic book authors. Add RTL export support and per-chapter provenance tracking.

---

### S2.1 — Arabic-aware export pipeline

**Files:**
- `internal/exporter/pdf.go`
- `internal/exporter/epub.go`
- `internal/exporter/docx.go`
- `internal/exporter/web.go`
- **NEW:** `internal/exporter/arabic.go` (shared RTL helpers)
- **NEW:** `internal/exporter/templates/arabic.tex` (XeLaTeX template)

**Changes per format:**

**PDF (`pdf.go`):**
```go
func arabicPDFArgs() []string {
    return []string{
        "--pdf-engine=xelatex",
        "--variable", "dir=rtl",
        "--variable", "mainfont=Amiri",
        "--variable", "mainfontoptions=Script=Arabic",
        "--variable", "geometry:margin=2.5cm",
        "--template", templatePath("arabic.tex"),
    }
}
```
Call in `Export()` when `cfg.Language == "ar" || cfg.Language == "ara"`.

**EPUB (`epub.go`):**
- Add `--variable dir=rtl`
- Set `page-progression-direction: rtl` in metadata

**DOCX (`docx.go`):**
- Add `--reference-doc` pointing to Arabic-configured template

**Web (`web.go`):**
- Add `dir="rtl"` and `lang="ar"` to HTML wrapper
- Include Amiri font from Google Fonts

**XeLaTeX template (`arabic.tex`):**
```latex
\documentclass[12pt,a4paper]{book}
\usepackage{polyglossia}
\setmainlanguage{arabic}
\setmainfont{Amiri}
\newfontfamily\arabicfont[Script=Arabic]{Amiri}
```

**Test:** Add `internal/exporter/exporter_test.go`:
- `TestCommonArgs_Language` — verifies language flag
- `TestArabicPDFArgs` — verifies xelatex, RTL, Amiri
- `TestExport_UnknownFormat` — error for invalid format
- Skip tests if pandoc binary not found (`exec.LookPath`)

---

### S2.2 — Provenance sidecar generation

**NEW file:** `pkg/agents/provenance.go`

**Data model:**
```go
type Provenance struct {
    ChapterNum       int            `yaml:"chapter"`
    GeneratedAt      time.Time      `yaml:"generated_at"`
    Model            string         `yaml:"model"`
    TokensIn         int64          `yaml:"tokens_in"`
    TokensOut        int64          `yaml:"tokens_out"`
    ContextSources   []SourceRecord `yaml:"context_sources"`
    ResearchNotes    []string       `yaml:"research_notes"`
    ClaimsReferenced []string       `yaml:"claims_referenced"`
    EpistemicDebt    []string       `yaml:"epistemic_debt"`
    WordCountBefore  int            `yaml:"word_count_before"`
    WordCountAfter   int            `yaml:"word_count_after"`
    QAResult         *QAProvenance  `yaml:"qa_result,omitempty"`
}

type SourceRecord struct {
    Path   string `yaml:"path"`
    Type   string `yaml:"type"`   // chapter, research, outline, rules
    Status string `yaml:"status"` // INCLUDED, EXCLUDED, DEGRADED, MISSING
}

type QAProvenance struct {
    Passed         bool     `yaml:"passed"`
    Issues         []string `yaml:"issues,omitempty"`
    DeterministicOK bool    `yaml:"deterministic_ok"`
}
```

**Integration points:**
1. `agents/context_builder.go` — `WriteContextFile()` returns `[]SourceRecord` tracking included/excluded sources
2. `agents/writer.go` — `WriteChapter()` captures token usage, populates Provenance
3. `agents/qa.go` — `RunQA()` appends QA results to Provenance
4. **NEW:** `WriteProvenance(bookDir, prov)` writes `chapters/ch-XX.provenance.md`

**Output format:** YAML frontmatter + narrative markdown.

**Test:** Add `pkg/agents/provenance_test.go`:
- `TestProvenance_RoundTrip` — create → write → read → verify fields
- `TestProvenance_Marshal` — verify YAML structure

---

### Sprint 2 Checklist

- [ ] S2.1 Arabic export: PDF/EPUB/DOCX/Web RTL support
- [ ] S2.1 XeLaTeX Arabic template
- [ ] S2.1 Exporter tests (skip if pandoc absent)
- [ ] S2.2 Provenance struct + writer
- [ ] S2.2 Wire into context_builder, writer, qa
- [ ] S2.2 Provenance round-trip test
- [ ] `make check` passes
- [ ] Commit: `feat: Arabic RTL export + chapter provenance tracking`

---

## Sprint 3: Iterative Quality Loops + Integration (1-2 days)

**Goal:** Make the pipeline robust. Add iterative QA→fix loops, wire style engine and context stacks, fix parallel state merge.

---

### S3.1 — Iterative quality loops

**Files:**
- `pkg/pipeline/pipeline.go` (add loop logic)
- `pkg/agents/writer.go` (support revision mode)
- `pkg/agents/fixer.go` (support FixModeQA)
- `pkg/agents/qa.go` (return structured pass/fail with score)

**Design:**
```
Context → Write → QA → pass? → END
              ↓ fail
            Revise → QA → pass? → END
              ↓ fail (iter < max)
            Revise → QA → END (keep best)
```

**Config:**
```go
type QualityLoopConfig struct {
    MaxIterations int     // default: 3
    QAThreshold   float64 // 0.0-1.0, default: 0.8
    AutoFix       bool    // default: true
}

type QualityLoopResult struct {
    Attempts []LoopAttempt
    Best     int // index of best attempt
}

type LoopAttempt struct {
    Iteration int
    Draft     string
    QAResult  *agents.QAResult
    Score     float64
}
```

**Function:**
```go
func RunQualityLoop(ctx context.Context, client llm.Provider,
    bookDir string, cfg *config.BookConfig, chNum int,
    loopCfg QualityLoopConfig) (*QualityLoopResult, error)
```

**CLI integration:**
- `nqb pipeline --chapter 3 --quality-loop`
- `nqb pipeline --chapter 3 --max-iterations 5`
- Default pipeline stays single-pass for backwards compat

**Test:** Add `pkg/pipeline/quality_loop_test.go`:
- `TestRunQualityLoop_PassesFirstTry` — QA passes on first draft
- `TestRunQualityLoop_RevisesAndPasses` — QA fails once, revision passes
- `TestRunQualityLoop_MaxIterations` — exhausts max iterations, returns best

---

### S3.2 — Wire style engine into write pipeline

**Files:**
- `pkg/agents/writer.go` (inject style constraints)
- `internal/style/apply.go` (already implemented)

**Design:**
1. Before calling LLM writer, check for active style:
   ```go
   stylePath := filepath.Join(bookDir, ".naqb", "styles", "active.yaml")
   if img, err := style.Load(stylePath); err == nil {
       systemPrompt += style.PromptConstraints(img)
   }
   ```
2. Style constraints appended as guidelines (sentence length, register, etc.)
3. No postprocess mode during initial write — too expensive
4. `nqb fix --style` uses postprocess mode for rewrites

**Test:** Add `pkg/agents/writer_test.go`:
- `TestWriteChapter_WithStyle` — verify style constraints appear in prompt

---

### S3.3 — Wire context stacks into context builder

**Files:**
- `pkg/agents/context_builder.go` (inject context layers)
- `internal/context/stack.go` (already implemented)
- `internal/context/arabic/layers.go` (11 Arabic layers)

**Design:**
For Arabic books (`cfg.Language == "ar"`):
1. Load 11 Arabic analytical layers from `arabic/layers.go`
2. Evaluate relevance to chapter topic
3. Include relevant layer prompts in context file

For non-Arabic: use simpler default stack.

**Integration point:**
```go
if cfg.Language == "ar" {
    layers := arabic.DefaultLayers()
    relevant := filterByChapterTopic(layers, chapterConfig)
    contextFile += "\n\n## Analytical Framework\n"
    for _, layer := range relevant {
        contextFile += fmt.Sprintf("### %s\n%s\n\n", layer.Name, layer.Prompt)
    }
}
```

**Test:** Add `pkg/agents/context_builder_test.go`:
- `TestBuildContext_ArabicLayers` — verify Arabic layers included when language == "ar"

---

### S3.4 — Fix parallel state merge in runtime

**File:** `pkg/runtime/graph.go:211-216`

**Problem:** `runBatch` uses last-writer-wins, silently drops concurrent node outputs.

**Design:** Introduce `Mergeable` interface:
```go
type Mergeable[S any] interface {
    Merge(other S) S
}
```

For `PipelineState`, implement `Merge` to combine maps/slices:
```go
func (s PipelineState) Merge(other PipelineState) PipelineState {
    merged := s
    merged.Stages = append(merged.Stages, other.Stages...)
    for k, v := range other.Completed {
        merged.Completed[k] = v
    }
    return merged
}
```

In `runBatch`, if state implements `Mergeable`, use it. Otherwise fall back to last-writer-wins with a log warning.

**Test:** Add to `pkg/runtime/graph_test.go`:
- `TestInvokeParallel_MergesState` — verify all parallel node outputs preserved

---

### Sprint 3 Checklist

- [ ] S3.1 Quality loop implementation + `RunQualityLoop()`
- [ ] S3.1 CLI flags `--quality-loop`, `--max-iterations`
- [ ] S3.1 Quality loop tests
- [ ] S3.2 Style engine wired into writer
- [ ] S3.2 Writer style test
- [ ] S3.3 Context stacks wired into context builder
- [ ] S3.3 Arabic layer context test
- [ ] S3.4 Parallel state Mergeable interface
- [ ] S3.4 PipelineState.Merge implementation
- [ ] S3.4 Parallel merge test
- [ ] `make check` passes
- [ ] Commit: `feat: iterative quality loops + style/context integration + parallel merge`

---

## Cross-Cutting Concerns

### Makefile Update (S1.1 — ✅ Done)

`make check`, `make test`, `make test-v`, `make test-race`, `make vet`, `make lint`, and `make cover` now iterate all `pkg/*` workspace modules. `make tidy` uses `go work sync`.

See `.github/workflows/ci.yml` for the CI equivalent.

### Migration (S1.2 — optional, can defer)

Migration `006_fix_cascade.sql` recreates `claim_relations` and `concept_claims` with `ON DELETE CASCADE`. Since SQLite doesn't support `ALTER TABLE` for FK changes, the migration recreates both tables.

**Can defer** if claim deletion is not a primary user path yet.

---

## Approval Gate

Before implementation begins, confirm:

1. **Sprint 0 scope:** All 7 bugs approved for fix?
2. **Sprint 2 scope:** Arabic RTL export + provenance tracking approved?
3. **Sprint 3 scope:** Iterative quality loops + style/context integration + parallel merge approved?
4. **Order:** Sprint 0 → Sprint 2 → Sprint 3 (sequential, or parallel where possible)?
5. **Deferred items:** S1.2 (CASCADE migration), S1.3 (ContextDebt wiring), S1.4 (RESEARCH/SYNTHESIZE stages) — defer to later?

Reply with approval and any changes, or ask questions about specific items.
