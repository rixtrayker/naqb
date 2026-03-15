# Claude Skills Plan for نقب (naqb)

## Status: Decisions locked — ready to implement

### Decisions

| Question | Answer |
|---|---|
| Google Workspace auth | Composio managed auth (no GCP project needed) |
| Gemini key | Yes — `GEMINI_API_KEY` env var, deep-research activates when set |
| Google Docs direction | Bidirectional but manual: push chapters to **one master Doc** (all chapters as sections); also support importing from a Doc |
| Google Docs structure | One Doc for the whole book — chapters as top-level sections |
| Priority (ship first) | 1. `article-extractor` 2. `google-workspace` 3. `deep-research` |

---

---

## Skills to Add

### 1. `markdown-to-epub`
**What it does:** Converts Markdown files to professional EPUB format, Kindle-ready.

**Use case for نقب:**
Right now `nqb export --format epub` shells out to pandoc, which requires pandoc
to be installed and produces generic output. This skill would let `nqb export`
produce a polished, device-aware EPUB directly from the chapter `.md` files —
no pandoc dependency needed. Especially useful for Arabic books where pandoc's
RTL EPUB handling needs manual CSS patches.

**Trigger:** `nqb export --format epub` or `/export epub` in book TUI.

---

### 2. `pdf`
**What it does:** Programmatic PDF extraction, merging, annotation, and generation.

**Use case for نقب:**
Two directions:
- **Input:** Author has a reference PDF (academic paper, prior book). Use this
  skill to extract its text into a research note without manual copy-paste.
  Feeds directly into `.naqb/research/`.
- **Output:** Augment `nqb export --format pdf` — the skill handles page layout,
  font embedding, and RTL direction more reliably than the current XeLaTeX path
  for authors who don't have LaTeX installed.

**Trigger:** `nqb research --from-pdf <file>` (new flag) or `/import pdf` in TUI.

---

### 3. `deep-research`
**What it does:** Autonomous multi-step web research via Gemini Deep Research Agent.
Produces structured reports with citations.

**Use case for نقب:**
Drop-in upgrade for our Scout→Explorer→Scribe pipeline. Instead of our own
search provider + Scribe synthesis, call this skill and get a citation-backed
research report back — then split it into atomic notes with `buildFrontmatter`.
Particularly powerful for Arabic cultural/historical research topics where our
current DuckDuckGo/Brave search returns shallow results.

**Activation:** Requires `GEMINI_API_KEY`. When set, `nqb research --chapter N --deep`
uses this skill. Falls back to Scout→Explorer→Scribe when key is absent.

**Trigger:** `nqb research --chapter N --deep` flag.

---

### 4. `content-research-writer`
**What it does:** Writes content sections backed by live research. Adds citations,
improves hooks, gives section-by-section feedback.

**Use case for نقب:**
After `nqb write --chapter N` produces a first draft, this skill runs a
"research pass" — it finds claims that need citations, suggests stronger
section openers, and flags thin sections. Output feeds back into the chapter
file as an editor annotation layer before the QA stage.

**Trigger:** New `nqb enrich --chapter N` command, or `/enrich` in book TUI.

---

### 5. `youtube-transcript`
**What it does:** Downloads transcripts (with timestamps) from YouTube videos.

**Use case for نقب:**
Arabic tech and cultural content lives heavily on YouTube (lectures, conference
talks, podcasts). This skill lets the author paste a YouTube URL and get a
research note automatically: transcript → Scribe synthesis → frontmatter →
saved to `.naqb/research/`. Unlocks a huge corpus that web search misses.

**Trigger:** `nqb research --from-youtube <url>` or `/import youtube` in TUI.

---

### 6. `article-extractor`
**What it does:** Extracts clean full text + metadata from any web URL.
Strips ads, navigation, boilerplate.

**Use case for نقب:**
Replaces the raw HTTP fetch in `internal/research/explorer.go`. Right now
Explorer fetches HTML and passes it raw to the LLM, wasting context on
navigation menus and cookie banners. With this skill, Explorer gets clean
article text + title + author + date — better signal, cheaper LLM calls.

**Trigger:** Internal — transparently replaces `explorer.go` HTTP fetching.
No new user-facing command needed.

---

### 7. `git-operations`
**What it does:** Automates git operations including push, branch management,
pull requests, and remote sync.

**Use case for نقب:**
Our `pipeline/git.go` auto-commits after each stage but never pushes. This
skill adds the remote sync layer: after a successful chapter pipeline run,
offer to push to origin and optionally open a draft PR titled
`draft(NN): Chapter N first draft`. Authors collaborating with editors or
co-authors get a clean review workflow without leaving the TUI.

**Trigger:** `nqb pipeline --push` flag, or a new `p` keybinding in book TUI
after a pipeline run completes.

---

### 8. `google-workspace`
**What it does:** Full OAuth integration with Gmail, Google Docs, Sheets, Drive,
Calendar, Slides, and Chat. Auth via Composio managed OAuth (no GCP project needed).

**Use cases for نقب:**

- **نقب → Google Docs (primary):** `nqb sync gdocs` pushes all written chapters
  into **one master Google Doc** — each chapter becomes a top-level `Heading 1`
  section. Re-running the command updates the existing Doc in-place (same file,
  same URL). Editors leave comments/suggestions in Google's native UI without
  touching git.

- **Google Docs → نقب (manual import):** `nqb import --from-gdoc <url>` pulls
  the Doc content, converts it to Markdown, and writes it as `outline.md` or
  a research note. Useful when the author drafted the outline or early notes
  in Google Docs before starting the نقب project.

- **Google Sheets as bibliography:** `nqb research --from-sheet <url>` reads a
  Sheets bibliography (title, URL, chapter tag, notes) and imports each row
  as a frontmatter-tagged research note in `.naqb/research/`.

- **Google Drive backup:** `nqb export --sync-drive` uploads PDF/EPUB output
  files to a designated Drive folder after each export run.

**Auth:** Composio managed OAuth — `nqb config set composio-key <key>` stored
in `~/.naqb/config.yaml`.

**Trigger:** `nqb sync gdocs`, `nqb import --from-gdoc <url>`,
`nqb research --from-sheet <url>`, `nqb export --sync-drive`.
Also `/sync` in book TUI palette.

---

### 9. `changelog-generator`
**What it does:** Reads git commit history and produces user-friendly changelogs
categorised by type (features, fixes, etc.).

**Use case for نقب:**
Every book is a git repo. After a writing session (multiple commits), run
`nqb changelog` to get a human-readable "what changed in this session" summary —
not for developers, but for the author: "Chapter 3 first draft written,
Chapter 2 QA passed, 4 research notes added." Acts as a session diary and
can be appended to `pipeline-report.md`.

**Trigger:** `nqb changelog` command or `nqb status --verbose`.

---

### 10. `tapestry`
**What it does:** Creates interlinked knowledge networks from documents —
summarises, connects, and maps relationships between notes.

**Use case for نقب:**
After accumulating 20–50 research notes in `.naqb/research/`, run
`nqb tapestry` to get a visual map of how topics connect across chapters.
Surfaces hidden relationships ("chapter 3's AI safety notes overlap with
chapter 7's governance notes") that our current gap analysis misses.
Output is a Markdown knowledge graph that feeds into the context builder
alongside research notes.

**Trigger:** `nqb tapestry [--chapter N | --all]` or `/tapestry` in TUI.

---

### 11. `notebooklm`
**What it does:** Queries a Google NotebookLM notebook with source-grounded Q&A.
Answers are always backed by the sources loaded into the notebook.

**Use case for نقب:**
Author loads all research PDFs and web articles into a NotebookLM notebook
for the book. `nqb context --chapter N` queries NotebookLM for chapter-relevant
passages — getting source-grounded answers rather than hallucinated synthesis.
Critical for Arabic research books where factual accuracy and attribution matter.

**Trigger:** Optional tier in `buildResearchNotes()` when a NotebookLM notebook
URL is set in `book.yaml` (`llm.notebooklm_url`).

---

### 12. `docx`
**What it does:** Full Word document creation and editing with tracked changes,
comments, styles, and formatting.

**Use case for نقب:**
Our `exporter/docx.go` uses pandoc for a basic conversion. This skill replaces
it with a proper Word export: publisher-ready styles, correct RTL paragraph
direction for Arabic, tracked-changes layer so editors can suggest edits,
and a table of contents that actually works. Publishers and academic editors
still live in Word — this makes نقب output compatible.

**Trigger:** `nqb export --format docx` (existing command, upgraded backend).

---

### 13. `xlsx`
**What it does:** Create and manipulate Excel spreadsheets with formulas,
charts, and data transformations.

**Use case for نقب:**
Generate a book progress dashboard: one row per chapter, columns for
word count, QA status, research note count, last commit date, export status.
`nqb status --xlsx` writes `progress.xlsx` that authors can share with
publishers or co-authors as a project status report.

**Trigger:** `nqb status --xlsx` or `nqb export --format xlsx` for the
progress report.

---

### 14. `csv` (CSV Data Summarizer)
**What it does:** Analyses CSV files and generates insights with visualisations.

**Use case for نقب:**
The vector store and word count data can be exported as CSV. This skill
analyses them: "Which chapters are shortest?", "Which research tags appear
most?", "Word count trend across the book." Feeds a `nqb analyse` command
that helps authors identify weak chapters before final export.

**Trigger:** `nqb analyse` command that exports internal data as CSV and
pipes it through this skill.

---

## Implementation Order

### Wave 1
1. `article-extractor` ✅ **SHIPPED** — `FetchPage` now uses Jina Reader (`r.jina.ai`), falls back to direct fetch. No new command, transparent upgrade.
2. `deep-research` ✅ **SHIPPED** — `GeminiSearcher` in `research/search.go`, `nqb research --deep` flag. Reads key from Keychain. Falls back gracefully.
3. `google-workspace` ⏳ **BLOCKED** — Composio key stored in Keychain (`COMPOSIO_API_KEY`). Blocked on OAuth flow setup (Nango deferred). Commands planned: `nqb sync gdocs`, `nqb import --from-gdoc <url>`.

### Wave 2
4. `youtube-transcript` — `nqb research --from-youtube`
5. `markdown-to-epub` — replace pandoc EPUB backend
6. `changelog-generator` — `nqb changelog` session diary

### Wave 3 (after core output pipeline complete)
7. `notebooklm` — optional tier in `buildResearchNotes`
8. `tapestry` — `nqb tapestry` knowledge graph
9. `content-research-writer` — `nqb enrich --chapter N`
10. `docx` — replace pandoc DOCX backend
11. `pdf` — `nqb research --from-pdf` import
12. `git-operations` — `nqb pipeline --push`
13. `xlsx` — `nqb status --xlsx`
14. `csv` — `nqb analyse`
