# Vault System

`nqb` organizes books into **vaults** — directories that contain book projects.
The concept is similar to Obsidian vaults: one directory, many books, managed from one place.

---

## Directory Layout

| Path | Purpose |
|------|---------|
| `~/.naqb/` | Global config directory |
| `~/.naqb/config.yaml` | API key and global settings |
| `~/.naqb/vault.yaml` | Vault registry + recent projects |
| `~/.naqb/projects/` | **Default vault** — books created with `--vault` go here |
| `~/.naqb/nqb.log` | Application log |
| Any directory | Can be registered as a vault with `nqb vault add` |

---

## Default Vault

Books created with `nqb init --vault` go into `~/.naqb/projects/<slugified-title>/`.

```bash
nqb init --vault        # creates ~/.naqb/projects/my-book-title/
nqb open my-book-title  # opens by name
```

---

## Custom Vaults

Register any directory as a named vault:

```bash
# Register ~/work/books as vault "work"
nqb vault add work ~/work/books

# List all vaults
nqb vault list

# Remove a vault (does NOT delete files)
nqb vault remove work
```

`nqb` scans all registered vaults when you open the home screen or run `nqb open`.

---

## The Registry File

`~/.naqb/vault.yaml` example:

```yaml
vaults:
  - name: default
    path: /Users/you/.naqb/projects
  - name: work
    path: /Users/you/work/books

recents:
  - name: My Arabic History Book
    path: /Users/you/.naqb/projects/my-arabic-history-book
    language: ar
    opened_at: 2025-01-15T14:00:00Z
```

---

## Opening Books

```bash
nqb                      # TUI home screen — browse all vaults
nqb open my-book         # open by project name (searches all vaults)
nqb open ~/path/to/book  # open by filesystem path
nqb .                    # open current directory as a book
```

`nqb .` is useful when you `cd` into a book directory directly:

```bash
cd ~/work/books/my-cs-book
nqb .     # opens TUI if book.yaml found, asks to init if not
```

---

## Project Discovery

`ListProjects()` scans all vault directories for subdirectories containing `book.yaml`.
It loads metadata (title, language, domain, chapter count, written count) from each `book.yaml`
and sorts results by `book.yaml` modification time — most recently edited first.

Projects without a `book.yaml` are silently ignored.

---

## Recent Projects

`nqb` records the last 20 opened projects in `vault.yaml`.
The home screen shows projects sorted by modification time, not recency —
recents are available for future "recently opened" UI features.
