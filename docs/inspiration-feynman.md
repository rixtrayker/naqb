# Inspiration: Feynman (getcompanion-ai/feynman)

> Source: https://github.com/getcompanion-ai/feynman
> Reviewed: 2026-04-18
> Purpose: Feature and architecture inspiration for naqb

Feynman is a terminal-based AI research agent (~5.5k stars, MIT) that automates
research workflows through multi-agent orchestration. TypeScript/Node.js on the
Pi coding agent runtime with AlphaXiv for academic paper search.

---

## Key Ideas Worth Stealing

### 1. Agent Definitions as Markdown Files

Feynman defines each agent (Researcher, Writer, Verifier, Reviewer) as a plain
`.md` file in `.feynman/agents/`. No code, no JSON schema -- just natural
language instructions with explicit constraints.

**naqb adaptation:**
- `.naqb/agents/` directory with persona files per agent role
- `writer.md`, `qa-reviewer.md`, `researcher.md`, `editor.md`
- Each contains: role description, integrity rules, tool permissions, output format
- Per-project overrides: `<book>/.naqb/agents/writer.md` shadows global
- Version-controlled, human-editable, swappable per book genre

### 2. Integrity Commandments (Source Verification Rules)

Each Feynman agent embeds six "integrity commandments":
1. Every named source requires a verifiable URL; fabrication prohibited
2. Projects/papers must be verified before citation
3. Details require direct inspection; inference forbidden without source access
4. Evidence entries mandatory; unverified claims excluded
5. Direct reading required; no title-based summaries
6. Status must distinguish direct claims, inferences, and unresolved questions

**naqb adaptation -- Arabic scholarly integrity rules:**
- Verify isnad (chain of transmission) before citing hadith
- Cross-reference hadith gradings across major collections
- Cite precise volume/page/edition for classical texts
- Distinguish between a scholar's direct statement vs. student's narration
- Mark claims as: VERIFIED / UNVERIFIED / INFERRED / BLOCKED
- Never conflate Quranic tafsir with the Quran text itself
- Already partially in place via `knowledge/claim.go` (8 claim types) +
  `knowledge/epistemic.go` -- needs surfacing into agent system prompts

### 3. Provenance Sidecars

Every Feynman research output produces a `.provenance.md` companion:
- Sources consulted and rejected
- Verification status of each claim
- Intermediate research files generated
- What was blocked or degraded
- Disk verification before completion

**naqb adaptation:**
- `chapters/ch-01.provenance.md` alongside `chapters/ch-01.md`
- Generated automatically by the QA stage
- Contents: sources cited (with verification status), research notes consulted,
  knowledge claims referenced, epistemic debt items, word count delta
- Links to the chapter's EpistemicState snapshot in SQLite
- Machine-readable YAML frontmatter + human-readable body

### 4. Tiered Processing by Document Size

Feynman uses three tiers for document summarization:

| Tier | Size | Strategy |
|---|---|---|
| 1 | < 8K chars | Direct read into context |
| 2 | 8K-60K chars | Windowed 6K extraction with progressive notes |
| 3 | > 60K chars | Parallel subagents on 6K chunks with 500-char overlap |

**naqb adaptation:**
- Already have `chunker/SplitTextParentChild` for splitting
- Add tier-aware processing to `context_builder.go`:
  - Small source (<8K): inline into context prompt
  - Medium (8-60K): extract key passages with sliding window
  - Large (>60K, common for classical Arabic texts): parallel chunk
    processing, then synthesis
- Especially important for multi-volume works (tafsir, sharh collections)

### 5. Skills as Declarative Metadata

Feynman separates **skills** (trigger + metadata in `SKILL.md`) from
**prompts** (full procedural instructions). Skills declare "what" and "when";
prompts declare "how."

19 bundled skills including: deep-research, literature-review, peer-review,
paper-code-audit, replication, source-comparison, autoresearch, watch, eli5,
session-search, preview.

**naqb adaptation:**
- Already planned in `agent-os/standards/future/skills-plan.md` (14 skills)
- Feynman validates the approach: each skill = one YAML/MD file declaring
  trigger phrases, required agents, output location, tools needed
- Could replace current hardcoded pipeline templates with skill files
- Skills directory: `~/.naqb/skills/` (global) + `<book>/.naqb/skills/` (local)

### 6. Autoresearch: Iterative Improvement Loops

Four-phase cycle: Gather -> Environment -> Confirm -> Run.
Agent modifies, benchmarks, records, decides retention, iterates.

**naqb adaptation -- Chapter Quality Loop:**
1. Write chapter (or section)
2. Run QA (deterministic + LLM audit)
3. Measure quality metrics (source coverage, coherence, word count target)
4. If below threshold: generate revision plan, revise, re-QA
5. Iterate until quality gate passes or max iterations reached
6. Record attempt history in provenance sidecar
- Currently naqb pipeline is single-pass; this would be the biggest upgrade
- Pairs naturally with `pipeline/debt.go` ContextDebt tracking

### 7. Research Watch (Recurring Monitoring)

Feynman's `/watch` establishes ongoing monitoring with baselines and
scheduled recurring checks.

**naqb adaptation:**
- `nqb watch --topic "hadith authentication methods"` -- monitor for new
  publications, new scholarly discussions
- Baseline scan stored in `.naqb/watch/<slug>-baseline.md`
- Periodic re-scan via cron or on `nqb .` startup
- Alert in agent chat: "3 new sources found since last check"
- Useful for living books that track evolving scholarly discourse

### 8. Source Comparison Matrices

Structured comparison across: source origin, primary claim, supporting
evidence, limitations, confidence assessment.

**naqb adaptation -- Ikhtilaf Tables:**
- Compare different scholars' positions on a given topic
- Columns: Scholar | Position | Evidence (dalil) | Strength | Counter-argument
- Auto-generated from knowledge graph relations (SUPPORTS/CONTRADICTS)
- Output as markdown table in chapter context or as standalone artifact
- Natural fit for `knowledge/graph.go` BFS traversal

### 9. ELI5 (Explain Like I'm 5)

Standardized simplification: one-sentence summary, big idea, how it works,
why it matters, what to be skeptical of, 3 key takeaways.

**naqb adaptation -- Tabsit (تبسيط):**
- `nqb tabsit --chapter 3` -- generate simplified summary
- Useful for: complex fiqh reasoning, hadith science terminology,
  theological debates, grammatical analysis
- Output in `.naqb/tabsit/ch-03-simple.md`
- Could also feed into a "reader's guide" appendix
- One strong analogy > many weak ones (Feynman principle)

### 10. Lab Notebook Pattern

Long-running workflows maintain a `CHANGELOG.md` recording:
- What changed, what failed, what's next, verification outcomes

**naqb adaptation:**
- `.naqb/changelog.md` per book project -- running log of agent activity
- Auto-appended by pipeline stages and agent chat sessions
- Records: chapters written/revised, sources added, QA results,
  editorial decisions, word count milestones
- Human-readable narrative, not just logs
- Complements SQLite session history with a scannable timeline

### 11. File-Based Agent Communication

Feynman subagents communicate through disk artifacts, not context injection.
Each agent reads/writes files rather than passing large blobs through memory.

**naqb adaptation:**
- Already partially doing this: `contexts/ch-XX-context.md` is a file-based
  handoff between context_builder and writer
- Extend to all inter-agent communication:
  - Research notes in `.naqb/research/` (already exists)
  - QA reports in `.naqb/qa/ch-XX-report.md`
  - Revision plans in `.naqb/revisions/ch-XX-plan.md`
- Keeps context windows lean; enables human review between stages

### 12. Source Priority Hierarchy

Feynman ranks sources: academic papers > official docs > primary datasets >
expert blogs >> SEO listicles > undated posts >> anonymous content.

**naqb adaptation -- Arabic Source Hierarchy:**

| Priority | Source Type |
|---|---|
| 1 (Highest) | Quran text (with specific ayah reference) |
| 2 | Mutawatir hadith from Sahihayn (Bukhari/Muslim) |
| 3 | Authenticated hadith from other major collections |
| 4 | Classical scholarly consensus (ijma') |
| 5 | Classical scholarly works (with tahqiq/edition info) |
| 6 | Contemporary peer-reviewed Islamic scholarship |
| 7 | Reputable contemporary scholars' published works |
| 8 | Academic theses and dissertations |
| 9 | Conference papers and working papers |
| 10 (Lowest) | Web sources, blogs, social media (with caveats) |

- Embed this hierarchy in the agent's system prompt
- Use it to weight claims in `knowledge/claim.go`
- QA stage flags citations that rely too heavily on low-priority sources

---

## Architecture Patterns to Note

### Separation of Concerns
```
skills/     -- WHEN to trigger (metadata)
prompts/    -- HOW to execute (procedures)
agents/     -- WHO does the work (personas + constraints)
extensions/ -- WHAT tools are available (capabilities)
```

naqb equivalent mapping:
```
skills/           -- pipeline templates / skill files
agents/           -- agent persona definitions
internal/agent/   -- tool implementations
pipeline/         -- execution engine
```

### Degraded Mode Philosophy
Feynman explicitly handles tool failures: mark blocked steps, continue with
available data, report gaps honestly. This maps directly to naqb's existing
`pipeline/debt.go` ContextDebt system (FAIL/DEGRADE/SUBSTITUTE/HUMAN_GATE).

### Session Continuity
JSONL session files in `~/.feynman/sessions/` with search across sessions.
naqb already has SQLite sessions in `internal/db/` -- more structured and
queryable. But the "search across sessions" UX is worth adding to agent chat.

---

## Priority Implementation Order

If adopting these ideas, suggested order:

1. **Provenance sidecars** -- low effort, high scholarly value
2. **Integrity commandments in system prompt** -- just prompt engineering
3. **Iterative quality loops** -- biggest pipeline upgrade
4. **Tiered document processing** -- important for classical texts
5. **Source comparison matrices** -- leverages existing knowledge graph
6. **Lab notebook / changelog** -- simple append-only pattern
7. **Agent definitions as Markdown** -- bigger refactor, but cleaner
8. **Tabsit (ELI5)** -- new agent tool, moderate effort
9. **Research watch** -- needs scheduling infrastructure
10. **Declarative skill system** -- replaces pipeline templates long-term

---

## What NOT to Copy

- **TypeScript/Node.js runtime** -- naqb is Go, and that's the right choice
  for a CLI tool (single binary, fast startup, strong typing)
- **GPU compute integration** -- irrelevant for scholarly writing
- **AlphaXiv dependency** -- naqb has its own research pipeline
  (Scout/Explorer/Scribe) which is more appropriate for Arabic sources
- **Pi agent framework** -- naqb uses charm.land/fantasy which is Go-native
  and well-integrated
- **Browser preview** -- naqb has pandoc export which is more appropriate
  for scholarly output (PDF/EPUB with proper Arabic typesetting)
