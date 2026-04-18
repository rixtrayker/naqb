# Shaping Session — WeKnora Integration & Store Layer

**Date:** 2026-03-16
**Session:** 2026-03-16-1200

## Problem Statement

نقب currently uses chromem-go for vector search — a simple in-process store with no BM25, no reranking, no hybrid search. The research pipeline produces notes but the retrieval quality is limited.

## Appetite

6 weeks of focused engineering (not a spike — this is a production-grade upgrade).

## Solution Shaped

Vendor WeKnora's battle-tested Go components (chunker, embedding clients, reranker, searchutil) and build a layered store architecture on top:

1. **searchutil** — pure utility functions (tokenization, dedup, Jaccard)
2. **chunker** — parent-child chunking with Arabic separators
3. **embedding** — Embedder interface (OpenAI-compat, Voyage AI, Ollama, Bedrock stub)
4. **rerank** — Reranker interface with composite scoring + NullReranker passthrough
5. **store** — VectorStore + KeywordStore (bleve/BM25) + HybridStore (concurrent dispatch + MMR)
6. **knowledge** — Claim graph + EpistemicState + contextual ingestion pipeline
7. **pipeline/dag** — DAG executor replacing linear stage runner
8. **context** — ContextStack + BraidedField strand processing
9. **style** — StyleImage extract/apply/blend registry + naqb-style CLI

## Key Decisions

### Vendor, don't `go get`
WeKnora is not published as a Go module. We vendor the relevant files directly into `internal/`, stripping GORM and DB imports.

### chromem-go → HybridStore
The existing `internal/search/store.go` is preserved for API compatibility but routes through the new store layer. The keyword fallback (file-scan) is kept for zero-config operation.

### Bleve for BM25
`github.com/blevesearch/bleve/v2` — pure Go, no CGO, good Arabic Unicode support.

### Voyage AI as default embedder
Model: `voyage-3-large`, dim=1024. OpenAI-compatible API so uses the existing openai.go client.

### DAG engine — no deletion of existing pipeline.go
The linear `Run()` function stays. DAG is an additive layer for new template-driven pipelines.

### Context stacks — file-based registry
Stacks stored as YAML in `~/.naqb/stacks/`. No DB required.

### Style images — file-based registry
Stored as YAML in `~/.naqb/styles/`. Fingerprinted for dedup.

## Rabbit Holes Avoided

- **zk integration** — still deferred (upstream blockers unchanged)
- **Google Workspace** — still blocked on OAuth setup
- **LanceDB/Zilliz backends** — stubbed with ErrNotImplemented; implement when SDKs mature
- **Arabic morphology microservice** — bleve hook degrades gracefully if not running
- **Bedrock embedding** — stubbed; Titan endpoint not yet available in all regions

## No-gos

- No breaking changes to existing `pipeline.Run()` signature
- No removal of keyword fallback in `internal/search/store.go`
- No CGO dependencies
