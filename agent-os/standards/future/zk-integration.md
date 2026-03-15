# Deferred: zk Integration

## Status

**Not implemented. Deferred until after core output features are shipped.**

zk (https://github.com/zk-org/zk) is a plain-text Zettelkasten CLI written in Go.
Its ideas are worth adopting, but full runtime integration requires upstream
contributions that are not yet done.

---

## What Was Analysed

zk is **not viable as a runtime dependency today** because it lacks:

- JSON output from `zk list` (output is human-readable only)
- Stdin content piping to `zk new` (no programmatic note creation)
- RTL / Arabic layout support in any output
- Export capability (PDF, EPUB, HTML)

---

## What Is Already Adopted (shipped, no zk dep)

These ideas from zk were implemented without any zk binary dependency:

| Idea | Where | Status |
|---|---|---|
| YAML frontmatter on notes | `internal/research/scribe.go` → `buildFrontmatter()` | ✅ Shipped |
| Keyword search fallback | `internal/search/store.go` → `keywordSearch()` | ✅ Shipped |

The YAML frontmatter (`title`, `chapter`, `tags`, `source`, `date`) already
makes `.naqb/research/` compatible with any tool that understands frontmatter,
including zk, Obsidian, and standard grep/fzf workflows.

---

## What Needs Upstream Work Before Integration

To use `zk` as an optional runtime dependency, these must be contributed to
the zk upstream or maintained in a fork:

1. **JSON output flag** — `zk list --format json` so results can be parsed
   programmatically without screen-scraping.

2. **Stdin note creation** — `zk new --stdin` or equivalent so `Scribe` can
   pipe note content directly instead of writing a temp file first.

3. **RTL / Arabic support** — `zk edit` and any rendered output must handle
   right-to-left text correctly for Arabic books.

---

## Integration Design (when prerequisites are met)

### New file: `internal/research/zk.go`

```go
func ZkAvailable() bool                                          // checks PATH
func EnsureZkNotebook(dir string) error                          // writes .zk/config.toml
func ZkSearch(dir, query string, topK int) ([]SearchResult, error) // zk list --format json
```

### `.zk/config.toml` to write into `.naqb/research/`

```toml
[notebook]
dir = "."

[note]
filename = "{{id}}"
extension = "md"

[format]
markdown.link-format = "wiki"

[tool]
fzf-preview = "bat --color=always {path}"
```

### Pipeline hook (in `internal/research/pipeline.go`)

After Scribe saves notes, call `EnsureZkNotebook` best-effort:

```go
if err := EnsureZkNotebook(researchDir); err != nil {
    log.Warn("zk notebook init failed (non-fatal)", "err", err)
}
```

### Search tier (in `internal/agents/context_builder.go`)

Insert as Tier 2 between semantic and file-scan:

```
Tier 1: Vector semantic (OPENAI/MISTRAL key)
Tier 2: zk list --match (zk binary + JSON output available)   ← NEW
Tier 3: File-scan keyword (always)
```

### `nqb index` tip

After successful indexing, if `ZkAvailable()`:
```
Tip: run `zk list` in .naqb/research/ to browse notes with FTS.
```

---

## Release Gate

Do not ship zk integration until:

- [ ] Upstream JSON output is available (or fork is maintained)
- [ ] Stdin piping works for `zk new`
- [ ] RTL/Arabic rendering is verified
- [ ] `nqb research` + `nqb context` end-to-end test passes with zk active
- [ ] Core output pipeline (collect notes → formatted export) is feature-complete
