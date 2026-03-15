# Testing

## Gate Rule

`go build ./... && go test ./...` must pass after every change before committing.

```bash
go build ./... && go test ./...
```

- Never commit with a failing build or failing tests
- Applies to all changes: features, refactors, docs-only changes that touch `.go` files
