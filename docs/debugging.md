# Debugging

## Log File

`nqb` writes structured logs to `~/.naqb/nqb.log`.

Every INFO-level event (LLM calls, pipeline stages, vault operations, git commits)
is logged. Errors and warnings always appear.

```bash
# Tail the log live
make log
# or:
tail -f ~/.naqb/nqb.log

# View recent entries
tail -50 ~/.naqb/nqb.log

# Filter by package
grep "writer.go" ~/.naqb/nqb.log
grep "chapter=3" ~/.naqb/nqb.log
grep "ERROR" ~/.naqb/nqb.log
```

---

## Debug Mode

Set `NQB_DEBUG=1` to enable DEBUG-level logging and echo all logs to stderr in real time:

```bash
NQB_DEBUG=1 nqb write --chapter 1
# or:
make debug
```

Debug mode adds:
- Every LLM call with model, token counts, response size
- Context file sizes before LLM calls
- Git commit skip reasons
- Vault registry parse details

---

## Log Format

```
2025-01-15 14:32:01.234 INFO  writer.go:42 write chapter start chapter=3 book=My Book
2025-01-15 14:32:01.235 DEBUG client.go:71 LLM stream start model=claude-sonnet-4-6 max_tokens=8192 messages=1
2025-01-15 14:32:08.901 DEBUG client.go:112 LLM stream done model=claude-sonnet-4-6 total_chars=14823
2025-01-15 14:32:08.902 INFO  writer.go:67 write chapter done chapter=3 path=chapters/ch-03.md chars=14823
```

Fields: `timestamp level file:line message key=value...`

---

## Common Issues

### `nqb init` fails with "planner LLM call failed"
- Check your API key: `nqb config`
- Verify `ANTHROPIC_API_KEY` is set or run `nqb config --set-key`
- Check `~/.naqb/nqb.log` for the full error

### `nqb export --format pdf` fails
- Verify pandoc is installed: `pandoc --version`
- Verify xelatex is installed: `xelatex --version`
- For Arabic PDFs: verify Amiri font is installed (`fc-list | grep Amiri`)
- Run with `NQB_DEBUG=1` to see the exact pandoc command

### TUI is blank or garbled
- Your terminal may not support 256 colors — try `TERM=xterm-256color nqb`
- Minimum terminal width: ~80 columns

### `context file not found` warning during write
- This is non-fatal: `nqb write` builds context on-the-fly if missing
- Run `nqb context --chapter N` explicitly first for best results

### Vault shows no projects
- Check `~/.naqb/vault.yaml` exists and has the correct vault paths
- Run `nqb vault list` to see registered vaults
- Ensure each project directory contains a `book.yaml`

---

## Clearing the Log

```bash
make log-clear
# or:
> ~/.naqb/nqb.log
```

## Reporting a Bug

Include in your issue:
1. `nqb` version (git SHA for now)
2. OS + Go version (`go version`)
3. The command you ran
4. Contents of `~/.naqb/nqb.log` with `NQB_DEBUG=1`
