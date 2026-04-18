# Product Roadmap

## Phase 1: Core Engine (Current — Single User)

The foundation that everything else builds on. All work is in Go core + Python NLP microservices.

### 1.1 — WeKnora Integration & Store Layer
Adopt from WeKnora (vendor, strip DB deps):
- [ ] `chunker/splitter.go` → `internal/chunker/` — add Arabic separators `،` `؛` `۔`
- [ ] `models/embedding/` → `internal/embedding/` — OpenAI-compatible + Voyage AI + Bedrock
- [ ] `models/rerank/` → `internal/rerank/` — composite score + threshold degradation
- [ ] `searchutil/` → `internal/searchutil/` — `ContentSignature`, `JaccardSimilarity`, `MMR`

Build the store layer:
- [ ] `internal/store/interface.go` — `VectorStore` + `KeywordStore` + `HybridStore` interfaces
- [ ] `internal/store/vector/zilliz.go` — primary vector backend (Milvus SDK)
- [ ] `internal/store/vector/lancedb.go` — local embedded backend
- [ ] `internal/store/vector/chroma.go` — dev/test backend
- [ ] `internal/store/keyword/bleve.go` — embedded BM25 (blevesearch/bleve)
- [ ] `internal/store/hybrid.go` — concurrent vector+keyword, merge, rerank, MMR

### 1.2 — Knowledge Layer
- [ ] `internal/knowledge/claim.go` — `Claim` struct, `ClaimRelation`, provenance chain
- [ ] `internal/knowledge/graph.go` — knowledge graph CRUD (claims + concepts + relations)
- [ ] `internal/knowledge/epistemic.go` — `EpistemicState` struct, persistence, query
- [ ] `internal/knowledge/ingestion.go` — contextual chunking pipeline (Anthropic pattern)
  - chunk → contextualize (Haiku) → embed → index vector + BM25
  - prompt cache full document across chunks

### 1.3 — DAG Pipeline Engine
Refactor/replace `internal/pipeline/pipeline.go`:
- [ ] `internal/pipeline/dag.go` — DAG stage graph (nodes, edges, fan-out, fan-in)
- [ ] `internal/pipeline/stage.go` — `Stage` interface + `StageDecl` (type, model, gate, deps)
- [ ] `internal/pipeline/executor.go` — parallel branch execution, event emission
- [ ] `internal/pipeline/gate.go` — `HUMAN_GATE`: blocking vs advisory, resume protocol
- [ ] `internal/pipeline/debt.go` — context debt tracking + resolution policy

Stage types: DECOMPOSE, COMPARE, VERIFY, CORRECT, TASHKEEL, CRITIQUE, SYNTHESIZE, COMPOSE, QA, HUMAN_GATE

### 1.4 — Context Engine
- [ ] `internal/context/stack.go` — `ContextStack`, `ContextLayer`, layer types
- [ ] `internal/context/braid.go` — `BraidedField`, `BraidPoint`, `InterferenceMap`
- [ ] `internal/context/processor.go` — parallel strand execution + synthesis pass
- [ ] `internal/context/debt.go` — context debt per stack
- [ ] `internal/context/registry.go` — stack library (`~/.naqb/stacks/`)
- [ ] `internal/context/arabic/` — Arabic-specific standard layers (isnād, balāgha, morphology)

### 1.5 — Style Engine
New binary + packages:
- [ ] `cmd/naqb-style/main.go` — standalone binary
- [ ] `internal/style/image.go` — `StyleImage` struct + serialization
- [ ] `internal/style/extract.go` — extraction pipeline (linguistic + structural + rhetorical + voice)
- [ ] `internal/style/apply.go` — prompt mode + postprocess mode
- [ ] `internal/style/blend.go` — blend, fork, cherry-pick, diff, fingerprint
- [ ] `internal/style/arabic.go` — binyan tracking, balāgha, diacritical policy, register
- [ ] `internal/style/registry.go` — `~/.naqb/styles/` local registry

### 1.6 — Pipeline Templates
- [ ] `corpus-builder` — ingest library → knowledge graph
- [ ] `classical-tahqeeq` — OCR fix → tashkeel → نقد الإعراب → tahqeeq → apparatus
- [ ] `modern-synthesis` — corpus → thesis/outline/explore → draft
- [ ] `quick-review` — decompose → summarize → key claims → triage verdict
- [ ] `translation` — tashkeel → segment → translate → notes → introduction
- [ ] `comparative` — align concepts → compare → agreements/disagreements/lineage
- [ ] `summarize` — layered summary (one-page / structured / deep)
- [ ] `explain-annotate` — pedagogical layer for target audience
- [ ] `rewrite-revamp` — style extraction → profile approval → rewrite to spec

### 1.7 — Arabic NLP Microservices (Python)
- [ ] `services/tashkeel/` — Camel-Tools / Farasa — HTTP API over localhost
- [ ] `services/ocr/` — MinerU for scanned manuscripts
- [ ] `services/morphology/` — lemmatization, root extraction, stop-word removal for Bleve

### 1.8 — API Layer
- [ ] REST API (Gin) — project CRUD, pipeline trigger, HUMAN_GATE respond, corpus query
- [ ] SSE streaming — live stage progress events to clients
- [ ] `cmd/nqb/main.go` refactor — `nqb` CLI talks to local API server

---

## Phase 2: Multi-User & Collaboration

- [ ] Auth: JWT + API key (row-level tenant_id, WeKnora model)
- [ ] Shared corpora: multiple researchers on one knowledge graph
- [ ] Role-based access: owner / editor / reviewer per project
- [ ] Async pipeline jobs with status tracking (upgrade `internal/jobs/`)
- [ ] Web UI: project dashboard, pipeline progress, HUMAN_GATE review interface
- [ ] LanceDB → Zilliz migration path for teams that outgrow local storage

---

## Phase 3: SaaS Platform

- [ ] Multi-tenant architecture (organization + member model)
- [ ] Billing: usage-based (pipeline runs, LLM tokens, storage)
- [ ] Public style registry: share and discover style images
- [ ] Public stack library: community-contributed context stacks
- [ ] Institutional licensing: bulk seats for universities and publishers
- [ ] Arabic manuscript digitization partnerships

---

## Deferred / Backlog

- **zk integration** — requires upstream: JSON output, stdin piping, RTL support
  (spec: `standards/future/zk-integration.md`)
- **Google Workspace sync** — Composio OAuth not set up yet
- **NotebookLM integration** — after core output pipeline ships
- **Neo4j GraphRAG** — evaluate after knowledge graph matures
- **`nqb changelog`** skill — session diary from git history
