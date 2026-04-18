# naqb Gap Closure Plan

> Created: 2026-04-18
> Source: `docs/inspiration-feynman.md` (43 gaps identified)
> Goal: Close all gaps across 6 sprints, prioritized by production risk

---

## Sprint 0: Emergency Fixes (1-2 hours)

Bugs that will bite users in production. All are small, surgical fixes.

### S0.1 — Fix Arabic UTF-8 truncation in `nqb chat` [D13]

**File:** `internal/commands/chat.go:93-96`

**Problem:** `len(content)` counts bytes, not runes. Slicing at byte 8000
cuts Arabic multi-byte characters in half, producing invalid UTF-8.

**Fix:**
```go
// Before (broken):
if len(content) > 8000 {
    content = content[:8000] + "\n... (truncated)"
}

// After (safe):
runes := []rune(content)
if len(runes) > 4000 {
    content = string(runes[:4000]) + "\n... (truncated)"
}
```

Use 4000 runes (not 8000) because Arabic runes are 2-4 bytes each.

**Test:** Add `TestBuildChatSystem_ArabicTruncation` in `commands_test.go`
with Arabic text longer than 4000 chars, verify output is valid UTF-8.

---

### S0.2 — Remove hardcoded Composio test user ID [D15]

**Files:**
- `internal/commands/sync.go:59-61`
- `internal/gdocs/client.go:19` (same ID as `defaultUserID`)

**Fix:** Remove the fallback. If no user ID configured, error clearly:
```go
if userID == "" {
    return fmt.Errorf("composio user ID not configured — run 'nqb setup' or set sync.composio_user_id in book.yaml")
}
```

Also remove `defaultUserID` constant from `gdocs/client.go`.

---

### S0.3 — Fix `--reindex` no-op [D4]

**File:** `internal/commands/index.go`

**Problem:** The `--reindex` flag is defined but never passed through.

**Fix options (choose one):**
- **Option A:** Pass `reindex` bool to `store.IndexChapter()` and
  `store.IndexFile()`, skip if document already indexed (check by
  ContentSignature) unless `reindex` is true.
- **Option B:** Remove the `--reindex` flag entirely. Always re-index
  (current behavior). Simpler, less confusing.

Recommend **Option B** — remove the flag and the misleading message.
Re-indexing is fast enough that skip logic adds complexity for no benefit.

---

### S0.4 — Fix `NewProviderFromGlobalConfig` empty model [D5]

**File:** `pkg/agent/provider.go:68`

**Fix:** Return a sensible default model:
```go
model := agents.ModelFor(agents.StageChat, nil)
return provider, model, nil
```

This makes every caller get the correct default instead of relying on
each call site to resolve the model separately.

---

### S0.5 — Fix `readPrompt` wrong directory [D14]

**File:** `pkg/pipeline/reflection.go:216-222`

**Fix:** Check both paths, prefer the standard location:
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

---

### S0.6 — Fix `fix.go` indentation artifact [D24]

**File:** `internal/commands/fix.go:64`

**Fix:** Remove extra leading tabs. One-line change.

---

### S0.7 — Fix `write.go` misleading short description [D25]

**File:** `internal/commands/write.go:27`

**Fix:** Change `"Write a chapter using Claude Sonnet (with spinner)"` to
`"Write a chapter draft using the configured LLM"`.

---

### S0 Checklist

- [ ] S0.1 UTF-8 truncation fix + test
- [ ] S0.2 Remove hardcoded Composio user ID
- [ ] S0.3 Remove `--reindex` flag (Option B)
- [ ] S0.4 Return default model from `NewProviderFromGlobalConfig`
- [ ] S0.5 Fix prompt directory lookup
- [ ] S0.6 Fix indentation
- [ ] S0.7 Fix misleading description
- [ ] `make check` passes
- [ ] Commit: `fix: close 7 production bugs (UTF-8, hardcoded IDs, no-op flags)`

---

## Sprint 1: Critical Wiring + Build Integrity (Half day)

### S1.1 — Fix `make check` to cover all `pkg/` modules [GAP 15] ✅ DONE

**File:** `Makefile`

**What changed:**
- `make check` — now builds, vets, and tests root + all 11 `pkg/*` modules
- `make test` / `make test-v` / `make test-race` — iterate all modules
- `make vet` / `make lint` / `make cover` — iterate all modules
- `make tidy` — uses `go work sync` instead of per-module `go mod tidy`

**Also delivered:** Full GitHub Actions CI/CD pipeline:
- `.github/workflows/ci.yml` — test matrix (Go 1.26 + stable), race detector, shuffle, `go work sync` check, build, golangci-lint
- `.github/workflows/security.yml` — govulncheck, CodeQL, gosec (non-blocking)
- `.github/workflows/release.yml` — GoReleaser on `v*` tags
- `.goreleaser.yml` — multi-binary releases for `nqb`, `naqb-style`, `nqb-mcp`
- `.github/dependabot.yml` — weekly updates for root + all `pkg/*` modules + actions
- `.golangci.yml` — linter config (errcheck/gocritic disabled for legacy code, to be re-enabled incrementally)

---

### S1.2 — Add ON DELETE CASCADE to migration 003 [D16]

**File:** `internal/db/migrations/003_knowledge.sql`

**Problem:** FK constraints without CASCADE cause claim deletion to fail.

**Fix:** Create migration `006_fix_cascade.sql`:
```sql
-- +goose Up
-- Fix missing CASCADE on claim_relations and concept_claims FKs.
-- SQLite does not support ALTER TABLE to modify FK constraints,
-- so we recreate the tables.

CREATE TABLE claim_relations_new (
    id         TEXT PRIMARY KEY,
    source_id  TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    target_id  TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    rel_type   TEXT NOT NULL,
    weight     REAL DEFAULT 1.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO claim_relations_new SELECT * FROM claim_relations;
DROP TABLE claim_relations;
ALTER TABLE claim_relations_new RENAME TO claim_relations;

CREATE TABLE concept_claims_new (
    concept_id TEXT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    claim_id   TEXT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    PRIMARY KEY (concept_id, claim_id)
);
INSERT INTO concept_claims_new SELECT * FROM concept_claims;
DROP TABLE concept_claims;
ALTER TABLE concept_claims_new RENAME TO concept_claims;

-- +goose Down
-- Reverting to non-CASCADE is not worth the complexity.
-- The CASCADE version is strictly better.
```

---

### S1.3 — Wire ContextDebt into pipeline execution [D2]

**Files:**
- `pkg/pipeline/debt.go` (existing, unused)
- `pkg/pipeline/executor.go` (needs ContextDebt injection)

**Problem:** ContextDebt is fully implemented but never instantiated.

**Fix:**
1. Add `Debt *ContextDebt` field to the pipeline state/config
2. In each stage execution, call `debt.Record(event)` for FAIL/DEGRADE
3. After DAG completion, check `debt.HasViolations()` and include
   `debt.Summary()` in the pipeline report
4. If HUMAN_GATE violations exist, block and write gate_blocked job

---

### S1.4 — Register RESEARCH and SYNTHESIZE stage types [D3]

**File:** `pkg/pipeline/dag.go`

**Problem:** Constants exist but no implementation registered.

**Fix options:**
- **Option A:** Implement the stage handlers (uses research pipeline)
- **Option B:** Remove the constants if not planned for this phase

Recommend **Option A** — wire `StageTypeResearch` to call
`research.Run()` and `StageTypeSynthesize` to call a new synthesis
function that merges research notes into coherent context.

---

### S1.5 — Fix `loadOutlineSection` to return chapter-specific section [D6]

**File:** `pkg/agent/context.go:104-111`

**Fix:** Parse the outline markdown, find the section for the target
chapter by heading pattern (`## Chapter N` or `## Ch N` or numbered list),
and return only that section plus adjacent context:
```go
func loadOutlineSection(bookDir string, chapterNum int) string {
    data, err := os.ReadFile(filepath.Join(bookDir, "outline.md"))
    if err != nil {
        return ""
    }
    full := string(data)
    section := extractChapterSection(full, chapterNum)
    if section == "" {
        return full // fallback to full outline
    }
    return section
}
```

---

### S1.6 — Add `internal/changelog` to MEMORY.md [D28]

**File:** `memory/MEMORY.md`

Add line to architecture map:
```
  changelog/               ← Generate() → git log → categorized report; FormatMarkdown()
```

---

### S1 Checklist

- [x] S1.1 `make check` covers `pkg/` modules + GitHub Actions CI/CD pipeline
- [ ] S1.2 Migration 006 for CASCADE fix
- [ ] S1.3 Wire ContextDebt into executor
- [ ] S1.4 Register RESEARCH/SYNTHESIZE stages
- [ ] S1.5 Fix `loadOutlineSection`
- [ ] S1.6 Update MEMORY.md
- [ ] All tests pass across all modules
- [ ] Commit: `fix: build integrity + pipeline wiring (Sprint 1)`

---

## Sprint 2: Arabic Export + Provenance (1 day)

### S2.1 — Arabic-aware export pipeline [GAP 10]

**Files:**
- `internal/exporter/pdf.go`
- `internal/exporter/epub.go`
- `internal/exporter/docx.go`
- `internal/exporter/web.go`
- NEW: `internal/exporter/arabic.go` (shared RTL helpers)
- NEW: `internal/exporter/templates/arabic.tex` (XeLaTeX template)

**Changes per format:**

**PDF (pdf.go):**
```go
func (e PDFExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
    args := commonArgs(cfg)
    if cfg.Language == "ar" || cfg.Language == "ara" {
        args = append(args, arabicPDFArgs()...)
    }
    // ...
}

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

**EPUB (epub.go):**
- Add `--variable dir=rtl`
- Set `page-progression-direction: rtl` in epub metadata

**DOCX (docx.go):**
- Add `--reference-doc` pointing to an Arabic-configured template

**Web (web.go):**
- Add `dir="rtl"` and `lang="ar"` to HTML wrapper
- Include Arabic web font (Amiri from Google Fonts)

**XeLaTeX template (`arabic.tex`):**
```latex
\documentclass[12pt,a4paper]{book}
\usepackage{polyglossia}
\setmainlanguage{arabic}
\setmainfont{Amiri}
\newfontfamily\arabicfont[Script=Arabic]{Amiri}
% ... standard scholarly book template
```

**Test:** Add `internal/exporter/exporter_test.go` with tests that verify
Arabic-specific args are applied when `cfg.Language == "ar"`.

---

### S2.2 — Provenance sidecar generation [GAP 3]

**NEW file:** `pkg/agents/provenance.go`

```go
// Provenance records the source lineage of a generated chapter.
type Provenance struct {
    ChapterNum      int               `yaml:"chapter"`
    GeneratedAt     time.Time         `yaml:"generated_at"`
    Model           string            `yaml:"model"`
    TokensIn        int64             `yaml:"tokens_in"`
    TokensOut       int64             `yaml:"tokens_out"`
    ContextSources  []SourceRecord    `yaml:"context_sources"`
    ResearchNotes   []string          `yaml:"research_notes"`
    ClaimsReferenced []string         `yaml:"claims_referenced"`
    EpistemicDebt   []string          `yaml:"epistemic_debt"`
    WordCountBefore int               `yaml:"word_count_before"`
    WordCountAfter  int               `yaml:"word_count_after"`
    QAResult        *QAProvenance     `yaml:"qa_result,omitempty"`
}

type SourceRecord struct {
    Path   string `yaml:"path"`
    Type   string `yaml:"type"` // chapter, research, outline, rules
    Status string `yaml:"status"` // INCLUDED, EXCLUDED, DEGRADED, MISSING
}

type QAProvenance struct {
    Passed        bool     `yaml:"passed"`
    Issues        []string `yaml:"issues,omitempty"`
    DeterministicOK bool   `yaml:"deterministic_ok"`
}
```

**Integration points:**

1. `agents/context_builder.go` — `WriteContextFile()` returns a
   `[]SourceRecord` tracking what sources were included/excluded
2. `agents/writer.go` — `WriteChapter()` captures token usage and
   populates the Provenance struct
3. `agents/qa.go` — `RunQA()` appends QA results to Provenance
4. After all stages: `WriteProvenance(bookDir, prov)` writes
   `chapters/ch-XX.provenance.md` with YAML frontmatter + narrative

**Output format (`chapters/ch-03.provenance.md`):**
```markdown
---
chapter: 3
generated_at: 2026-04-18T14:30:00Z
model: anthropic/claude-sonnet-4-6
tokens_in: 45230
tokens_out: 12800
word_count_before: 0
word_count_after: 3200
qa_passed: true
---

# Provenance: Chapter 3 — عنوان الفصل

## Sources Consulted
- INCLUDED  outline.md (chapter 3 section)
- INCLUDED  chapters/ch-02.md (prior chapter for continuity)
- INCLUDED  .naqb/research/ch-03-note-01.md
- INCLUDED  .naqb/research/ch-03-note-02.md
- DEGRADED  .naqb/research/ch-03-note-03.md (file too large, truncated)
- MISSING   .naqb/research/ch-03-note-04.md (file not found)

## Research Notes Used
- ch-03-note-01.md: Primary sources on topic X
- ch-03-note-02.md: Scholarly debate overview

## Epistemic Debt
- (none)

## QA Result
- Deterministic checks: PASSED
- LLM audit: PASSED
```

**Test:** Add `pkg/agents/provenance_test.go` with round-trip test
(create Provenance → write file → read back → verify fields).

---

### S2 Checklist

- [ ] S2.1 Arabic export: PDF/EPUB/DOCX/Web RTL support
- [ ] S2.1 XeLaTeX Arabic template
- [ ] S2.1 Exporter tests
- [ ] S2.2 Provenance struct + writer
- [ ] S2.2 Wire into context_builder, writer, qa
- [ ] S2.2 Provenance tests
- [ ] `make check` passes
- [ ] Commit: `feat: Arabic RTL export + chapter provenance tracking`

---

## Sprint 3: Pipeline Evolution (1-2 days)

### S3.1 — Iterative quality loops [GAP 4]

**Files:**
- `pkg/pipeline/pipeline.go` (add loop logic)
- `pkg/agents/writer.go` (support revision mode)
- `pkg/agents/qa.go` (return structured pass/fail)

**Design:**

```
┌──────────────────────────────────────────┐
│            Chapter Pipeline              │
│                                          │
│  ┌─────────┐    ┌───────┐    ┌───────┐  │
│  │ Context  │───▶│ Write │───▶│  QA   │  │
│  └─────────┘    └───────┘    └───┬───┘  │
│                                  │       │
│                    ┌─────────────┤       │
│                    │ pass?       │       │
│                    ▼             ▼       │
│                  ┌───┐       ┌──────┐   │
│                  │END│       │Revise│   │
│                  └───┘       └──┬───┘   │
│                                 │       │
│                    ┌────────────┘       │
│                    ▼                    │
│                 ┌──────┐               │
│                 │ QA 2 │─── pass? ───▶ END
│                 └──┬───┘               │
│                    │ fail (iter < max) │
│                    └───────▶ Revise    │
│                                        │
│  Max iterations: 3 (configurable)      │
│  Provenance records each attempt       │
└──────────────────────────────────────────┘
```

**Implementation:**

```go
type QualityLoopConfig struct {
    MaxIterations int     // default: 3
    QAThreshold   float64 // 0.0-1.0, default: 0.8
    AutoFix       bool    // default: true
}

func RunQualityLoop(ctx context.Context, client llm.Provider,
    bookDir string, cfg *config.BookConfig, chNum int,
    loopCfg QualityLoopConfig) (*QualityLoopResult, error) {

    var attempts []LoopAttempt

    for i := 0; i < loopCfg.MaxIterations; i++ {
        if i == 0 {
            // Fresh write
            _, err := WriteChapter(ctx, client, bookDir, cfg, chNum, nil)
        } else {
            // Revision based on QA issues
            _, err := FixChapter(ctx, client, bookDir, cfg, chNum, FixModeQA)
        }

        qaResult, _ := RunQA(ctx, client, bookDir, cfg, chNum)
        attempts = append(attempts, LoopAttempt{
            Iteration: i + 1,
            QAResult:  qaResult,
        })

        if qaResult.Passed || qaResult.Score >= loopCfg.QAThreshold {
            break
        }
    }

    return &QualityLoopResult{Attempts: attempts}, nil
}
```

**CLI integration:**
- `nqb pipeline --chapter 3 --quality-loop` enables iterative mode
- `nqb pipeline --chapter 3 --max-iterations 5` sets max
- Default pipeline (no flag) stays single-pass for backwards compat

---

### S3.2 — Wire style engine into write pipeline [GAP 12]

**Files:**
- `pkg/agents/writer.go` (inject style constraints)
- `internal/style/apply.go` (already implemented)

**Design:**

1. Before calling the LLM writer, check if a style image exists:
   ```go
   stylePath := filepath.Join(bookDir, ".naqb", "styles", "active.yaml")
   if img, err := style.Load(stylePath); err == nil {
       systemPrompt += style.PromptConstraints(img)
   }
   ```
2. The style constraints are appended to the system prompt as
   guidelines (sentence length targets, vocabulary register, etc.)
3. No postprocess mode during initial write — too expensive
4. `nqb fix --style` continues to use postprocess mode for rewrites

**Test:** Verify that when a style image exists, the system prompt
includes style constraint text.

---

### S3.3 — Wire context stacks into context builder [GAP 11]

**Files:**
- `pkg/agents/context_builder.go` (inject context layers)
- `internal/context/stack.go` (already implemented)
- `internal/context/arabic/layers.go` (11 Arabic layers)

**Design:**

For Arabic books (`cfg.Language == "ar"`), the context builder
automatically applies the Arabic context stack:
1. Load the 11 Arabic analytical layers from `arabic/layers.go`
2. Evaluate which layers are relevant to this chapter's topic
3. Include relevant layer prompts in the context file

For non-Arabic books, use a simpler default stack.

**Integration point:** In `WriteContextFile()`, after assembling the
base context, append context stack layers:
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

---

### S3.4 — Fix parallel state merge in runtime [D1]

**File:** `pkg/runtime/graph.go:211-216`

**Problem:** Last-writer-wins silently drops data.

**Fix:** Require State to implement a `Merge` interface:
```go
type Mergeable[S any] interface {
    Merge(other S) S
}
```

For pipeline state (which is likely a struct with map fields), implement
Merge to combine maps/slices rather than overwrite:
```go
func (s PipelineState) Merge(other PipelineState) PipelineState {
    merged := s
    for k, v := range other.Outputs {
        merged.Outputs[k] = v
    }
    merged.Errors = append(merged.Errors, other.Errors...)
    return merged
}
```

If State doesn't implement Mergeable, fall back to last-writer-wins
with a log warning.

**Test:** Add `TestInvokeParallel_MergesState` verifying all parallel
node outputs are preserved.

---

### S3 Checklist

- [ ] S3.1 Quality loop implementation + CLI flags
- [ ] S3.2 Style engine wired into writer
- [ ] S3.3 Context stacks wired into context builder
- [ ] S3.4 Parallel state merge fix
- [ ] Tests for all
- [ ] Commit: `feat: iterative quality loops + style/context integration`

---

## Sprint 4: Test Debt + Provider Fixes (1 day)

### S4.1 — Tests for untested packages [GAP 2]

**Priority order** (by risk):

#### `pkg/research/` — Scout/Explorer/Scribe

NEW: `pkg/research/pipeline_test.go`

```go
func TestRun_MockProvider(t *testing.T) {
    // Mock LLM that returns canned scout queries
    // Mock HTTP client that returns canned search results
    // Verify: notes written, correct file count, no panics
}

func TestScout_GeneratesQueries(t *testing.T) { ... }
func TestExplorer_ParsesResults(t *testing.T) { ... }
func TestScribe_SynthesizesNotes(t *testing.T) { ... }
```

#### `internal/exporter/` — Pandoc wrappers

NEW: `internal/exporter/exporter_test.go`

```go
func TestCollectChapterFiles(t *testing.T) {
    // Create temp dir with ch-01.md, ch-02.md
    // Verify correct file list
}

func TestCommonArgs(t *testing.T) {
    cfg := &config.BookConfig{Title: "Test", Author: "A", Language: "ar"}
    args := commonArgs(cfg)
    // Verify title, author, language in args
}

func TestArabicPDFArgs(t *testing.T) {
    // Verify xelatex engine, RTL direction, Amiri font
}

func TestExport_UnknownFormat(t *testing.T) {
    _, err := Export("invalid", "/tmp", nil)
    // Verify error message
}
```

Note: Skip tests that require pandoc binary (check `exec.LookPath` first).

#### `internal/mcpserver/` — MCP tool handlers

NEW: `internal/mcpserver/server_test.go`

```go
func TestStatusHandler_ValidBook(t *testing.T) {
    // Create temp book dir with book.yaml + 2 chapters
    // Call statusHandler with mock request
    // Verify output contains chapter list
}

func TestStatusHandler_MissingBookDir(t *testing.T) {
    // Empty book_dir → error
}

func TestExportHandler_UnknownFormat(t *testing.T) { ... }
```

#### `internal/gdocs/` — Composio HTTP client

NEW: `internal/gdocs/client_test.go`

```go
func TestCreateDocument_ParsesResponse(t *testing.T) {
    // httptest server returning mock Composio response
    // Verify DocInfo fields
}

func TestBuildDocContent(t *testing.T) {
    chapters := []ChapterContent{
        {Number: 1, Title: "Introduction", Body: "..."},
    }
    content := BuildDocContent(chapters)
    // Verify format
}

func TestExtractDocText(t *testing.T) {
    data := map[string]any{
        "content": "Hello world",
    }
    text := extractDocText(data)
    // Verify extraction
}
```

#### `pkg/log/`

NEW: `pkg/log/log_test.go`

```go
func TestInfo_WritesToFile(t *testing.T) { ... }
func TestDebug_SuppressedByDefault(t *testing.T) { ... }
```

---

### S4.2 — Fix circuit breaker for streaming [D7]

**File:** `pkg/llm/retry.go`

**Fix:** Add circuit breaker check before streaming:
```go
func (r *RetryProvider) Stream(ctx context.Context, msgs []Message, opts ...Option) (<-chan StreamEvent, error) {
    cb := CBFor(r.name)
    if !cb.Allow() {
        return nil, fmt.Errorf("circuit breaker open for %s", r.name)
    }
    ch, err := r.inner.Stream(ctx, msgs, opts...)
    if err != nil {
        cb.RecordFailure()
        return nil, err
    }
    cb.RecordSuccess()
    return ch, nil
}
```

---

### S4.3 — Add BedrockProvider TokenReporter [D11]

**File:** `pkg/llm/bedrock.go`

**Fix:** Add `lastIn`, `lastOut` fields and implement `LastTokens()`:
```go
func (b *BedrockProvider) LastTokens() (in, out int64) {
    return b.lastIn, b.lastOut
}
```

Extract token counts from Bedrock API response
(`inputTokenCount`, `outputTokenCount` in the response body).

---

### S4.4 — MCP server multi-provider support [D12]

**File:** `internal/commands/mcp.go`

**Fix:** Replace hardcoded Anthropic key with `providerFor()` chain:
```go
// Use same provider resolution as other commands
apiKey := config.APIKey() // tries env → keychain → config.yaml
if apiKey == "" {
    // Try OpenRouter, Bedrock, etc.
    gcfg, _ := config.LoadGlobal()
    if gcfg.DefaultProvider != "" {
        // ...
    }
}
```

---

### S4.5 — Add missing model IDs to KnownModels [D19]

**File:** `pkg/llm/models.go`

**Fix:** Register the native/Bedrock model ID constants with their
capabilities and pricing in the `KnownModels` map.

---

### S4.6 — Fix ProviderNameFor missing cases [D9]

**File:** `pkg/agents/model_selector.go`

**Fix:** Add Plan → InitProvider, Research → WriteProvider cases:
```go
case StagePlan:
    if cfg != nil && cfg.LLM.InitProvider != "" {
        return cfg.LLM.InitProvider
    }
case StageResearch:
    if cfg != nil && cfg.LLM.WriteProvider != "" {
        return cfg.LLM.WriteProvider
    }
```

---

### S4.7 — Remove or document dead code [D10, D8]

**RaceComplete (D10):** If not planned for use, delete `pkg/llm/race.go`.
If planned for future use (e.g., racing Anthropic vs OpenRouter for
fastest response), add a `// TODO: wire into ...` comment explaining
the intent and a test.

**agent.Stream (D8):** Add clear godoc explaining the limitation:
```go
// Stream is not yet implemented. Use Run() with the onDelta callback
// parameter for streaming output. This method exists to satisfy the
// runtime.Runnable interface.
```

---

### S4 Checklist

- [ ] S4.1 Tests for research, exporter, mcpserver, gdocs, log
- [ ] S4.2 Circuit breaker for streaming
- [ ] S4.3 Bedrock TokenReporter
- [ ] S4.4 MCP multi-provider
- [ ] S4.5 KnownModels registry complete
- [ ] S4.6 ProviderNameFor complete
- [ ] S4.7 Dead code cleanup
- [ ] Coverage targets met
- [ ] Commit: `fix: test debt + provider completeness (Sprint 4)`

---

## Sprint 5: UX Polish + Session Intelligence (1 day)

### S5.1 — Session search [GAP 5]

**Files:**
- `internal/db/queries.go` (add search query)
- `internal/commands/session.go` (add `--search` flag)

**DB query:**
```go
func (db *DB) SearchMessages(ctx context.Context, query string, limit int) ([]SearchHit, error) {
    rows, err := db.QueryContext(ctx, `
        SELECT m.id, m.session_id, m.role, m.content, s.title
        FROM messages m
        JOIN sessions s ON s.id = m.session_id
        WHERE m.content LIKE '%' || ? || '%'
        ORDER BY m.created_at DESC
        LIMIT ?
    `, query, limit)
    // ...
}
```

**CLI:**
```
nqb session --search "hadith authentication"
nqb session -s "source verification"
```

**Output:** Show matching messages with session title, role, and
a context window (±2 lines around the match).

---

### S5.2 — Deprecate `nqb chat` in favor of `nqb .` [GAP 7]

**File:** `internal/commands/chat.go`

**Fix:**
1. Add deprecation notice:
   ```go
   Deprecated: "Use 'nqb .' for the full agent chat experience with tools and analysis.",
   ```
2. When `nqb chat` is run, print:
   ```
   Note: 'nqb chat' is deprecated. Use 'nqb .' for the full agent experience.
   Starting simple chat mode...
   ```
3. Keep the command working but don't invest further in it.

---

### S5.3 — Lab notebook / activity log [GAP 8]

**NEW file:** `internal/activity/log.go`

```go
// Package activity provides a human-readable activity log per book project.

type Entry struct {
    Timestamp time.Time
    Action    string // "write", "qa", "fix", "research", "chat"
    Chapter   int    // 0 for book-level actions
    Summary   string
    Details   string // optional longer description
}

func Append(bookDir string, entry Entry) error {
    path := filepath.Join(bookDir, ".naqb", "activity.md")
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    // ...
    fmt.Fprintf(f, "\n### %s — %s\n%s\n",
        entry.Timestamp.Format("2006-01-02 15:04"),
        entry.Action,
        entry.Summary)
    if entry.Details != "" {
        fmt.Fprintf(f, "\n%s\n", entry.Details)
    }
}
```

**Integration:** Call `activity.Append()` from:
- `agents/writer.go` — after writing a chapter
- `agents/qa.go` — after QA run
- `agents/fix.go` — after chapter fix
- `commands/research.go` — after research run
- `tui/agent_chat.go` — after agent chat session ends

---

### S5.4 — Fix nil context in epistemic.Load [D22]

**File:** `pkg/agent/context.go:25`

**Fix:** Pass actual context:
```go
if state, err := epistemic.Load(ctx, bookID); err == nil {
```

Requires threading `ctx` through `BuildChapterTask` (add parameter).

---

### S5.5 — Fix `languageDescs()` for non-Arabic/English [D23]

**File:** `pkg/agents/context_builder.go`

**Fix:** Add a generic language description path:
```go
func languageDescs(lang string) string {
    switch lang {
    case "ar", "ara":
        return arabicLanguageGuidance
    case "fr":
        return "Write in formal French academic style..."
    case "es":
        return "Write in formal Spanish academic style..."
    default:
        return "Write in clear, formal academic prose."
    }
}
```

---

### S5.6 — Legacy text fixes [D17, D18, D21]

**D17 — Extract `repeat()` to shared helper:**
Move to a `commands/helpers.go` file.

**D18 — Update config.go text:**
Change "Anthropic API key" to "API key" in prompts.

**D21 — Rename `grep_chunks` to `search_chunks`:**
```go
fantasy.NewAgentTool[SearchInput]("search_chunks",
    "Search research notes and chapter content using semantic similarity",
    // ...
)
```

---

### S5 Checklist

- [ ] S5.1 Session search
- [ ] S5.2 Deprecate `nqb chat`
- [ ] S5.3 Activity log
- [ ] S5.4 Fix nil context
- [ ] S5.5 Language descriptions
- [ ] S5.6 Legacy text fixes
- [ ] Tests for new code
- [ ] Commit: `feat: session search + activity log + deprecate chat`

---

## Sprint 6: Decisions + Cleanup (Half day)

### S6.1 — Decide on `pkg/runtime` fate [GAP 9, D26, D27]

**Decision matrix:**

| Option | Pros | Cons |
|---|---|---|
| **Integrate** as next-gen pipeline | Checkpoint/resume, conditional routing, tool integration | Requires rewriting pipeline stages |
| **Keep** as experimental module | No work needed now | Maintenance burden, confusion |
| **Remove** entirely | Clean codebase | Lose the work invested |

**Recommendation:** Keep for now, mark as experimental, fix the two bugs:
- D1 (parallel merge) — fix in Sprint 3
- D27 (non-deterministic tool order) — sort by name in `List()`

If the pipeline ever needs checkpoint/resume (for long-running multi-chapter
pipelines), `pkg/runtime` becomes the natural upgrade path.

---

### S6.2 — Fix `ToolRegistry.List()` ordering [D27]

**File:** `pkg/runtime/registry.go`

```go
func (r *ToolRegistry) List() []Tool {
    out := make([]Tool, 0, len(r.tools))
    for _, t := range r.tools {
        out = append(out, t)
    }
    sort.Slice(out, func(i, j int) bool {
        return out[i].Name() < out[j].Name()
    })
    return out
}
```

---

### S6.3 — Fix import wizard chapter number [D20]

**File:** `internal/tui/screen_import.go`

Add a chapter number field to the import wizard form.

---

### S6.4 — Clean up stub backends [GAP 1]

**Files:** `internal/store/vector/lancedb.go`, `zilliz.go`

**Fix:** Improve error messages to guide users:
```go
func NewLanceDB(_ embedding.Embedder) (*lanceDBStore, error) {
    return nil, fmt.Errorf("LanceDB backend is not yet implemented — use backend: chroma in your config")
}
```

Also remove lancedb/zilliz from any user-facing documentation or
config examples that suggest they work.

---

### S6.5 — Separate `pkg/runtime` SQLite dep [D26]

Move `checkpoint_sqlite.go` to `pkg/runtime/sqlite/` subpackage with
its own import. The main `pkg/runtime` module stays dependency-free.

---

### S6 Checklist

- [ ] S6.1 Runtime decision documented
- [ ] S6.2 Deterministic tool ordering
- [ ] S6.3 Import wizard chapter number
- [ ] S6.4 Stub backend error messages
- [ ] S6.5 SQLite dep separation
- [ ] Final `make check` across all modules
- [ ] Commit: `chore: cleanup sprint — runtime decision, stubs, ordering`

---

## Full Timeline

| Sprint | Focus | Effort | Gaps Closed |
|---|---|---|---|
| **S0** | Emergency bug fixes | 1-2 hours | D4, D5, D13, D14, D15, D24, D25 |
| **S1** | Build integrity + wiring | Half day | GAP 15, D2, D3, D6, D16, D28 |
| **S2** | Arabic export + provenance | 1 day | GAP 3, GAP 10 |
| **S3** | Pipeline evolution | 1-2 days | GAP 4, GAP 11, GAP 12, D1 |
| **S4** | Test debt + providers | 1 day | GAP 2, D7, D8, D9, D10, D11, D12, D19 |
| **S5** | UX polish | 1 day | GAP 5, GAP 7, GAP 8, D17, D18, D21, D22, D23 |
| **S6** | Decisions + cleanup | Half day | GAP 1, GAP 9, D20, D26, D27 |
| **Total** | | **~5-6 days** | **43/43 gaps** |

## Gaps Deferred Beyond Plan

These gaps are documented but intentionally left for later:

| Gap | Reason |
|---|---|
| GAP 6 (Google Docs sync improvements) | Needs Composio OAuth setup first |
| GAP 13 (Knowledge graph CLI) | Low priority, works via agent |
| GAP 14 (YouTube docs) | Low priority |
| Feynman-inspired features (Watch, Tabsit, Ikhtilaf tables, etc.) | New features, not gap closure |

---

## Post-Plan Verification

After all sprints complete:

```bash
# Full build + test across all modules
make check

# Coverage check
make cover-text

# Verify no regressions
go vet ./...
for dir in pkg/*/; do (cd "$dir" && go vet ./...); done

# Verify docs updated
# - MEMORY.md reflects new packages
# - README.md reflects new commands/flags
# - Standards files updated for architecture changes
```
