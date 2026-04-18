# Knowledge Graph

## The Atomic Unit: A Claim

A **Claim** is a single assertable proposition extracted from the text.

- Finer than a paragraph; coarser than a word
- Every other operation (comparison, critique, tashkeel critique, apparatus generation)
  operates on claims, not raw paragraphs
- Claims are extracted by a `DECOMPOSE` stage; they are never written by hand

## Provenance Chain (Mandatory)

Every claim traces an unbroken chain:

```
book → chapter → paragraph → sentence → claim
```

A claim without full provenance is invalid and must be rejected at extraction time.
The provenance chain is what makes the critical apparatus auto-generatable — if it
is incomplete, apparatus generation cannot run.

## Claim Schema

```go
type Claim struct {
    ID         string        // uuid
    Text       string        // the proposition, verbatim or normalized
    SourceBook  string
    Chapter     int
    Paragraph   int           // 1-based index within chapter
    Sentence    int           // 1-based index within paragraph
    Type        ClaimType     // see below
    Confidence  float64       // 0.0–1.0; set by extracting model
    Concepts    []string      // linked concept IDs in the graph
    Relations   []ClaimRelation
}

type ClaimType string
const (
    ClaimAssertion  ClaimType = "assertion"
    ClaimDefinition ClaimType = "definition"
    ClaimExample    ClaimType = "example"
    ClaimCounter    ClaimType = "counter"
    ClaimCitation   ClaimType = "citation"
)

type ClaimRelation struct {
    Type   RelationType
    Target string  // target Claim.ID
}
```

## Relationship Types

```go
type RelationType string
const (
    RelSupports           RelationType = "supports"
    RelContradicts        RelationType = "contradicts"
    RelElaborates         RelationType = "elaborates"
    RelSummarizes         RelationType = "summarizes"
    RelCites              RelationType = "cites"
    RelDerivesFrom        RelationType = "derives_from"
    RelHistoricallyPrior  RelationType = "historically_precedes"
    RelSupersedes         RelationType = "supersedes"
)
```

Relationships are directional. The inverse is inferred but not stored. Adding a
new relationship type requires updating this standard.

## Retrieval: Always Three-Tier

Never retrieve claims or corpus passages using a single method.

| Tier | Method | Implementation |
|---|---|---|
| 1 | Dense vector (semantic similarity) | chromem-go embeddings on claim text |
| 2 | BM25 keyword | `keywordSearch()` on stored claim text + source fields |
| 3 | Graph traversal | Follow `Relations` chains up to depth-N from seed claims |

Combine all three tiers using **reciprocal rank fusion** (RRF):

```go
// score = Σ 1/(k + rank_i) for each tier i, k=60
func rrf(rankings [][]string, k int) []string { ... }
```

The `k=60` constant is the standard RRF default. Do not tune it per query.

## Contextual Chunking at Ingestion (Mandatory)

Before embedding any paragraph chunk from corpus ingestion:

1. Call LLM (Haiku) to generate a 50–100 token context string that situates the
   chunk within the full document structure
2. Prepend that context string to the chunk before embedding AND before BM25 indexing
3. Cache the full document across all chunk calls (prompt caching → 90% cost reduction
   on the document-level prefix)

```go
// Context generation prompt — runs once per chunk, cached across document
const chunkContextPrompt = `
Here is the full document: <document>{{.FullText}}</document>

Situate this chunk within the document in 1-2 sentences (50-100 tokens).
Do not summarize the chunk itself. Only provide context.

Chunk: <chunk>{{.ChunkText}}</chunk>
`
```

Naive chunking (embed raw paragraph text without context prefix) is **not permitted**
for corpus ingestion. It may be used only for single-chapter scratch-pad operations
where corpus quality is not required.

See `context/retrieval-patterns.md` Rule 2 for the full policy.
