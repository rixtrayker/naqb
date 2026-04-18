# Inspiration: Feynman (getcompanion-ai/feynman)

> Source: https://github.com/getcompanion-ai/feynman
> Reviewed: 2026-04-18
> Purpose: Feature and architecture inspiration for naqb

Feynman is a terminal-based AI research agent (~5.5k stars, MIT) that automates
research workflows through multi-agent orchestration. TypeScript/Node.js on the
Pi coding agent runtime with AlphaXiv for academic paper search.

---

## Key Ideas Worth Stealing

### 1. Agent Definitions as Markdown Files

Feynman defines each agent (Researcher, Writer, Verifier, Reviewer) as a plain
`.md` file in `.feynman/agents/`. No code, no JSON schema -- just natural
language instructions with explicit constraints.

**naqb adaptation:**
- `.naqb/agents/` directory with persona files per agent role
- `writer.md`, `qa-reviewer.md`, `researcher.md`, `editor.md`
- Each contains: role description, integrity rules, tool permissions, output format
- Per-project overrides: `<book>/.naqb/agents/writer.md` shadows global
- Version-controlled, human-editable, swappable per book genre

### 2. Integrity Commandments (Source Verification Rules)

Each Feynman agent embeds six "integrity commandments":
1. Every named source requires a verifiable URL; fabrication prohibited
2. Projects/papers must be verified before citation
3. Details require direct inspection; inference forbidden without source access
4. Evidence entries mandatory; unverified claims excluded
5. Direct reading required; no title-based summaries
6. Status must distinguish direct claims, inferences, and unresolved questions

**naqb adaptation -- Arabic scholarly integrity rules:**
- Verify isnad (chain of transmission) before citing hadith
- Cross-reference hadith gradings across major collections
- Cite precise volume/page/edition for classical texts
- Distinguish between a scholar's direct statement vs. student's narration
- Mark claims as: VERIFIED / UNVERIFIED / INFERRED / BLOCKED
- Never conflate Quranic tafsir with the Quran text itself
- Already partially in place via `knowledge/claim.go` (8 claim types) +
  `knowledge/epistemic.go` -- needs surfacing into agent system prompts

### 3. Provenance Sidecars

Every Feynman research output produces a `.provenance.md` companion:
- Sources consulted and rejected
- Verification status of each claim
- Intermediate research files generated
- What was blocked or degraded
- Disk verification before completion

**naqb adaptation:**
- `chapters/ch-01.provenance.md` alongside `chapters/ch-01.md`
- Generated automatically by the QA stage
- Contents: sources cited (with verification status), research notes consulted,
  knowledge claims referenced, epistemic debt items, word count delta
- Links to the chapter's EpistemicState snapshot in SQLite
- Machine-readable YAML frontmatter + human-readable body

### 4. Tiered Processing by Document Size

Feynman uses three tiers for document summarization:

| Tier | Size | Strategy |
|---|---|---|
| 1 | < 8K chars | Direct read into context |
| 2 | 8K-60K chars | Windowed 6K extraction with progressive notes |
| 3 | > 60K chars | Parallel subagents on 6K chunks with 500-char overlap |

**naqb adaptation:**
- Already have `chunker/SplitTextParentChild` for splitting
- Add tier-aware processing to `context_builder.go`:
  - Small source (<8K): inline into context prompt
  - Medium (8-60K): extract key passages with sliding window
  - Large (>60K, common for classical Arabic texts): parallel chunk
    processing, then synthesis
- Especially important for multi-volume works (tafsir, sharh collections)

### 5. Skills as Declarative Metadata

Feynman separates **skills** (trigger + metadata in `SKILL.md`) from
**prompts** (full procedural instructions). Skills declare "what" and "when";
prompts declare "how."

19 bundled skills including: deep-research, literature-review, peer-review,
paper-code-audit, replication, source-comparison, autoresearch, watch, eli5,
session-search, preview.

**naqb adaptation:**
- Already planned in `agent-os/standards/future/skills-plan.md` (14 skills)
- Feynman validates the approach: each skill = one YAML/MD file declaring
  trigger phrases, required agents, output location, tools needed
- Could replace current hardcoded pipeline templates with skill files
- Skills directory: `~/.naqb/skills/` (global) + `<book>/.naqb/skills/` (local)

### 6. Autoresearch: Iterative Improvement Loops

Four-phase cycle: Gather -> Environment -> Confirm -> Run.
Agent modifies, benchmarks, records, decides retention, iterates.

**naqb adaptation -- Chapter Quality Loop:**
1. Write chapter (or section)
2. Run QA (deterministic + LLM audit)
3. Measure quality metrics (source coverage, coherence, word count target)
4. If below threshold: generate revision plan, revise, re-QA
5. Iterate until quality gate passes or max iterations reached
6. Record attempt history in provenance sidecar
- Currently naqb pipeline is single-pass; this would be the biggest upgrade
- Pairs naturally with `pipeline/debt.go` ContextDebt tracking

### 7. Research Watch (Recurring Monitoring)

Feynman's `/watch` establishes ongoing monitoring with baselines and
scheduled recurring checks.

**naqb adaptation:**
- `nqb watch --topic "hadith authentication methods"` -- monitor for new
  publications, new scholarly discussions
- Baseline scan stored in `.naqb/watch/<slug>-baseline.md`
- Periodic re-scan via cron or on `nqb .` startup
- Alert in agent chat: "3 new sources found since last check"
- Useful for living books that track evolving scholarly discourse

### 8. Source Comparison Matrices

Structured comparison across: source origin, primary claim, supporting
evidence, limitations, confidence assessment.

**naqb adaptation -- Ikhtilaf Tables:**
- Compare different scholars' positions on a given topic
- Columns: Scholar | Position | Evidence (dalil) | Strength | Counter-argument
- Auto-generated from knowledge graph relations (SUPPORTS/CONTRADICTS)
- Output as markdown table in chapter context or as standalone artifact
- Natural fit for `knowledge/graph.go` BFS traversal

### 9. ELI5 (Explain Like I'm 5)

Standardized simplification: one-sentence summary, big idea, how it works,
why it matters, what to be skeptical of, 3 key takeaways.

**naqb adaptation -- Tabsit (تبسيط):**
- `nqb tabsit --chapter 3` -- generate simplified summary
- Useful for: complex fiqh reasoning, hadith science terminology,
  theological debates, grammatical analysis
- Output in `.naqb/tabsit/ch-03-simple.md`
- Could also feed into a "reader's guide" appendix
- One strong analogy > many weak ones (Feynman principle)

### 10. Lab Notebook Pattern

Long-running workflows maintain a `CHANGELOG.md` recording:
- What changed, what failed, what's next, verification outcomes

**naqb adaptation:**
- `.naqb/changelog.md` per book project -- running log of agent activity
- Auto-appended by pipeline stages and agent chat sessions
- Records: chapters written/revised, sources added, QA results,
  editorial decisions, word count milestones
- Human-readable narrative, not just logs
- Complements SQLite session history with a scannable timeline

### 11. File-Based Agent Communication

Feynman subagents communicate through disk artifacts, not context injection.
Each agent reads/writes files rather than passing large blobs through memory.

**naqb adaptation:**
- Already partially doing this: `contexts/ch-XX-context.md` is a file-based
  handoff between context_builder and writer
- Extend to all inter-agent communication:
  - Research notes in `.naqb/research/` (already exists)
  - QA reports in `.naqb/qa/ch-XX-report.md`
  - Revision plans in `.naqb/revisions/ch-XX-plan.md`
- Keeps context windows lean; enables human review between stages

### 12. Source Priority Hierarchy

Feynman ranks sources: academic papers > official docs > primary datasets >
expert blogs >> SEO listicles > undated posts >> anonymous content.

**naqb adaptation -- Arabic Source Hierarchy:**

| Priority | Source Type |
|---|---|
| 1 (Highest) | Quran text (with specific ayah reference) |
| 2 | Mutawatir hadith from Sahihayn (Bukhari/Muslim) |
| 3 | Authenticated hadith from other major collections |
| 4 | Classical scholarly consensus (ijma') |
| 5 | Classical scholarly works (with tahqiq/edition info) |
| 6 | Contemporary peer-reviewed Islamic scholarship |
| 7 | Reputable contemporary scholars' published works |
| 8 | Academic theses and dissertations |
| 9 | Conference papers and working papers |
| 10 (Lowest) | Web sources, blogs, social media (with caveats) |

- Embed this hierarchy in the agent's system prompt
- Use it to weight claims in `knowledge/claim.go`
- QA stage flags citations that rely too heavily on low-priority sources

---

## Architecture Patterns to Note

### Separation of Concerns
```
skills/     -- WHEN to trigger (metadata)
prompts/    -- HOW to execute (procedures)
agents/     -- WHO does the work (personas + constraints)
extensions/ -- WHAT tools are available (capabilities)
```

naqb equivalent mapping:
```
skills/           -- pipeline templates / skill files
agents/           -- agent persona definitions
internal/agent/   -- tool implementations
pipeline/         -- execution engine
```

### Degraded Mode Philosophy
Feynman explicitly handles tool failures: mark blocked steps, continue with
available data, report gaps honestly. This maps directly to naqb's existing
`pipeline/debt.go` ContextDebt system (FAIL/DEGRADE/SUBSTITUTE/HUMAN_GATE).

### Session Continuity
JSONL session files in `~/.feynman/sessions/` with search across sessions.
naqb already has SQLite sessions in `internal/db/` -- more structured and
queryable. But the "search across sessions" UX is worth adding to agent chat.

---

## Priority Implementation Order

If adopting these ideas, suggested order:

1. **Provenance sidecars** -- low effort, high scholarly value
2. **Integrity commandments in system prompt** -- just prompt engineering
3. **Iterative quality loops** -- biggest pipeline upgrade
4. **Tiered document processing** -- important for classical texts
5. **Source comparison matrices** -- leverages existing knowledge graph
6. **Lab notebook / changelog** -- simple append-only pattern
7. **Agent definitions as Markdown** -- bigger refactor, but cleaner
8. **Tabsit (ELI5)** -- new agent tool, moderate effort
9. **Research watch** -- needs scheduling infrastructure
10. **Declarative skill system** -- replaces pipeline templates long-term

---

## What NOT to Copy

- **TypeScript/Node.js runtime** -- naqb is Go, and that's the right choice
  for a CLI tool (single binary, fast startup, strong typing)
- **GPU compute integration** -- irrelevant for scholarly writing
- **AlphaXiv dependency** -- naqb has its own research pipeline
  (Scout/Explorer/Scribe) which is more appropriate for Arabic sources
- **Pi agent framework** -- naqb uses charm.land/fantasy which is Go-native
  and well-integrated
- **Browser preview** -- naqb has pandoc export which is more appropriate
  for scholarly output (PDF/EPUB with proper Arabic typesetting)

---

## Current Gaps in naqb (Audit: 2026-04-18)

A honest assessment of what's missing, incomplete, or stubbed in the codebase.

### GAP 1: Stub Backends (Will Error at Runtime)

| Component | Status | Detail |
|---|---|---|
| `store/vector/lancedb.go` | STUB | All methods return `ErrNotImplemented` |
| `store/vector/zilliz.go` | STUB | All methods return `ErrNotImplemented` |
| `embedding/bedrock` | STUB | `NewBedrock()` returns stub; all calls error |

Only **Chroma** works as a vector store backend. LanceDB and Zilliz are
declared in the factory but will fail if configured. The Bedrock embedder
is similarly a placeholder.

**Impact:** Users who configure `backend: lancedb` or `backend: zilliz` in
their book.yaml will get a runtime error with no helpful guidance.

**Fix priority:** Medium. Chroma works. But should at least return a clear
error message pointing users to Chroma, or remove the options from docs.

---

### GAP 2: Packages Without Tests (12 packages)

| Package | Risk |
|---|---|
| `internal/exporter` | HIGH -- PDF/EPUB/DOCX export untested |
| `internal/gdocs` | HIGH -- Google Docs sync untested |
| `internal/mcpserver` | HIGH -- MCP server (6 tools) untested |
| `internal/store` (interface pkg) | LOW -- just type definitions |
| `internal/context/arabic` | MEDIUM -- 11 Arabic analytical layers untested |
| `internal/tui/components` | LOW -- UI components |
| `internal/tui/keys` | LOW -- key binding definitions |
| `internal/tui/theme` | LOW -- color theme definitions |
| `pkg/log` | LOW -- simple logger |
| `pkg/research` | HIGH -- Scout/Explorer/Scribe pipeline untested |
| `cmd/*` (all 3) | LOW -- entry points, thin wrappers |

The highest-risk untested packages are:
- **research pipeline** -- core feature, no tests at all
- **exporter** -- pandoc wrappers, easy to break with pandoc version changes
- **mcpserver** -- 6 tool handlers with no coverage
- **gdocs** -- HTTP client for Composio API, untested

---

### GAP 3: No Provenance Tracking

Chapters are written to `chapters/ch-XX.md` with no record of:
- Which sources were consulted during context building
- Which research notes were injected into the prompt
- What the LLM's confidence level was
- Whether any sources were unavailable (degraded context)

The EpistemicState system exists in `knowledge/epistemic.go` and is injected
via `agent/context.go`, but it doesn't produce a human-readable artifact.
There's no `.provenance.md` sidecar per chapter.

**Fix priority:** HIGH -- this is the #1 Feynman feature worth adopting.

---

### GAP 4: Single-Pass Pipeline (No Iterative Quality Loops)

The pipeline runs: context -> write -> QA (optional). If QA finds issues,
the user must manually run `nqb fix --chapter N`. There's no automated
write -> QA -> revise -> re-QA loop.

The `pipeline/debt.go` ContextDebt system tracks degradation but doesn't
trigger automatic remediation. The DAG executor (`pipeline/executor.go`)
supports parallel stages but not cycles/loops.

**Fix priority:** HIGH -- this is the biggest pipeline upgrade available.

---

### GAP 5: No Session Search

Sessions are stored in SQLite (`internal/db/queries.go`) with full message
history. But there's no way to search across sessions from the CLI or TUI.

`nqb session` lists sessions but can't search message content. The agent
chat has no `/search` command. Feynman's session search (across JSONL files)
is a clear gap.

**Fix priority:** MEDIUM -- useful for long-running book projects.

---

### GAP 6: Google Docs Sync is Fragile

`internal/gdocs/client.go` is fully implemented but:
- No tests (the HTTP calls to Composio are never mocked/tested)
- Hardcoded fallback user ID (`pg-test-f3eaa561-...`)
- No pull direction -- only push (chapters -> Google Doc)
- No conflict detection (overwrites entire doc content)
- No incremental sync (pushes all chapters every time)

**Fix priority:** LOW -- works for the basic use case but brittle.

---

### GAP 7: `nqb chat` vs `nqb .` Duplication

Two separate chat implementations:
- `commands/chat.go` -> `tui.RunChat()` -- simple REPL with truncated context
- `commands/open.go` -> `tui/agent_chat.go` -- full Fantasy agent with tools

The `nqb chat` command is a simpler, less capable version that truncates
chapter content at 8000 chars and doesn't use the agent tool system.
Users who discover `nqb chat` first get a worse experience than `nqb .`.

**Fix priority:** LOW -- could deprecate `nqb chat` in favor of `nqb .`.

---

### GAP 8: No Lab Notebook / Activity Log

No human-readable running log of what the agent did across sessions.
The SQLite database stores structured data but there's no:
- `.naqb/activity.md` or similar narrative log
- Summary of "what happened today" across agent sessions
- Record of editorial decisions and why chapters were revised

`internal/changelog/generator.go` generates changelogs from git commits,
but this is git-level (commit messages), not agent-level (what the LLM
decided, what sources it used, what it changed and why).

**Fix priority:** MEDIUM -- valuable for multi-week writing projects.

---

### GAP 9: Missing `pkg/runtime` Integration

`pkg/runtime/` contains a complete LangGraph-style StateGraph engine with:
- Nodes, edges, conditional routing
- Checkpoint persistence (SQLite-backed)
- Interrupt/resume support
- Tool integration

This is a **fully built, tested** runtime that appears to be **unused** by
the main application. It's untracked in git (listed in `.gitignore` or just
untracked), and no internal package imports it.

**Fix priority:** DECISION NEEDED -- either integrate it as the next-gen
pipeline engine (replacing `pipeline/dag.go`) or remove it to reduce
maintenance burden. It duplicates `pipeline/executor.go` functionality
with more features.

---

### GAP 10: Exporter Has No Arabic-Specific Handling

`internal/exporter/` wraps pandoc for PDF/EPUB/DOCX/Web export but:
- No RTL-specific pandoc flags (`--variable dir=rtl`)
- No Arabic font configuration (critical for proper rendering)
- No XeLaTeX template for Arabic typography
- No EPUB metadata for RTL reading direction
- Web export doesn't set `dir="rtl"` on HTML

For a tool focused on Arabic scholarly writing, the export pipeline should
handle bidirectional text, proper Arabic fonts, and RTL layout by default.

**Fix priority:** HIGH for Arabic books, low for English books.

---

### GAP 11: Context Stack System Unused in Practice

`internal/context/` has a sophisticated system:
- `stack.go` -- ContextLayer (positions 0-5), ContextStack (YAML serialization)
- `braid.go` -- BraidedField, BraidPoint (AGREEMENT/CONFLICT/RESONANCE/SILENCE)
- `processor.go` -- RunStrands (parallel goroutine processing)
- `arabic/layers.go` -- 11 standard Arabic analytical layers

Tests exist and pass, but it's unclear if this system is actually used
by the main pipeline. The context builder (`agents/context_builder.go`)
may be using a simpler approach while this sophisticated system sits idle.

**Fix priority:** MEDIUM -- the system is built and tested, needs wiring
into the actual context building flow.

---

### GAP 12: Style Engine Disconnected

`internal/style/` has a complete style engine:
- Extract linguistic/structural/rhetorical profiles from text
- Apply style constraints (prompt mode or postprocess mode)
- Blend, fork, diff, fingerprint style images
- Registry at `~/.naqb/styles/`

Plus a standalone CLI (`cmd/naqb-style/main.go`).

But `nqb fix --style` is the only connection point. The write pipeline
doesn't apply style constraints during initial chapter generation. The
style engine should be part of the standard write flow, not just fixes.

**Fix priority:** MEDIUM -- integrate into `agents/writer.go`.

---

### GAP 13: Knowledge Graph Not Queryable from CLI

`internal/knowledge/` has:
- 8 claim types, ClaimStore (SQLite-backed)
- 8 relation types, Graph with BFS shortest path
- EpistemicState with Load/Save/Accumulate/Summary

But there's no CLI command to:
- List claims for a chapter
- Query the knowledge graph
- View epistemic state
- Add manual claims or relations

The agent tool `knowledge_search` exists, but it's only accessible inside
the Fantasy agent loop, not from the command line.

**Fix priority:** LOW -- the system works internally, CLI access is a UX
improvement.

---

### GAP 14: YouTube Research (`pkg/youtube/`) Exists but Status Unknown

`pkg/youtube/transcript.go` with tests exists. It's imported by
`pkg/research/youtube.go`. But there's no documentation and it's unclear
if YouTube transcript fetching actually works or requires API keys.

**Fix priority:** LOW -- document or test in integration.

---

### GAP 15: `pkg/` Modules Not in Main Go Workspace Test

The `pkg/` directory contains 12 separate Go modules (each with their own
`go.mod`). Running `go test ./...` from the project root only tests
`internal/` packages. The `pkg/` modules are tested independently via
`go.work` but this isn't reflected in `make check`.

`pkg/log` and `pkg/research` have zero test files.

**Fix priority:** MEDIUM -- `make check` should cover all modules.

---

### Gap Summary Matrix

| # | Gap | Severity | Effort |
|---|---|---|---|
| 1 | Stub backends (LanceDB/Zilliz/Bedrock embed) | Medium | Low |
| 2 | 12 packages without tests | High | High |
| 3 | No provenance tracking | High | Medium |
| 4 | Single-pass pipeline (no iterative loops) | High | High |
| 5 | No session search | Medium | Low |
| 6 | Fragile Google Docs sync | Low | Medium |
| 7 | `nqb chat` vs `nqb .` duplication | Low | Low |
| 8 | No lab notebook / activity log | Medium | Medium |
| 9 | `pkg/runtime` StateGraph unused | Decision | Low |
| 10 | No Arabic-specific export handling | High | Medium |
| 11 | Context stack system unused in practice | Medium | Low |
| 12 | Style engine disconnected from write pipeline | Medium | Low |
| 13 | Knowledge graph not queryable from CLI | Low | Low |
| 14 | YouTube research undocumented | Low | Low |
| 15 | `pkg/` modules not in `make check` | Medium | Low |

### Recommended Attack Order

**Wave 1 (Quick wins, high impact):**
- #3 Provenance sidecars (pairs with Feynman inspiration)
- #10 Arabic RTL export flags
- #15 Fix `make check` to cover `pkg/` modules

**Wave 2 (Core pipeline upgrades):**
- #4 Iterative quality loops
- #12 Wire style engine into write pipeline
- #11 Wire context stacks into context builder

**Wave 3 (Test debt):**
- #2 Tests for exporter, mcpserver, research, gdocs

**Wave 4 (UX polish):**
- #5 Session search
- #8 Lab notebook / activity log
- #7 Deprecate `nqb chat`
- #13 CLI for knowledge graph

**Deferred / Decision needed:**
- #9 Decide on `pkg/runtime` fate
- #1 Stub backends (only if users request LanceDB/Zilliz)
- #6 Google Docs sync improvements (only if users need bidirectional sync)
- #14 YouTube research documentation

---

## Deep Audit: Code-Level Gaps (28 additional findings)

Beyond the 15 architectural gaps above, a deep code audit uncovered 28
additional issues — bugs, dead code, missing wiring, and inconsistencies.

### HIGH Severity (5)

#### D1. Parallel state merge is last-writer-wins

**File:** `pkg/runtime/graph.go:211-216`

```go
// Merge states: for simplicity, use the last non-zero state.
for _, s := range states {
    *state = s
}
```

When `InvokeParallel` runs concurrent nodes, only the last goroutine's
state survives. N-1 nodes' output is silently dropped. Any DAG with
truly concurrent stages will lose data.

#### D2. ContextDebt defined but never wired into execution

**Files:** `pkg/pipeline/debt.go:15-65`, `pkg/pipeline/dag_test.go:133`

`ContextDebt` (with `Record`, `HasViolations`, `Summary`) is fully
implemented and tested but never instantiated by `pipeline.Run()`,
`RunDAG()`, or any stage execution. Budget tracking uses
`llm.SessionBudget.Record()` instead. The policy-violation tracking
(FAIL/DEGRADE/SUBSTITUTE/HUMAN_GATE) documented in the DAG spec is
dead code.

#### D3. RESEARCH and SYNTHESIZE stage types declared but never registered

**File:** `pkg/pipeline/dag.go:16-17`

```go
StageTypeResearch   StageType = "RESEARCH"
StageTypeSynthesize StageType = "SYNTHESIZE"
```

These constants exist but no implementation is registered in the stage
registry. The DAG planner could generate plans referencing them, but
they would fail at runtime with "unknown stage type".

#### D4. `--reindex` flag in `nqb index` is a no-op

**File:** `internal/commands/index.go:19,96-98`

The flag is defined and accepted but never passed to `store.IndexChapter()`
or `store.IndexFile()`. It only controls printing a message. Documents
are always re-indexed regardless, making the flag misleading.

#### D5. `NewProviderFromGlobalConfig` returns empty model string

**File:** `pkg/agent/provider.go:68`

Returns `(provider, "", nil)` — empty model. The caller in
`openBookAtAgentChat()` works around this by separately calling
`agents.ModelFor()`, but any other caller gets an empty model leading
to API errors.

---

### MEDIUM Severity (11)

#### D6. `loadOutlineSection` returns entire outline, ignores chapter number

**File:** `pkg/agent/context.go:104-111`

Despite the name and `chapterNum` parameter, returns the full outline.
For large outlines this wastes context tokens and dilutes chapter-specific
guidance.

#### D7. Circuit breaker bypassed for streaming

**Files:** `pkg/llm/circuit_breaker.go`, `pkg/llm/retry.go:73`

`CBFor()` is only called from `RetryProvider.Complete()`. The `Stream()`
path says "Streaming is never retried — pass through directly". Since
agent chat (`nqb .`) uses streaming, circuit-breaker protection is
absent for the primary interactive use case.

#### D8. `agent.Stream()` returns "not yet implemented"

**File:** `pkg/agent/agent.go:227-229`

The `Stream()` method satisfies the `runtime.Runnable` interface but
always returns an error. Any code relying on `Runnable.Stream()` will
fail. This is a partial interface implementation.

#### D9. `ProviderNameFor` missing cases for Plan and Research stages

**File:** `pkg/agents/model_selector.go:98-118`

Has explicit cases for Write, QA/Gap/Conflict, Fix, Chat, Init but no
case for `StagePlan` or `StageResearch`. Falls through to `return ""`,
so the provider cannot be overridden per-book for these stages (though
the model can via `ModelFor()`).

#### D10. `RaceComplete` is dead code

**File:** `pkg/llm/race.go`

`RaceComplete()` fires two providers in parallel and returns the first
adequate response. Fully implemented but never called from anywhere
in the project.

#### D11. BedrockProvider missing TokenReporter interface

**File:** `pkg/llm/bedrock.go`

No `LastTokens()` method. The `TokenReporter` interface is checked via
type assertion in `RetryProvider` and `FallbackProvider`. When Bedrock
is used, token counts are always (0, 0), breaking cost tracking and
budget degradation.

#### D12. MCP server hardcoded to Anthropic only

**File:** `internal/commands/mcp.go:42-44`

Only accepts Anthropic API key and bypasses the multi-provider
architecture (`providerFor()`, `config.ProviderConfigFor()`).

#### D13. `nqb chat` truncation breaks Arabic UTF-8

**File:** `internal/commands/chat.go:93-96`

```go
if len(content) > 8000 {
    content = content[:8000] + "\n... (truncated)"
}
```

`len()` counts bytes, not characters. For Arabic (multi-byte UTF-8),
this slices mid-character, producing invalid UTF-8 in the system prompt.
Can cause LLM API errors or garbled output.

#### D14. `readPrompt` looks in wrong directory

**File:** `pkg/pipeline/reflection.go:216-222`

Looks in `bookDir/prompts/` but `config.InitBookDir()` creates
`config/prompts/`. User-customized prompts placed in the standard
location will never be found. The fallback system prompt masks this.

#### D15. Hardcoded Composio test user ID leaks to production

**File:** `internal/commands/sync.go:59-61`

When `cfg.Sync.ComposioUserID` is empty, falls back to
`"pg-test-f3eaa561-6583-4190-9d84-06e15fd4b522"` — a dev artifact
that will route data to the wrong account.

#### D16. Migration 003 FK without ON DELETE CASCADE

**File:** `internal/db/migrations/003_knowledge.sql`

`claim_relations` and `concept_claims` reference `claims(id)` without
`ON DELETE CASCADE`. Deleting a claim will fail with FK constraint
violation. Migrations 001 and 005 correctly use CASCADE.

---

### LOW Severity (12)

#### D17. `repeat()` cross-file coupling
`internal/commands/doctor.go` and `session.go` call `repeat()` defined
in `batch.go`. Fragile coupling — removing `batch.go` breaks two
unrelated commands.

#### D18. Legacy "Anthropic API key" text in config command
`internal/commands/config.go` `--set-key` prompt still says "Anthropic
API key" despite multi-provider support.

#### D19. Native/Bedrock model IDs not in KnownModels registry
`pkg/llm/models.go` — `ModelAnthropicHaiku`, `ModelAnthropicSonnet`,
`ModelAnthropicOpus`, `BedrockModelMiniMax*` are defined as constants
but absent from `KnownModels`. `nqb models` won't show them, cost
tracking returns zero.

#### D20. Import wizard hardcodes ChapterNum=1
`internal/tui/screen_import.go:92` — no way to specify which chapter
number the imported draft should be assigned to.

#### D21. `grep_chunks` tool name misleading
`pkg/booktools/research.go` — tool named `grep_chunks` actually calls
`store.QueryResearch()` (semantic/vector search), not grep.

#### D22. nil context in `epistemic.Load()`
`pkg/agent/context.go:25` — passes `nil` as context arg. Works now
but violates Go conventions and will panic if implementation ever adds
context-dependent behavior.

#### D23. `languageDescs()` only handles "ar" and default
`pkg/agents/context_builder.go` — books in fr/es/de get generic English
language description, potentially producing inappropriate style guidance.

#### D24. `fix.go` indentation artifact
`internal/commands/fix.go:64` — extra leading whitespace (4 extra tabs),
likely a merge artifact.

#### D25. `write.go` short desc says "Claude Sonnet" but model is configurable
`internal/commands/write.go:27` — misleading for users on non-Anthropic
providers.

#### D26. `pkg/runtime` heavy SQLite dependency for generic module
`pkg/runtime/go.mod` — depends on `modernc.org/sqlite` just for
`DBCheckpointer`. Couples a generic runtime to a specific persistence
technology.

#### D27. `ToolRegistry.List()` non-deterministic order
`pkg/runtime/registry.go:25-31` — iterates `map[string]Tool`, producing
non-deterministic tool ordering in LLM system prompts. Different runs
may generate inconsistent agent plans.

#### D28. `internal/changelog` not tracked in MEMORY.md
`internal/commands/changelog.go` imports `internal/changelog` which
exists, works, and has tests — but is absent from the architecture map.

---

### Combined Gap Summary (All 43 Gaps)

| Category | Count | Items |
|---|---|---|
| **Architectural gaps** | 15 | #1-#15 (original audit) |
| **Code bugs (HIGH)** | 5 | D1-D5 |
| **Missing wiring (MEDIUM)** | 11 | D6-D16 |
| **Polish/consistency (LOW)** | 12 | D17-D28 |
| **TOTAL** | **43** | |

### Critical Fix Priorities (bugs that will bite in production)

1. **D13** `nqb chat` UTF-8 truncation — will corrupt Arabic text
2. **D1** Parallel state merge — silently drops concurrent stage output
3. **D15** Hardcoded Composio test user ID — data goes to wrong account
4. **D5** Empty model string — API errors for any non-standard caller
5. **D14** Wrong prompt directory — user customizations silently ignored
6. **D4** `--reindex` no-op — flag does nothing, misleading users
7. **D16** Missing CASCADE — claim deletion fails with FK violation
