# Product Mission

## Problem

Serious scholarly and research work on books — especially classical Arabic texts — requires
the equivalent of a team of expert researchers, a philologist, and a typographer working
a text simultaneously. That team doesn't exist at scale, is prohibitively expensive, and
produces output that cannot be easily reproduced or iterated on.

Existing AI writing tools treat text as input/output. They have no concept of scholarly
apparatus, provenance, tashkeel critique, manuscript comparison, or the accumulated
epistemic state that makes a serious research project coherent over time.

## Target Users

**Phase 1 — Single researcher / author:**
A scholar who works with classical Arabic texts, large corpora of books, or multi-source
research projects. Needs to produce critical editions, comparative studies, translations,
or original synthesis works grounded in hundreds of sources.

**Phase 2 — Research groups:**
Teams of 2–10 researchers sharing a corpus, dividing pipeline stages, and collaborating
on a single scholarly output.

**Phase 3 — SaaS platform:**
Research institutions, publishers, and universities running large-scale book processing
pipelines. Multi-tenant, role-based access, billing.

## Solution

A **scholarly text intelligence engine** — not a writing assistant, not a chatbot.

The system runs configurable DAG pipelines over books and corpora. A single paragraph
can be processed multiple times through different analytical lenses (tashkeel critique,
tahqeeq, comparative analysis, style analysis) with results braided together into a
structured interference map. Every claim traces back to its source. The apparatus
generates itself from the provenance chain.

**Key differentiators:**
- Classical Arabic support at the philological level: tashkeel, نقد الإعراب, balāgha,
  isnād chain verification — not just language detection
- Project epistemic state: accumulated understanding persists across pipeline stages
  and makes every subsequent pass richer without re-injecting the full context
- Author style as a portable composable artifact: extract, blend, fork, apply — like
  Docker images for prose voice
- Nine configurable pipeline templates from triage to full critical edition
- Human-in-the-loop at any stage: blocking or advisory gates, per project and per stage
- Hybrid retrieval (vector + BM25 + graph) with contextual chunking, HyDE, and MMR —
  built for scholarly text, not web search
