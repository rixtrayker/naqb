# Future Roadmap: Beyond the Current Sprints

> This document describes the long-term trajectory of naqb after the current implementation plan (Sprint 0 + Sprint 2 + Sprint 3) is complete.

---

## Phase 1: Core Engine Completion (Current — Single User)

After the current sprints land, these items remain to finish Phase 1:

### 1.1 — Store Layer Completion

| Item | File | Status |
|------|------|--------|
| Vector store interface | `internal/store/interface.go` | Partial |
| Zilliz/Milvus backend | `internal/store/vector/zilliz.go` | TODO stub |
| LanceDB local backend | `internal/store/vector/lancedb.go` | TODO stub |
| Chroma dev backend | `internal/store/vector/chroma.go` | Partial |
| Bleve keyword backend | `internal/store/keyword/bleve.go` | Partial |
| Hybrid search (vector + BM25) | `internal/store/hybrid.go` | Partial |

**Blockers:** LanceDB Go SDK maturity, Zilliz cloud access.

### 1.2 — Knowledge Layer Completion

| Item | File | Status |
|------|------|--------|
| Claim struct + relations | `internal/knowledge/claim.go` | Draft |
| Knowledge graph CRUD | `internal/knowledge/graph.go` | Draft |
| EpistemicState persistence | `internal/knowledge/epistemic.go` | Partial |
| Contextual chunking pipeline | `internal/knowledge/ingestion.go` | Partial |

**Design:** Anthropic pattern — chunk → contextualize (Haiku) → embed → index vector + BM25. Prompt cache full document across chunks.

### 1.3 — DAG Pipeline Engine Completion

| Item | Status |
|------|--------|
| Stage interface + registry | ✅ Done |
| Parallel execution | ✅ Done |
| Human-in-the-loop gates | ✅ Done |
| ContextDebt tracking | Implemented but **unwired** |
| RESEARCH / SYNTHESIZE stages | Constants exist, **no implementations** |

**To finish:** Wire ContextDebt into executor, implement research/synthesize stage handlers.

### 1.4 — Context Engine Completion

| Item | File | Status |
|------|------|--------|
| ContextStack + layers | `internal/context/stack.go` | Draft |
| BraidedField / BraidPoint | `internal/context/braid.go` | Concept |
| Arabic analytical layers (11) | `internal/context/arabic/` | Draft |
| Stack registry (`~/.naqb/stacks/`) | `internal/context/registry.go` | Not started |

**To finish:** Wire context stacks into context builder (Sprint 3 covers Arabic layers), implement braid processor, build stack library.

### 1.5 — Style Engine Completion

| Item | File | Status |
|------|------|--------|
| Standalone binary | `cmd/naqb-style/main.go` | Skeleton |
| StyleImage + serialization | `internal/style/image.go` | Draft |
| Extraction pipeline | `internal/style/extract.go` | Draft |
| Apply (prompt + postprocess) | `internal/style/apply.go` | Partial |
| Blend / fork / diff | `internal/style/blend.go` | Draft |
| Arabic style support | `internal/style/arabic.go` | Draft |
| Style registry (`~/.naqb/styles/`) | `internal/style/registry.go` | Draft |

**To finish:** Wire style into write pipeline (Sprint 3 covers basic prompt mode), build style registry, implement blend/fork/diff.

### 1.6 — Pipeline Templates

| Template | Description | Status |
|----------|-------------|--------|
| `standard` | context → write → qa | ✅ Built-in |
| `thorough` | + conflict + gap | ✅ Built-in |
| `qa-only` | QA + conflict + gap | ✅ Built-in |
| `corpus-builder` | ingest library → knowledge graph | **Not started** |
| `classical-tahqeeq` | OCR → tashkeel → إعراب → tahqeeq → apparatus | **Not started** |
| `modern-synthesis` | corpus → thesis/outline → draft | **Not started** |
| `quick-review` | decompose → summarize → triage | **Not started** |
| `translation` | tashkeel → segment → translate → notes | **Not started** |
| `comparative` | align → compare → agreements/disagreements | **Not started** |
| `summarize` | layered summary (one-page / structured / deep) | **Not started** |
| `explain-annotate` | pedagogical layer for target audience | **Not started** |
| `rewrite-revamp` | style extraction → profile approval → rewrite | **Not started** |

### 1.7 — Arabic NLP Microservices (Python)

| Service | Tool | Interface | Status |
|---------|------|-----------|--------|
| Tashkeel | Camel-Tools / Farasa | HTTP API over localhost | **Not started** |
| OCR | MinerU | HTTP API over localhost | **Not started** |
| Morphology | lemmatization, root extraction | HTTP API over localhost | **Not started** |

**Design:** Python FastAPI services, Go client calls via `pkg/research/` pipeline.

### 1.8 — API Layer

| Item | Status |
|------|--------|
| REST API (Gin) | **Not started** |
| SSE streaming — live progress | **Not started** |
| `cmd/nqb` talks to local API | **Not started** |

---

## Phase 2: Multi-User & Collaboration

**Goal:** Enable teams of researchers to collaborate on shared corpora and knowledge graphs.

### 2.1 — Authentication
- JWT + API key authentication
- Row-level `tenant_id` (WeKnora model)
- Keychain integration for API keys

### 2.2 — Shared Corpora
- Multiple researchers on one knowledge graph
- Claim provenance tracks who added what
- Merge conflict resolution for overlapping claims

### 2.3 — Role-Based Access
- Owner / Editor / Reviewer per project
- Read-only access for reviewers
- Gate approval requires reviewer role

### 2.4 — Async Jobs
- Upgrade `internal/jobs/` from SQLite queue to proper job worker
- Webhook notifications on completion
- Retry with exponential backoff

### 2.5 — Web UI
- Project dashboard (chapters, progress, word counts)
- Pipeline progress viewer with live stage events
- HUMAN_GATE review interface (approve/reject with comments)
- Knowledge graph visualizer (Neo4j Browser-style)

### 2.6 — Store Scaling
- LanceDB → Zilliz migration path for teams outgrowing local storage
- Managed vector store option (no self-hosting)

---

## Phase 3: SaaS Platform

**Goal:** Turn naqb into a hosted, monetizable product.

### 3.1 — Multi-Tenant Architecture
- Organization + member model
- Per-organization billing and quotas
- Custom domains for institutional customers

### 3.2 — Billing
- Usage-based: pipeline runs, LLM tokens, storage
- Seat-based plans for teams
- Pay-as-you-go for individuals

### 3.3 — Public Registries
- **Style registry:** Share and discover style images (academic formal, journalistic, narrative, etc.)
- **Stack library:** Community-contributed context stacks (classical Arabic, computer science, medical, legal)
- **Template marketplace:** Pipeline templates for common workflows

### 3.4 — Institutional Licensing
- Bulk seats for universities and publishers
- SSO integration (SAML, OIDC)
- Custom onboarding for departments

### 3.5 — Arabic Manuscript Digitization
- Partnerships with libraries and archives
- OCR → tashkeel → critical edition pipeline
- TEI XML export for digital humanities

---

## Deferred / Backlog

These items are explicitly deferred until upstream dependencies mature or core phases complete:

### zk Integration
- Requires: JSON output mode, stdin piping, RTL support
- Spec: `agent-os/standards/future/zk-integration.md`
- Status: Blocked on upstream zk feature roadmap

### Google Workspace Sync
- Requires: Composio OAuth setup
- Status: Blocked on OAuth configuration

### NotebookLM Integration
- Requires: Core output pipeline ships first
- Status: Waiting for stable export format

### Neo4j GraphRAG
- Requires: Knowledge graph matures
- Status: Evaluate after Phase 1.2 completes

### `nqb changelog` Skill
- Session diary from git history
- Auto-generate change logs from commit messages
- Status: Nice-to-have, no blocker

---

## Decision Matrix: What to Build When

| If you need... | Build this next |
|----------------|-----------------|
| Stability for existing users | Sprint 0 (7 bugs) |
| Arabic book production | Sprint 2 (RTL export + provenance) |
| Better output quality | Sprint 3 (quality loops + style) |
| Research collaboration | Phase 1.2 (knowledge layer) + Phase 2.2 (shared corpora) |
| Team workflows | Phase 2 (multi-user + web UI) |
| Revenue | Phase 3 (SaaS + billing) |
| Manuscript digitization | Phase 1.7 (NLP microservices) + Phase 3.5 |

---

## Metrics to Track

| Metric | Current | Phase 1 Target | Phase 2 Target | Phase 3 Target |
|--------|---------|---------------|---------------|---------------|
| Test coverage (pkg/) | ~45% avg | 70% avg | 80% avg | 85% avg |
| Supported export formats | 4 (PDF/EPUB/DOCX/Web) | 4 + Arabic RTL | 4 + Arabic RTL | 4 + Arabic RTL + TEI |
| Pipeline templates | 3 | 12 | 12 | 12 + marketplace |
| Store backends | 1 (SQLite) | 3 (SQLite + LanceDB + Chroma) | 4 (+ Zilliz) | 4 + managed |
| Auth methods | 0 | 0 | 2 (JWT + API key) | 3 (+ SSO) |
| Context languages | 2 (ar + en) | 2 | 5 | 10+ |
| Concurrent users | 1 | 1 | 10/project | 1000/org |

---

*Last updated: 2026-04-15*
