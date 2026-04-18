# Tech Stack

## Core Language

**Go 1.24** — pipeline engine, API server, job queue, knowledge store, all CLI binaries.
Single binary deployment. No Python in the hot path.

## Binaries

| Binary | Purpose |
|---|---|
| `nqb` | Main CLI — project management, pipeline trigger, chat, TUI |
| `nqb-mcp` | Standalone MCP server (6 tools) |
| `naqb-style` | Style engine CLI — extract, blend, apply, diff author voice |

## Frameworks & Libraries (Go)

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI command structure |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `charm.land/fantasy v0.12.3` | Agent loop (ReAct pattern over any LLM provider) |
| `github.com/gin-gonic/gin` | REST API server |
| `modernc.org/sqlite` | Local SQLite (WAL+FK via PRAGMA, not DSN) |
| `pressly/goose/v3` | SQLite migrations |
| `github.com/blevesearch/bleve/v2` | Embedded BM25 full-text search |
| `github.com/milvus-io/milvus-sdk-go/v2` | Zilliz / Milvus vector store client |
| `github.com/google/uuid` | UUID generation |
| `github.com/dustin/go-humanize` | Human-readable sizes and timestamps |
| `github.com/sahilm/fuzzy` | Fuzzy search in TUI home |
| `github.com/fsnotify/fsnotify` | File watcher for live rebuild |
| `gopkg.in/yaml.v3` | YAML config and style image serialization |
| `github.com/anthropics/anthropic-sdk-go` | Anthropic API (v1.26.0, streaming + non-streaming) |

### Adopted from WeKnora (vendored, DB deps stripped)

| Package / File | What we take |
|---|---|
| `internal/infrastructure/chunker/splitter.go` | Recursive text chunker + parent-child chunking |
| `internal/models/embedding/` | OpenAI-compatible embedding HTTP clients |
| `internal/models/rerank/` | Reranker HTTP clients + composite score formula |
| `internal/searchutil/` | `ContentSignature`, `JaccardSimilarity`, `TokenizeContent` |

## Vector Store (priority order)

| Backend | Mode | Status |
|---|---|---|
| **Zilliz** (managed Milvus cloud) | Remote managed | Primary — first to implement |
| **LanceDB** | Embedded / local | Second — local single-user |
| **Chroma** | Embedded local | Third — dev/test only |

All three implement the same `VectorStore` interface. Switch via `VECTOR_DRIVER` config.

**Embedding model default:** Voyage AI `voyage-3-large` — dim=1024, best multilingual + Arabic.
**Dim is fixed at collection creation. Changing it requires full re-indexing.**

## Keyword Search

**Bleve** (`blevesearch/bleve/v2`) — embedded BM25, pure Go, no server.
For Arabic: pre-normalize via Python morphology microservice before Bleve indexing.

## Hybrid Retrieval Pattern

Vector (Zilliz/LanceDB/Chroma) + BM25 (Bleve) run concurrently.
Merge by content signature → rerank (composite score) → MMR (λ=0.7, Jaccard).
Graph traversal added as third branch when Neo4j is configured (future).

## LLM Providers

| Provider | Access | Used for |
|---|---|---|
| Anthropic (Claude) | Direct API + AWS Bedrock | Primary — all stages |
| Gemini 2.0 Flash | Google AI API | Deep research (`--deep` flag, Google Search grounding) |
| OpenRouter | API | Model routing / fallback |
| Ollama | Local HTTP | Offline / private deployments |

**Model routing per stage type:**
- Haiku — DECOMPOSE, CORRECT, TASHKEEL generation, SUMMARIZE (fast + cheap)
- Sonnet — COMPARE, VERIFY, COMPOSE, QA (balanced)
- Opus — CRITIQUE, SYNTHESIZE, نقد الإعراب, complex COMPOSE (best reasoning)

## API Keys (all in macOS Keychain)

| Key | Keychain service | Helper |
|---|---|---|
| Anthropic | `ANTHROPIC_API_KEY` | `config.APIKey()` |
| Gemini | `GEMINI_API_KEY` | `config.GeminiAPIKey()` |
| Composio | `COMPOSIO_API_KEY` | `config.ComposioAPIKey()` |
| Zilliz | `ZILLIZ_URI` + `ZILLIZ_API_KEY` | `config.ZillizConfig()` |
| Voyage AI | `VOYAGE_API_KEY` | `config.VoyageAPIKey()` |

Lookup order for all: env var → Keychain → `~/.naqb/config.yaml`

## Python NLP Microservices

Communicate with Go core over local HTTP. Optional — pipeline degrades gracefully if not running.

| Service | Library | Exposes |
|---|---|---|
| Tashkeel | Camel-Tools / Farasa | `POST /tashkeel` — add harakat to Arabic text |
| OCR | MinerU | `POST /ocr` — scanned PDF/image → Markdown |
| Morphology | Camel-Tools | `POST /normalize` — lemmatize + root extract for Bleve indexing |

## Storage

| Data | Store | Path |
|---|---|---|
| Agent sessions, messages | SQLite | `~/.naqb/naqb.db` |
| Job queue | SQLite (`internal/jobs/`) | `~/.naqb/naqb.db` |
| Vector embeddings | Zilliz / LanceDB / Chroma | per `VECTOR_DRIVER` |
| BM25 index | Bleve | `~/.naqb/bleve/` |
| Style images | YAML files | `~/.naqb/styles/` |
| Context stacks | YAML files | `~/.naqb/stacks/` |
| Research notes | Markdown + YAML frontmatter | `.naqb/research/` (per project) |
| Book chapters | Markdown | `chapters/` (per project) |
| Project config | YAML | `book.yaml` (per project) |
| Global config | YAML | `~/.naqb/config.yaml` |

## Interface Layer

| Interface | Status | Notes |
|---|---|---|
| CLI (`nqb`) | Shipped | Cobra commands |
| TUI | Shipped | Bubbletea — home, book view, outline, preview, chat |
| REST API | Planned (Phase 1.8) | Gin — project, pipeline, corpus, gate endpoints |
| MCP server | Shipped (`nqb-mcp`) | 6 tools |
| Web UI | Phase 2 | SaaS dashboard, gate review |

## WeKnora Reference

Source: https://github.com/Tencent/WeKnora (Go 1.24, MIT)
Role: Code reference + selective adoption. **Not a runtime dependency.**
Full adoption decisions: `standards/future/weknora-integration.md`
