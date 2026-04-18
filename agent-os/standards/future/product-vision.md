# Product Vision

## What This System Is

A **scholarly text intelligence engine** — not a writing assistant, not a chatbot.
The operational equivalent of a team of expert researchers, a philologist, and a
typographer, running as a configurable automated pipeline.

Single-user first. SaaS-aware architecture from day one.

## Three Operating Modes

| Mode | Description | Human role |
|---|---|---|
| **Autopilot** | Fully automated pipeline; human is notified at HUMAN_GATE checkpoints | Approve / correct at gates |
| **Copilot** | Real-time writing assistant; human drives, engine assists per keystroke / paragraph | Active throughout |
| **Chat** | Converse with the book or corpus via the epistemic state | Question / answer |

The same pipeline engine runs all three modes. Mode selection changes when HUMAN_GATEs
fire, not what stages run.

## Nine Pipeline Templates

Templates are named run configurations that define the DAG shape. A project selects
exactly one template; templates can inherit from each other.

| Template | Purpose | Primary output |
|---|---|---|
| `corpus-builder` | Ingest a library, build knowledge graph. Works at any scale (single article → thousands of books) | Populated knowledge graph + claim store |
| `classical-tahqeeq` | Critical edition with apparatus. Stages: OCR fix → tashkeel → نقد الإعراب → tahqeeq → compare editions → generate apparatus | Critical edition Markdown + apparatus JSON |
| `modern-synthesis` | New work from an existing corpus. Stages: thesis formation → outline → explore entry points → draft | Chapter drafts grounded in corpus |
| `quick-review` | Triage and orientation. Fully automated, fast. Targeted Q&A output | Structured summary + Q&A report |
| `translation` | Arabic text → target language with scholarly annotation layer | Translated text + translator's notes |
| `comparative` | Trace ideas across religious, philosophical, or historical traditions | Comparative analysis document |
| `summarize` | Layered summary: one-page abstract / structured summary / deep summary with claim map | Summary at each requested depth |
| `explain-annotate` | Add a pedagogical layer targeted to a specific audience | Annotated text with glosses |
| `rewrite-revamp` | Extract style profile → present for approval → rewrite to spec | Rewritten text matching target style image |

## Input Types

| Input | Handling |
|---|---|
| PDF (clean, born-digital) | Direct text extraction |
| Scanned manuscript | OCR via MinerU microservice → correction stage |
| EPUB | Unpack → chapter segmentation → standard text pipeline |
| Web articles / URLs | Jina Reader fetch → clean Markdown |

Input type is declared in project config. The pipeline engine selects the appropriate
ingestion head automatically.

## Primary Output Target

**Critical edition with apparatus** — every other output type is secondary.

The apparatus is auto-generated from the provenance chains in the knowledge graph.
No apparatus output without full provenance on every claim.

## Tech Stack Decisions

| Layer | Technology | Source |
|---|---|---|
| Pipeline engine, API, job queue | Go | Custom |
| Chunker | `internal/infrastructure/chunker/splitter.go` from WeKnora | **Adopt** — pure Go, zero deps. Add Arabic separators (`،` `؛` `۔`) |
| Embedding model clients | `internal/models/embedding/` from WeKnora | **Adopt** — clean OpenAI-compatible HTTP wrappers |
| Rerank model clients | `internal/models/rerank/` from WeKnora | **Adopt** — clean HTTP wrappers, MMR built-in |
| Hybrid retrieval pattern | WeKnora `CompositeRetrieveEngine` pattern | **Adopt pattern** — implement against chosen store |
| Vector store | pgvector (HNSW half-precision) | WeKnora migration as reference |
| ReAct agent loop | Native Go on top of `charm.land/fantasy` | Custom — WeKnora pattern as reference only |
| Arabic NLP microservices | Python: Camel-Tools / Farasa / MinerU | Fully custom — WeKnora has zero Arabic NLP |
| Interface order | API-first → TUI → Web UI | Custom |
| MCP server | Go (`cmd/nqb-mcp/`) | Custom — WeKnora MCP is Python-only |
| DeepAgents | Not adopted | All patterns built natively in Go |
| WeKnora as service | Not adopted | Use as code reference and pattern source only |

See `future/weknora-integration.md` for the full adoption decision table.
