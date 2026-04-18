---
name: naqb-build-test
description: "Run build, test, and coverage checks for the naqb (نقب) project. Use when compiling the Go binary, running tests, checking coverage targets, or verifying code quality before commits."
user-invocable: true
license: MIT
compatibility: Designed for Claude Code or similar AI coding agents working on the github.com/amr/naqb project.
metadata:
  author: naqb
  version: "1.0.0"
allowed-tools: Read Edit Write Glob Grep Bash(go:*) Bash(make:*) Agent
---

**Persona:** You are a build engineer for the naqb project. You ensure every change compiles, passes vet, meets coverage targets, and follows the commit protocol.

## Quick Commands

| Task | Command |
|------|---------|
| Full check (build + vet + test) | `make check` |
| Build only | `go build -o bin/nqb ./cmd/nqb` |
| Run all tests | `go test ./...` |
| Run tests with race | `go test -race ./...` |
| Text coverage report | `make cover-text` |
| HTML coverage report | `make cover` |

## Coverage Targets

- `internal/db` ≥ 70%
- `internal/jobs` ≥ 60%
- `internal/agent` ≥ 55%
- `internal/search` ≥ 45%
- `internal/wordcount` ≥ 80%
- `internal/vault` ≥ 60%

## Pre-Commit Protocol

**Always run `make check` before suggesting a commit.**

Never commit with:
- Compilation errors
- Failing `go vet` warnings
- Failing tests
- Missing documentation updates

## Test Patterns

- New package → new `_test.go` file in the same package
- New exported function → at least one test (happy path + one error case)
- Bug fix → add a test that would have caught the bug
- All file I/O tests use `t.TempDir()` — never touch real user directories
- Skip network/LLM tests when credentials not set: `t.Skip("ANTHROPIC_API_KEY not set")`
