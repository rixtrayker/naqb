# Commit Protocol

**Run before every commit — no exceptions:**

```bash
make check    # go build + go vet + go test
```

Never commit with:
- A compilation error
- A failing `go vet` warning
- A failing test
- Missing documentation updates (see Documentation Rule standard)

Doc-only commits still require `go build ./...` to pass.
