# Vector Store
**Status:** Decided
**Related:** knowledge-graph.md, retrieval-patterns.md, weknora-integration.md

---

## Decision

Three vector DB backends, in priority order:

| Priority | Backend | Mode | Use case |
|---|---|---|---|
| 1 | **Zilliz** (managed Milvus cloud) | Remote managed service | Production — primary backend |
| 2 | **LanceDB** | Embedded / local service | Single-user local, offline, fast iteration |
| 3 | **Chroma** | Embedded local | Development, testing, simplest setup |

Keyword search: **Bleve** (`blevesearch/bleve`) — embedded BM25 in Go, no external service,
runs in-process alongside any vector backend.

---

## Architecture: VectorStore Interface

All three backends implement one interface. Backend is a config switch — no code change required.

```go
// internal/store/vector.go

type VectorStore interface {
    // Indexing
    Upsert(ctx context.Context, docs []VectorDoc) error
    Delete(ctx context.Context, ids []string) error

    // Retrieval
    Search(ctx context.Context, query []float32, topK int, filter Filter) ([]SearchResult, error)
    SearchByID(ctx context.Context, id string) (*VectorDoc, error)

    // Lifecycle
    CreateCollection(ctx context.Context, cfg CollectionConfig) error
    DropCollection(ctx context.Context, name string) error
    Close() error
}

type VectorDoc struct {
    ID        string
    Vector    []float32
    Payload   map[string]any   // metadata: book, chapter, paragraph, claim type, etc.
    Content   string           // raw text (stored for retrieval, not embedded)
}

type SearchResult struct {
    Doc   VectorDoc
    Score float64
}

type Filter struct {
    Must    []FilterClause
    MustNot []FilterClause
}

type FilterClause struct {
    Field string
    Op    string   // "eq", "in", "range"
    Value any
}
```

Backend selected via `VECTOR_DRIVER` env var or `vector.driver` in config:
`zilliz` | `lancedb` | `chroma`

---

## Backend 1: Zilliz (Primary)

**What it is:** Managed Milvus cloud. Fully compatible with the Milvus Go SDK.
No infrastructure to operate — connect via URI + API key.

**Go SDK:** `github.com/milvus-io/milvus-sdk-go/v2`

**Connection:**
```go
client, err := milvus.NewClient(ctx, milvus.Config{
    Address: os.Getenv("ZILLIZ_URI"),   // e.g. https://xxx.zillizcloud.com
    APIKey:  config.ZillizAPIKey(),     // from keychain
})
```

**Collection schema for chunks:**
```go
schema := &entity.Schema{
    CollectionName: "naqb_chunks",
    Fields: []*entity.Field{
        {Name: "id",           DataType: entity.FieldTypeVarChar, PrimaryKey: true, MaxLength: 64},
        {Name: "vector",       DataType: entity.FieldTypeFloatVector, Dim: 1024},
        {Name: "book_id",      DataType: entity.FieldTypeVarChar, MaxLength: 64},
        {Name: "chapter",      DataType: entity.FieldTypeInt32},
        {Name: "paragraph",    DataType: entity.FieldTypeInt32},
        {Name: "claim_type",   DataType: entity.FieldTypeVarChar, MaxLength: 32},
        {Name: "language",     DataType: entity.FieldTypeVarChar, MaxLength: 8},
        {Name: "content",      DataType: entity.FieldTypeVarChar, MaxLength: 8192},
    },
}
```

**Index:** HNSW, cosine metric:
```go
idx, _ := entity.NewIndexHNSW(entity.COSINE, 16, 64)
client.CreateIndex(ctx, "naqb_chunks", "vector", idx, false)
```

**Filtering on search** (Zilliz supports boolean expressions):
```go
results, _ := client.Search(ctx, "naqb_chunks",
    nil,
    fmt.Sprintf("book_id == \"%s\" && chapter == %d", bookID, chapterNum),
    []string{"id", "content", "chapter", "paragraph"},
    []entity.Vector{entity.FloatVector(queryVec)},
    "vector",
    entity.COSINE,
    topK,
    sp,
)
```

**Zilliz-specific config:**
```yaml
vector:
  driver: zilliz
  zilliz:
    uri: ${ZILLIZ_URI}
    collection: naqb_chunks
    dim: 1024
    index_type: HNSW
    metric: COSINE
    m: 16
    ef_construction: 64
    ef_search: 64
```

---

## Backend 2: LanceDB

**What it is:** Embedded columnar vector DB built on Apache Arrow / Lance format.
No server process — opens a directory on disk. Extremely fast for local single-user use.

**Go SDK:** `github.com/lancedb/lancedb-go` (or HTTP API if Go SDK is immature)

**Storage:** A directory (e.g. `~/.naqb/lancedb/`) containing Lance table files.

**Schema:** Arrow schema maps to our `VectorDoc` struct. Built-in full-text search (Tantivy)
can serve as secondary keyword search — but we use Bleve as primary BM25 for consistency
across all backends.

**LanceDB-specific config:**
```yaml
vector:
  driver: lancedb
  lancedb:
    path: ~/.naqb/lancedb
    table: naqb_chunks
    dim: 1024
    metric: cosine
```

**When to use:** Local development, offline operation, single-machine deployments.
Fast enough for corpora up to ~10M vectors without a server process.

---

## Backend 3: Chroma

**What it is:** Embedded local vector DB. Simplest possible setup.
Python-native but has an HTTP server mode with a Go client.

**Go client:** `github.com/amikos-tech/chroma-go` (HTTP client to local Chroma server)
or embed via the HTTP API directly.

**When to use:** Development, unit tests, smallest possible footprint.
Not recommended for corpora beyond ~100k vectors.

**Chroma-specific config:**
```yaml
vector:
  driver: chroma
  chroma:
    host: http://localhost:8000
    collection: naqb_chunks
    dim: 1024
```

---

## Keyword Search: Bleve (BM25)

**What it is:** Pure Go full-text search engine. Embedded, no server. BM25 scoring.
Persistent index stored on disk alongside the vector index.

**Package:** `github.com/blevesearch/bleve/v2`

**Index setup:**
```go
// internal/store/keyword.go

mapping := bleve.NewIndexMapping()
mapping.DefaultAnalyzer = "en"   // swap to "ar" analyzer for Arabic if available

index, err := bleve.Open(path)   // reopen existing
if err == bleve.ErrorIndexPathDoesNotExist {
    index, err = bleve.New(path, mapping)
}
```

**Arabic analyzer:** Bleve has limited Arabic support. For Arabic text, pre-process with the
Python morphology microservice (stemming, stop-word removal) before indexing. Store the
normalized form in Bleve, original in VectorDoc.Content.

**Index and search:**
```go
type BleveDoc struct {
    ID        string `json:"id"`
    Content   string `json:"content"`    // normalized for Arabic
    BookID    string `json:"book_id"`
    Chapter   int    `json:"chapter"`
    Language  string `json:"language"`
}

// Index
index.Index(doc.ID, BleveDoc{...})

// Search
query := bleve.NewMatchQuery(queryText)
query.Analyzer = "en"   // or "ar" when Arabic analyzer is available
req := bleve.NewSearchRequestOptions(query, topK, 0, false)
results, _ := index.Search(req)
```

**Storage:** `~/.naqb/bleve/` — one directory per project or a shared global index.

---

## Hybrid Retrieval: Vector + BM25

Both run concurrently. Results merged by content signature. Reranked. MMR applied.

```go
// internal/store/hybrid.go

func (h *HybridStore) Search(ctx context.Context, query string, vec []float32, topK int, filter Filter) ([]SearchResult, error) {
    var (
        vectorResults  []SearchResult
        keywordResults []SearchResult
        wg             sync.WaitGroup
        mu             sync.Mutex
        errs           []error
    )

    wg.Add(2)
    go func() {
        defer wg.Done()
        res, err := h.vector.Search(ctx, vec, topK*3, filter)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        vectorResults = res
    }()
    go func() {
        defer wg.Done()
        res, err := h.keyword.Search(ctx, query, topK*3, filter)
        mu.Lock(); defer mu.Unlock()
        if err != nil { errs = append(errs, err); return }
        keywordResults = res
    }()
    wg.Wait()

    merged := mergeBySignature(vectorResults, keywordResults)
    reranked := h.reranker.Rerank(ctx, query, merged)
    return applyMMR(reranked, 0.7, topK), nil
}
```

Composite rerank score (from WeKnora pattern):
```
score = 0.6 × model_score + 0.3 × base_score + 0.1 × position_prior
```

MMR (λ=0.7): reduces redundancy using Jaccard token similarity across selected results.

---

## Package Structure

```
internal/
  store/
    interface.go       ← VectorStore + KeywordStore + HybridStore interfaces
    hybrid.go          ← HybridStore: concurrent vector+keyword, merge, rerank, MMR
    vector/
      zilliz.go        ← Zilliz/Milvus implementation
      lancedb.go       ← LanceDB implementation
      chroma.go        ← Chroma implementation
      factory.go       ← NewVectorStore(cfg) → VectorStore
    keyword/
      bleve.go         ← Bleve BM25 implementation
      arabic.go        ← Arabic pre-processing before Bleve indexing
    util/
      signature.go     ← ContentSignature() for dedup (from WeKnora searchutil)
      mmr.go           ← applyMMR(results, lambda, k)
      merge.go         ← mergeBySignature(vectorResults, keywordResults)
```

---

## Configuration

```yaml
# book.yaml or ~/.naqb/config.yaml

vector:
  driver: zilliz          # zilliz | lancedb | chroma
  dim: 1024               # embedding dimension — must match embedding model
  metric: cosine          # cosine | l2 | ip

  zilliz:
    uri: ${ZILLIZ_URI}
    collection: naqb_chunks

  lancedb:
    path: ~/.naqb/lancedb
    table: naqb_chunks

  chroma:
    host: http://localhost:8000
    collection: naqb_chunks

keyword:
  engine: bleve
  path: ~/.naqb/bleve
  arabic_normalize: true  # pre-process Arabic via Python microservice before indexing

retrieval:
  top_k: 20
  vector_weight: 0.6
  keyword_weight: 0.4
  rerank: true
  mmr_lambda: 0.7
```

---

## Embedding Dimension by Model

| Model | Dim | Notes |
|---|---|---|
| Voyage AI `voyage-3-large` | 1024 | Best for multilingual + Arabic |
| OpenAI `text-embedding-3-large` | 3072 (or 1536 truncated) | Strong English, decent Arabic |
| Jina `jina-embeddings-v3` | 1024 | Supports late chunking for Arabic |
| `BGE-M3` (self-hosted) | 1024 | Strong Arabic, free, needs GPU |

Default: **Voyage AI `voyage-3-large` at dim=1024**. Change `vector.dim` if switching model.
Dim is set once at collection creation — changing it requires re-indexing the entire corpus.
