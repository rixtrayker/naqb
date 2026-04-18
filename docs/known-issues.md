# Known Issues

All issues from the 2026-03-20 codebase audit have been resolved.

## Resolved (2026-03-20) — Top 5 (High Impact)

| # | File | Fix |
|---|------|-----|
| 1 | `tui/chat.go` | Chat stream deadlock — replaced single-read with channel-drain pattern |
| 2 | `tui/preview.go:168` | Preview type assertion panic — comma-ok pattern |
| 3 | `db/db.go` | Parallel DAG SQLite locking — added `PRAGMA busy_timeout=5000` |
| 4 | `commands/batch.go` | Partial batch enqueue returns success — now tracks failures |
| 5 | `commands/export.go` | Partial export returns success — now tracks failures |

## Resolved (2026-03-20) — Remaining 16

| # | File | Fix |
|---|------|-----|
| 1 | `store/hybrid.go` | Single-source search errors — now logged with `log.Warn` |
| 2 | `store/vector/chroma.go` | Unguarded Documents/Distances — added length validation |
| 3 | `tui/book_view.go` | Missing nil guard on `m.cfg` — added nil check in View |
| 4 | `searchutil/searchutil.go` | `JaccardSimilarity(empty, empty)` — now returns 1.0 |
| 5 | `rerank/rerank.go` | Threshold degradation duplicates — explicit `ranked[:0]` reset |
| 6 | `knowledge/ingestion.go` | `strings.Index` wrong for duplicates — now tracks running offset |
| 7 | `agent/agent.go:117-119` | OnStepFinish DB errors discarded — now logged with `log.Warn` |
| 8 | `agent/agent.go:123-125` | OnError doesn't persist — now writes error message to DB |
| 9 | `agent/tools.go:117-121` | WriteFile backup failure ignored — now returns error |
| 10 | `tui/home.go` | Project load error hidden — now shows error message in UI |
| 11 | `store/util/merge.go` + `store/hybrid.go` | MergeBySignature keeps first — now keeps highest score |
| 12 | `agent/agent.go:72` | UNIQUE detection via substring — now case-insensitive, checks both UNIQUE and CONSTRAINT |
| 13 | `pipeline/pipeline.go` | Stage progress load silent fallback — now returns error |
| 14 | `pipeline/executor.go` | Unknown stage type silent no-op — now returns error with stage type |
| 15 | `tui/outline_editor.go` | Save message disappears — added statusTicks decay (persists ~10 key events) |
| 16 | `commands/qa.go` | No distinction disabled vs unavailable — now shows specific reason |
