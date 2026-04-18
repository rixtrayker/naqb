# Standards Applied — WeKnora Integration

## Commit Gate (from CLAUDE.md)

```bash
make check   # go build ./... && go vet ./... && go test ./...
```

Never commit with: compilation error, failing vet, failing test, missing docs.

## Testing Protocol

- Every new package → `_test.go` in same package
- Coverage targets enforced:
  - `internal/searchutil` ≥ 60%
  - `internal/chunker` ≥ 60%
  - `internal/embedding` ≥ 50% (skip network tests when key absent)
  - `internal/rerank` ≥ 50%
  - `internal/store` ≥ 55%
  - `internal/knowledge` ≥ 55%
  - `internal/style` ≥ 45% (skip LLM tests when key absent)
- All file I/O tests use `t.TempDir()`
- Network/LLM tests skip when credentials absent: `t.Skip("VOYAGE_API_KEY not set")`

## Package Conventions

- Module: `github.com/amr/naqb`
- No CGO dependencies
- No GORM (plain `database/sql`)
- No breaking changes to existing public APIs

## Store Interface Conventions

- All backends implement `VectorStore` / `KeywordStore` interfaces from `internal/store/interface.go`
- Backend selection via `VECTOR_DRIVER` env var or `vector.driver` config key
- Default backend: `chroma` (requires Chroma server) or keyword-only fallback
- `Close()` always safe to call multiple times

## Arabic Processing Standards

From `agent-os/standards/agents/llm-agents.md`:

- Research note frontmatter must include `language: ar` for Arabic content
- ContentSignature strips tashkeel before hashing (dedup across diacritical variants)
- Chunker must not split mid-Quranic-verse or mid-hadith chain
- Keyword search query lowercasing uses `strings.ToLower` (ASCII-safe for Arabic — Unicode fold via searchutil.NormalizeContent)

## DAG Pipeline Standards

From `agent-os/standards/pipeline/dag-engine.md`:

- Stage IDs must be unique within a template
- Cycles are rejected at load time (topological sort fails)
- Human gates write a `stage.blocked` event to the jobs table before returning `ErrGateBlocked`
- Parallel batches (same topological level) execute in goroutines with `errgroup`
- ContextDebt accumulates per run, not per stage

## Context Stack Standards

- Layers numbered 0–5 (foundational → analytical)
- Each layer stored separately; full content loaded on demand
- Braid points classified: AGREEMENT | CONFLICT | RESONANCE | SILENCE
- Parallel strand execution via goroutines; synthesis is sequential

## Style Engine Standards

- StyleImage serialized to YAML (human-readable, diffable)
- Fingerprint is SHA-256 of canonical JSON representation
- `Blend` weight is [0.0, 1.0] — 0.0 = pure A, 1.0 = pure B
- `Apply` in PromptMode injects style constraints before chapter write
- `Apply` in PostprocessMode runs a separate rewrite pass after draft
