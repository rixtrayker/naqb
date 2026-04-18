# Architecture

## Overview

`nqb` is a single Go binary. All functionality lives in `internal/` packages;
`cmd/nqb/main.go` is just a Cobra root that wires them together.

```
┌──────────────────────────────────────────────────────────────────┐
│                         nqb (binary)                             │
│                                                                  │
│  cmd/nqb/main.go   ← cobra root + dynamic completions           │
│                                                                  │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────────┐          │
│  │  commands/   │  │    tui/       │  │   vault/     │          │
│  │  (CLI cmds)  │  │  (Bubble Tea) │  │  (registry)  │          │
│  └──────┬───────┘  └──────┬────────┘  └──────────────┘          │
│         │                 │                                      │
│  ┌──────▼─────────────────▼────────────────────────────────┐    │
│  │                     agent/                               │    │
│  │  fantasy loop · 8 tools · EpistemicState injection       │    │
│  └─────────────────────────┬───────────────────────────────┘    │
│                             │                                    │
│  ┌──────────┐  ┌────────────▼───┐  ┌──────────────────────┐    │
│  │   llm/   │  │  pipeline/     │  │   exporter/          │    │
│  │(providers)│  │ legacy + DAG + │  │ (pandoc wrappers)    │    │
│  └──────────┘  │ gate + debt    │  └──────────────────────┘    │
│                └────────────────┘                               │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Retrieval Stack                                          │   │
│  │  store/  ← VectorStore + KeywordStore (Bleve) + Hybrid   │   │
│  │  searchutil/ · chunker/ · embedding/ · rerank/           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────┐  ┌──────────────────────┐  │
│  │  knowledge/                     │  │  context/            │  │
│  │  Claim + Graph + EpistemicState │  │  Stack + Braid +     │  │
│  │  + Ingestion pipeline           │  │  Arabic layers       │  │
│  └─────────────────────────────────┘  └──────────────────────┘  │
│                                                                  │
│  ┌──────────┐  ┌───────────────┐  ┌─────────────────────────┐  │
│  │  style/  │  │   db/         │  │   jobs/                 │  │
│  │ StyleImage│  │  SQLite+goose │  │  async queue+worker     │  │
│  │ extract/ │  │  8 migrations │  │                         │  │
│  │ blend/   │  └───────────────┘  └─────────────────────────┘  │
│  └──────────┘                                                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## Package Responsibilities

### `cmd/nqb`
Entry point. `nqb` alone → TUI home. `nqb .` → open current dir. Registers all subcommands.

### `internal/vault`
Obsidian-style vault registry stored at `~/.naqb/vault.yaml`.
Scans vault directories for subdirs containing `book.yaml`.
`ListProjects()` returns all projects sorted by modification time.

### `internal/config`
- `GlobalConfig` — API key, loaded from `~/.naqb/config.yaml` or `ANTHROPIC_API_KEY`
- `BookConfig` — per-project `book.yaml` CRUD
- `Template` — three built-in templates: `arabic-research`, `cs-book`, `general`

### `internal/llm`
Multi-provider LLM client with streaming support:
- `OpenRouterProvider` — OpenAI-compatible (default, MiniMax M2.5)
- `AnthropicProvider` — native Anthropic SDK
- `BedrockProvider` — AWS Bedrock Converse API

### `internal/agents`
The four pipeline stages, each independently callable:

| File | Stage | Input | Output |
|------|-------|-------|--------|
| `planner.go` | 0 — Plan | `InterviewAnswers` | `BookConfig` + `outline.md` |
| `context_builder.go` | 1 — Context | `BookConfig` + chapter N | `contexts/ch-NN-context.md` |
| `writer.go` | 2 — Write | context file | `chapters/ch-NN.md` |
| `qa.go` | 3 — QA | chapter file | `QAResult` + `pipeline-report.md` |

### `internal/pipeline`
`RunChapterPipeline` orchestrates stages 1→2→3 and calls `GitCommit` between each.

### `internal/exporter`
Pandoc wrappers for PDF (XeLaTeX + polyglossia for Arabic RTL), EPUB, DOCX, and static HTML.

### `internal/tui`
All Bubble Tea screens:

| File | Screen |
|------|--------|
| `home.go` | VSCode-style project picker |
| `book_view.go` | Sidebar + slash command palette |
| `outline_editor.go` | Visual chapter outline editor |
| `preview.go` | Glamour markdown renderer |
| `chat.go` | Streaming Opus chat REPL |
| `init_chat.go` | Multi-step init form |
| `keys.go` | Central keybinding definitions |
| `spinner.go` | Spinner wrapper for long tasks |

### `internal/research`
Scout→Explorer→Scribe research pipeline:
- `scout.go` — generates search queries from chapter outline
- `explorer.go` — fetches pages via Jina Reader (`r.jina.ai`), falls back to direct HTTP
- `scribe.go` — synthesises fetched content into YAML-frontmatter research notes
- `search.go` — `GeminiSearcher` (Google Search grounding, requires `GEMINI_API_KEY`)

### `internal/searchutil`
Pure utility functions (vendored from WeKnora, Arabic-extended):
- `NormalizeContent` — NFC + whitespace collapse
- `TokenizeContent` — word tokenizer
- `ContentSignature` — SHA-256 of tashkeel-stripped normalized text (first 16 hex chars) for dedup
- `JaccardSimilarity` — token overlap score

### `internal/chunker`
Recursive text splitter with Arabic separator support:
- `SplitText(text, separators, chunkSize, chunkOverlap)` → `[]Chunk`
- `SplitTextParentChild(text, parentSize, childSize, overlap)` → `[]ChunkPair`
- Arabic separators: `،` (U+060C), `؛` (U+061B), `۔` (U+06D4)
- Arabic protected patterns: hadith chains, basmala, tashkeel sequences

### `internal/embedding`
Embedder interface + implementations:
- `Embedder` interface: `Embed(ctx, texts) ([][]float32, error)` + `Dimensions() int`
- `NewOpenAI` — OpenAI-compatible (covers Voyage AI, Jina, Ollama)
- `NewVoyage` — Voyage AI (`voyage-3-large`, dim=1024)
- `NewOllama` — local Ollama endpoint
- `NewBedrock` — AWS Bedrock stub (future)

### `internal/rerank`
Cross-encoder reranking:
- `NullReranker` — passthrough (sorts by base score); used when no rerank API is configured
- `CohereReranker` — composite score: `0.6 × model + 0.3 × base + 0.1 × position_prior`
- Threshold degradation: retry at `0.7 × threshold`, floor `0.3`

### `internal/store`
Store layer interfaces + implementations:
- `interface.go` — `VectorStore`, `KeywordStore`, `HybridStore` interfaces; shared types: `VectorDoc`, `KeywordDoc`, `SearchResult`, `Filter`
- `keyword/bleve.go` — `BleveStore`: BM25 via `github.com/blevesearch/bleve/v2`; Arabic `lang/ar` analyzer; filter clauses → `BooleanQuery` must-matches
- `vector/factory.go` — `NewVectorStore(cfg)` dispatches on `cfg.Driver`
- `vector/chroma.go` — Chroma v1 REST HTTP client (no SDK dep); collection schema: id, vector[1024], book_id, chapter, paragraph, claim_type, language, content
- `vector/lancedb.go`, `vector/zilliz.go` — stubs returning `ErrNotImplemented`
- `hybrid.go` — `HybridStoreImpl`: concurrent vector+keyword dispatch, dedup by `ContentSignature`, rerank via `Reranker`, inline MMR (λ=0.7)
- `util/merge.go` — `MergeBySignature(a, b []Result) []Result`
- `util/mmr.go` — `ApplyMMR(results []Result, lambda float64, k int) []Result`

### `internal/search`
Local vector store (routing layer):
- `Open(bookDir)` — creates `.naqb/vectors/` and initialises the collection
- `IndexChapter` / `IndexResearchNote` / `QueryResearch` / `QueryChapters` — delegate to HybridStore
- `keywordSearch` — file-scan fallback (heading match: 3pts, body: 1pt); zero-config operation

### `internal/db`
SQLite persistence (`~/.naqb/naqb.db`). Opened once at startup via `db.Open(path)`.
- **WAL mode + FK** enforced via `PRAGMA` after open (not DSN — DSN params are ignored by modernc.org/sqlite)
- Goose migrations embedded in `migrations/` — run automatically on every startup
- Tables: `sessions`, `messages`, `jobs`, `claims`, `claim_relations`, `concepts`, `concept_claims`, `epistemic_states`
- `queries.go` — plain `database/sql` CRUD (no ORM); includes claim/relation/concept/epistemic CRUD

### `internal/knowledge`
Knowledge graph and epistemic state:
- `claim.go` — `Claim` struct (8 types: FACTUAL/INTERPRETIVE/METHODOLOGICAL/EVALUATIVE/CONTEXTUAL/RELATIONAL/NORMATIVE/HYPOTHETICAL); `ClaimStore` CRUD
- `graph.go` — `Graph`: SQLite-backed graph with 8 relation types (SUPPORTS/CONTRADICTS/QUALIFIES/ELABORATES/CITES/DERIVES_FROM/EXEMPLIFIES/NEGATES); BFS `ShortestPath`
- `epistemic.go` — `EpistemicState` JSON blob in SQLite; `Load/Save/Accumulate/Summary`; state injected into agent task prompts
- `ingestion.go` — `IngestDocument`: chunk → embed → upsert VectorStore + index KeywordStore

### `internal/agent`
charm.land/fantasy agent loop for agentic chapter writing:
- `agent.New(provider, modelID, db, bookDir, cfg)` → `agent.Run(ctx, task, sessionID, onDelta)`
- 8 tools: `read_file`, `write_file`, `search_research`, `run_qa`, `web_fetch`, `list_chapters`, `knowledge_search`, `grep_chunks`
- `provider.go` — builds `fantasy.Provider` from `ProviderConfig`
- `context.go` — `BuildChapterTask(bookDir, cfg, chapterNum, db...)` injects `EpistemicState.Summary()` when DB provided

### `internal/pipeline` (extended)
DAG pipeline engine (in addition to existing `RunChapterPipeline`):
- `dag.go` — `DAG` with topological sort → parallel batches; 8 `StageType`s (CONTEXT/WRITE/QA/CONFLICT/GAP/RESEARCH/SYNTHESIZE/CUSTOM)
- `executor.go` — `RunDAG(ctx, dag, input, emit)`: executes batches in parallel goroutines
- `gate.go` — `GateManager`: `HUMAN_GATE` writes `gate_blocked` job to DB; `ResumeGate` unblocks
- `debt.go` — `ContextDebt` accumulator: tracks token budget violations (FAIL/DEGRADE/SUBSTITUTE/HUMAN_GATE)
- `template.go` — load built-in templates (standard/thorough/qa-only) or custom YAML from `~/.naqb/templates/`

### `internal/context`
Context stacks and analytical braiding:
- `stack.go` — `ContextLayer` (position 0–5), `ContextStack` (YAML-serializable)
- `registry.go` — `StackRegistry` at `~/.naqb/stacks/`
- `braid.go` — `BraidedField`: parallel strand analysis + `BraidPoint` detection (AGREEMENT/CONFLICT/RESONANCE/SILENCE)
- `processor.go` — `RunStrands`: parallel strand goroutines → synthesis pass
- `arabic/layers.go` — 11 standard Arabic analytical layers: isnād-chain, manuscript-census, diacritical-variants, classical-grammar-rules, etymological-field, balāgha, ijāza-chain, semantic-shift, allusion-recognition, maqāmāt, arabic-morphology

### `internal/style`
Style image engine:
- `image.go` — `StyleImage` with `LinguisticProfile`, `StructuralProfile`, `RhetoricalProfile`, `VoiceProfile`, `ArabicProfile`
- `extract.go` — `Extract(ctx, texts, llmClient)` → deterministic metrics + LLM qualitative analysis
- `apply.go` — `Apply(ctx, img, content, mode, client)`: `PromptMode` (inject constraints) or `PostprocessMode` (LLM rewrite)
- `blend.go` — `Blend`, `Fork`, `Diff`, `Fingerprint` (SHA-256 of canonical JSON)
- `registry.go` — YAML-backed registry at `~/.naqb/styles/`

### `cmd/naqb-style`
Standalone CLI for the style engine:
```
naqb-style extract <files...> --output <name>
naqb-style apply <style> --to <chapter>
naqb-style blend <style-a> <style-b> --weight 0.5
naqb-style diff <style-a> <style-b>
naqb-style list | show <name> | fork <name> | fingerprint <name> | delete <name>
```

### `internal/jobs`
Async job queue backed by the SQLite `jobs` table:
- `types.go` — `JobType` constants + `*Payload` structs (write/qa/research/pipeline)
- `queue.go` — `Enqueue`, `Next`, `Complete`, `Fail`, `Cancel`, `Status`
- `worker.go` — `Worker.Run(ctx)` dispatches N goroutines draining the queue

### `internal/log`
Leveled logger (`DEBUG/INFO/WARN/ERROR`) → `~/.naqb/nqb.log`.
`NQB_DEBUG=1` drops to DEBUG and echoes to stderr.

---

## LLM Model Assignments (Default: OpenRouter)

| Stage | Default Model | Alternatives |
|-------|---------------|--------------|
| `nqb init` interview | MiniMax M2.5 | claude-haiku, claude-sonnet |
| `nqb write` drafting | MiniMax M2.5 | claude-sonnet, any OpenAI-compatible |
| `nqb qa` semantic audit | MiniMax M2.5 | claude-sonnet |
| `nqb chat` editing REPL | Claude Opus 4-5 (via OpenRouter) | claude-opus (native) |
| `nqb research` | MiniMax M2.5 | Any model |
| Context assembly | No LLM | Pure Go templating |

---

## Data Flow: `nqb init`

```
RunInitForm() [TUI]
    │
    ▼
InterviewAnswers{title, author, language, domain, synopsis, chapters, template}
    │
    ▼
RunPlanner(ctx, client, answers) [Haiku]
    │  → LLM generates YAML chapter list + outline.md
    ▼
PlannerResult{BookConfig, OutlineMD}
    │
    ▼
InitBookDir(bookDir, cfg)
    │  → creates directories, book.yaml, rules.yaml, default prompts, .gitignore
    ▼
writeTemplateFiles() [if template selected]
    │  → overrides rules.yaml, write.md, qa.md
    ▼
GitInit() + GitCommit("init: ...")
    │
    ▼
vault.RecordRecent()
```

## Data Flow: `nqb write --chapter N`

```
config.LoadBook(bookDir)
    │
    ▼
agents.WriteContextFile(bookDir, cfg, N)
    │  → reads outline.md section, previous chapter summaries, research/ notes
    │  → executes Go text/template → contexts/ch-NN-context.md
    ▼
agents.WriteChapter(ctx, client, bookDir, cfg, N, onDelta)
    │  → reads context file
    │  → streams Sonnet → chapters/ch-NN.md
    ▼
chapter file on disk
```

---

## Git Auto-Commits

After each meaningful pipeline stage:

```
init:          "My Book" — book initialized
context(01):   Chapter 1 context assembled
chapter(01):   Chapter 1 first draft
qa(01):        Chapter 1 QA complete
export(pdf):   PDF generated
```
