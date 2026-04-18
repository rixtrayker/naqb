# WeKnora Integration
**Source:** https://github.com/Tencent/WeKnora (Go 1.24, MIT)
**Status:** Code reference + selective adoption — NOT a runtime dependency

WeKnora is a production RAG/knowledge-base platform in Go. We do not run it as a service.
We adopt specific packages and patterns from its source.

---

## Adoption Decision Table

| WeKnora Component | File(s) | Decision | Notes |
|---|---|---|---|
| Recursive text chunker | `internal/infrastructure/chunker/splitter.go` | **ADOPT** | Pure Go, zero deps. Add Arabic separators: `،` `\u060c`, `؛` `\u061b`, `۔` `\u06d4` |
| Parent-child chunking | `splitter.go: SplitTextParentChild()` | **ADOPT** | Two-level chunking: large parent for context window, small child for embedding |
| Embedding model clients | `internal/models/embedding/` | **ADOPT** | OpenAI-compatible HTTP wrappers. `NewEmbedder(Config)` is fully standalone |
| Reranker clients | `internal/models/rerank/` | **ADOPT** | `NewReranker(Config)` is standalone. Composite score formula: `0.6*model + 0.3*base + 0.1*position` |
| MMR deduplication | `chat_pipline/rerank.go: applyMMR()` | **ADOPT PATTERN** | λ=0.7, Jaccard token similarity. Prevents redundant chunks in context |
| Search utilities | `internal/searchutil/` | **ADOPT** | Pure Go: `NormalizeContent()`, `TokenizeContent()`, `ContentSignature()`, `JaccardSimilarity()` |
| Hybrid retrieval pattern | `service/retriever/composite.go` | **ADOPT PATTERN** | Concurrent vector + keyword dispatch, merge by chunk ID + content signature |
| pgvector HNSW schema | `migrations/versioned/000002_embeddings.up.sql` | **REFERENCE** | `halfvec` (2 bytes/dim), HNSW index. Use as migration template |
| Rerank composite score | `chat_pipline/rerank.go` | **ADOPT PATTERN** | Score formula + threshold degradation (retry at `0.7×threshold`, floor `0.3`) |
| ReAct agent loop structure | `internal/agent/engine.go` | **REFERENCE** | We use `charm.land/fantasy`. Adopt the tool registry pattern |
| Tool registry | `internal/agent/tools/definitions.go` | **REFERENCE** | Tool name constants + `RegisterTool` / `ExecuteTool` pattern |
| Knowledge base REST design | `internal/router/router.go` | **REFERENCE** | Endpoint naming and grouping conventions |
| Contextual enrichment | `chat_pipline/` `CHUNK_SEARCH` stage | **REFERENCE** | `SkipContextEnrichment` toggle and enrichment prompt pattern |
| Pipeline event bus | `internal/event/` | **REFERENCE ONLY** | WeKnora's pipeline is linear. We build a DAG. Their event-bus is inspirational, not adoptable as-is |
| Multi-tenancy model | Row-level `tenant_id` on all tables | **REFERENCE** | Match this when SaaS layer is built |
| MCP server | `mcp-server/weknora_mcp_server.py` | **DO NOT USE** | Python only, wraps REST. Our Go MCP server (`cmd/nqb-mcp/`) already exists |
| Neo4j GraphRAG | `internal/application/repository/memory/neo4j/` | **DEFER** | Evaluate when knowledge graph requirements solidify |
| Asynq job queue | `hibiken/asynq` | **EVALUATE** | We have our own job queue (`internal/jobs/`). Asynq adds Redis dep. Use only if queue complexity grows |
| Arabic NLP | — | **NONE** | WeKnora has zero Arabic support. All Arabic NLP is custom Python microservices |
| DAG pipeline | — | **NONE** | WeKnora's pipeline is a hardcoded linear event chain. Our DAG is custom |
| Style engine | — | **NONE** | No equivalent in WeKnora |
| Epistemic state | — | **NONE** | No equivalent in WeKnora |
| Book / chapter ontology | — | **NONE** | WeKnora knows only KnowledgeBase / Knowledge / Chunk |

---

## Packages to Copy (not import as dependency)

WeKnora's `internal/` is not designed as a library — it pulls in GORM, Asynq, the full DI
container. Do not `go get` it. Instead vendor the specific files we adopt:

```
internal/infrastructure/chunker/splitter.go   → internal/chunker/splitter.go
internal/models/embedding/                    → internal/embedding/
internal/models/rerank/                       → internal/rerank/
internal/searchutil/                          → internal/searchutil/
```

Strip the GORM imports. These files have no database dependencies — they are pure HTTP
clients and pure Go functions.

---

## Chunker Adoption — Required Changes

`splitter.go` must be modified before use:

1. **Add Arabic separators** to the default separator list:
```go
var defaultSeparators = []string{
    "\n\n",
    "\n",
    "。",          // existing CJK
    "،",           // Arabic comma    \u060c
    "؛",           // Arabic semicolon \u061b
    "۔",           // Arabic full stop  \u06d4
    ".",
    " ",
    "",
}
```

2. **Add Arabic protected patterns** to the regex list (prevent splitting mid-Quranic-verse,
   mid-hadith chain, mid-tashkeel sequence).

3. **Test with classical Arabic text** — diacritical marks (tashkeel) are combining characters
   and affect rune counting. Verify `Start`/`End` offsets are correct for Arabic.

---

## Embedding Client Adoption — Configuration

WeKnora's `NewEmbedder(cfg ModelConfig, ...) (Embedder, error)` maps directly to our
`ProviderConfig` pattern. Mapping:

```go
// WeKnora ModelConfig → our ProviderConfig
cfg := embedding.ModelConfig{
    APIKey:     config.APIKey(),
    BaseURL:    "https://api.anthropic.com/v1",  // or Bedrock endpoint
    ModelName:  "voyage-3-large",
    Dimensions: 1024,
}
embedder, err := embedding.NewEmbedder(cfg)
```

Supported providers (from WeKnora): OpenAI-compatible, Ollama, Jina, Aliyun, Nvidia,
Volcengine. We add Voyage AI (OpenAI-compatible endpoint) and Bedrock Titan.

---

## Hybrid Retrieval — Pattern to Implement

From WeKnora's `CompositeRetrieveEngine` + `KeywordsVectorHybridRetrieveEngineService`:

```go
// Dispatch vector and keyword retrieval concurrently
var wg sync.WaitGroup
var vectorResults, keywordResults []*RetrieveResult

wg.Add(2)
go func() { defer wg.Done(); vectorResults, _ = store.VectorRetrieve(ctx, params) }()
go func() { defer wg.Done(); keywordResults, _ = store.KeywordRetrieve(ctx, params) }()
wg.Wait()

// Merge by content signature (deduplicate)
merged := mergeBySignature(vectorResults, keywordResults)

// Rerank with composite score
reranked := reranker.Rerank(ctx, query, merged)
reranked = applyMMR(reranked, lambda=0.7, k=TopK)
```

Add graph traversal as a third concurrent branch when Neo4j is configured.

---

## What WeKnowa Does Not Cover (build custom)

| Capability | Why Custom |
|---|---|
| Tashkeel generation and نقد الإعراب | Zero Arabic NLP in WeKnora. Python: Camel-Tools, Farasa |
| Classical Arabic morphology | Same — Python microservice |
| DAG pipeline with fan-out/fan-in | WeKnora pipeline is a hardcoded linear chain |
| HUMAN_GATE with resume | Not a concept in WeKnora |
| Style engine (`naqb-style`) | No equivalent |
| Project epistemic state | No equivalent |
| Context stacks / braided field | No equivalent |
| Book / chapter / outline model | WeKnora only knows KnowledgeBase / Knowledge / Chunk |
| Critical edition apparatus | No equivalent |
| Export pipeline (PDF, EPUB, DOCX) | No equivalent |
| Git auto-commit after stage | No equivalent |

---

## ReAct Tool Names to Adopt

From `internal/agent/tools/definitions.go` — adopt these as our tool name constants:

```go
const (
    ToolKnowledgeSearch    = "knowledge_search"
    ToolGrepChunks         = "grep_chunks"
    ToolListChunks         = "list_knowledge_chunks"
    ToolQueryGraph         = "query_knowledge_graph"
    ToolGetDocumentInfo    = "get_document_info"
    ToolWebSearch          = "web_search"
    ToolWebFetch           = "web_fetch"
    ToolTodoWrite          = "todo_write"
    ToolFinalAnswer        = "final_answer"
    ToolReadSkill          = "read_skill"
    ToolExecuteSkill       = "execute_skill_script"
)
```

We keep our existing tools (`read_file`, `write_file`, `run_qa`, `list_chapters`) and add
WeKnora-named tools where they overlap semantically.

---

## pgvector Schema Reference

From WeKnora migration `000002_embeddings.up.sql`:

```sql
CREATE TABLE embeddings (
    id            SERIAL PRIMARY KEY,
    chunk_id      UUID NOT NULL,
    knowledge_id  UUID NOT NULL,
    content       TEXT NOT NULL,
    dimension     INT  NOT NULL,
    embedding     halfvec(1024),   -- pgvector half-precision (2 bytes/dim)
    is_enabled    BOOLEAN DEFAULT TRUE,
    created_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ
);

CREATE INDEX ON embeddings USING hnsw (embedding halfvec_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

Use `halfvec` (not `vector`) — halves storage, negligible precision loss for retrieval.
HNSW parameters: `m=16`, `ef_construction=64` are WeKnora's production defaults.
