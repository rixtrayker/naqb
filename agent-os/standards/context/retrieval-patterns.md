# Retrieval Patterns

Implementation-focused rules for every context retrieval operation in the pipeline.
These rules apply to all stages, all lenses, and all agent tool calls.

---

## Rule 1 — Static Context Is Always Cached

Static content for a session:
- Book config (`book.yaml` fields: title, author, language, domain)
- Project outline (`EpistemicState.Outline`)
- Style image (`AuthorProfile.StyleImagePath`)
- Author profile (`EpistemicState.AuthorProfile`)
- Active stack definitions (the loaded full YAML for activated stacks)

**Always inject as a cached prefix.** After the first call in a session, token cost
is ~10% of face value (prompt caching).

Never re-inject static content as dynamic retrieval. If a field can change during
a pipeline run, it is dynamic, not static.

---

## Rule 2 — Contextual Chunking at Ingestion (Mandatory)

Before embedding any paragraph chunk from corpus ingestion:

1. Call Haiku to generate a 50–100 token context string situating the chunk in the document
2. Prepend the context string to the chunk text before embedding
3. Prepend the context string to the chunk text before BM25 indexing
4. Cache the full source document across all chunk calls for that document

```go
// Required: one Haiku call per chunk, with full-document prefix cached
func contextualizeChunk(ctx context.Context, docText, chunkText string) (string, error) {
    // docText is the cached prefix; chunkText is the current chunk
    // Returns: prefixedChunk = contextString + "\n\n" + chunkText
}
```

Cost: one Haiku call per chunk, but the full document is a cached prefix → ~10%
token cost for the document portion. The context generation call itself is cheap.

**Naive chunking is prohibited for corpus ingestion.** It is allowed only for
single-chapter scratch-pad operations where corpus retrieval quality is not required.

---

## Rule 3 — Multi-Vector Retrieval for Stacks and Corpus

Every retrieval operation on the stack library or corpus knowledge store uses two
parallel stores:

| Store | Contains | Used for |
|---|---|---|
| Vector store | Descriptions, one-sentence summaries, representative questions | Retrieval by semantic match |
| Docstore | Full definitions, full passage text, full stack YAML | Serving content after retrieval |

Process:
1. Embed the query (paragraph + task description)
2. Retrieve top-K from vector store by description match
3. Fetch full content for top-K results from docstore
4. Inject full content into context

Never retrieve full content directly by embedding full content. The description/full-content
split is mandatory for both the stack library and the claim/corpus store.

---

## Rule 4 — HyDE for Synthesis Retrieval

Before retrieving corpus passages for any `COMPOSE` or `SYNTHESIZE` stage:

1. Generate a **hypothetical** version of the target output (a draft of the section
   being synthesized) using Haiku — fast and cheap
2. Embed the hypothetical draft
3. Retrieve corpus passages using that embedding as the query

Never embed the section title, outline bullet, or task description as the retrieval
query for synthesis stages. The retrieval signal is the hypothetical output, not the prompt.

```go
// HyDE: generate hypothetical, then retrieve
hypo, _ := llm.Complete(ctx, llm.ModelHaiku, hydePrompt(sectionOutline))
passages, _ := corpus.Search(ctx, embed(hypo), topK)
```

This is not required for DECOMPOSE, CORRECT, or TASHKEEL stages where the query
naturally represents the retrieval target.

---

## Rule 5 — Semantic Memory with Deduplication

All pipeline observations are stored as semantic memories per project. On each
paragraph operation:

1. Embed the current paragraph + task
2. Search the memory store → retrieve top-K relevant memories
3. Inject top-K into the LLM call
4. After the call: extract new facts from the output
5. For each new fact: search existing memories → classify as ADD / UPDATE / DELETE / NOOP
6. Apply the classified operation to the store

**Never append blindly.** Each new observation must go through steps 4–6.
Deduplication threshold: cosine similarity > 0.90 → UPDATE or NOOP, not ADD.

Memory store compaction target: **~7,000 tokens** average for the full project
memory store. Trigger compaction when the store exceeds 12,000 tokens.

---

## Rule 6 — Late Chunking for Arabic Classical Text

For Arabic classical corpus ingestion, use **late chunking**:

1. Pass the full document through the transformer encoder
2. Pool token embeddings within chunk boundaries (not across them)
3. Chunk boundaries are defined by rhetorical / structural breaks, not fixed token counts

Standard fixed-token chunking is **prohibited** for classical Arabic text.
Rationale: classical Arabic has dense pronoun chains, frequent ellipsis, and
backward references across paragraph boundaries. Fixed-token chunking severs
these chains, producing semantically incomplete embeddings.

Late chunking is not required for modern Arabic (MSA) or non-Arabic text.
The ingestion pipeline selects the chunking strategy based on `book.yaml` fields:
`language: ar` + `register: classical` → late chunking.

---

## Rule 7 — Progressive Disclosure for Stack Library

| Stage | What is injected | Token cost |
|---|---|---|
| System prompt | Stack descriptions only (~1 sentence each, all stacks) | ~600 tokens total |
| Stage execution | Full YAML for activated stacks only | Paid only for activated stacks |

Implementation:
- Stack descriptions are a static cached prefix (see Rule 1)
- Full stack definitions are in the docstore, loaded on demand (see Rule 3)
- The system prompt instructs the LLM to name the stacks it selects; the engine
  then loads and injects the full YAML for those stacks before the main generation call

The stack library can grow to hundreds of stacks without impacting baseline token cost.

---

## Token Budget Per LLM Call

| Block | Budget | Cost tier |
|---|---|---|
| Cached static (outline, style image, stack definitions) | ≤ 6,000 tokens | ~10% (cached) |
| Retrieved dynamic (claims, corpus passages, memories) | ≤ 4,000 tokens | 100% |
| **Total effective** | **≤ 4,600 tokens** | weighted average |

Calls exceeding this budget must include a code comment explaining the justification.
Lens calls targeting a budget of ~3,500 tokens are preferred (see `knowledge/epistemic-state.md`).
