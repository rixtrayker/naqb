# Documentation Rule

**Every code change that affects behavior must update docs:**

| Change type | Required doc update |
|---|---|
| New/changed command flag | `README.md` usage section |
| New/changed function signature | Docstring on the function |
| New package or major feature | `agent-os/standards/` relevant file |
| Architecture or key file paths | `docs/architecture.md` or `docs/modular-architecture.md` |
| New dependency | `go.mod` comment + relevant doc |
| Breaking change to any public API | `README.md` + relevant standard |
