# WeKnora Integration — References

## WeKnora Source Pointers

**Repository:** WeKnora (Tencent, MIT License)

### Vendored Components

| WeKnora source | Our target |
|---|---|
| `internal/searchutil/` | `internal/searchutil/` |
| `internal/infrastructure/chunker/splitter.go` | `internal/chunker/splitter.go` |
| `internal/models/embedding/openai.go` | `internal/embedding/openai.go` |
| `internal/models/embedding/ollama.go` | `internal/embedding/ollama.go` |
| `internal/models/rerank/` | `internal/rerank/` |
| `chat_pipline/rerank.go` (composite score) | `internal/rerank/composite.go` |

### Patterns Adopted

**Composite rerank score (from `chat_pipline/rerank.go`):**
```
score = 0.6 × model_score + 0.3 × base_score + 0.1 × position_prior
```
Threshold degradation: retry at `0.7 × threshold`, floor `0.3`.

**Contextual chunk ingestion (Anthropic pattern):**
1. Chunk with parent-child splitter
2. Haiku call per chunk: `"<document>{full}</document>\n\nChunk: {chunk}\n\nGive context in 1–2 sentences."`
3. Embed contextualized text
4. Cache full document across chunks via `anthropic.BetaPromptCachingEnabled`

**HybridStore search flow:**
1. Concurrent vector + keyword dispatch
2. Merge by `ContentSignature` (dedup)
3. Composite rerank
4. MMR (λ=0.7) for diversity

## External Dependencies Added

| Package | Version | Purpose |
|---|---|---|
| `github.com/blevesearch/bleve/v2` | v2.x | BM25 keyword store |
| `github.com/amikos-tech/chroma-go` | latest | Chroma vector backend |

## External Dependencies Removed

| Package | Reason |
|---|---|
| `github.com/philippgille/chromem-go` | Replaced by HybridStore layer |

## Voyage AI

- **Endpoint:** `https://api.voyageai.com/v1`
- **Model:** `voyage-3-large`
- **Dimensions:** 1024
- **API format:** OpenAI-compatible (uses `openai.go` embedder with custom BaseURL)
- **Key helper:** `config.VoyageAPIKey()` — env `VOYAGE_API_KEY` → Keychain `VOYAGE_API_KEY` → config YAML

## Arabic Processing References

- **Unicode NFC normalization:** `golang.org/x/text/unicode/norm`
- **Tashkeel removal:** strip U+064B–U+065F (Arabic diacritics block)
- **Arabic separators:** U+060C (،), U+061B (؛), U+06D4 (۔)
- **Combining character note:** tashkeel are combining runes; rune-count offsets are correct but byte offsets require care
