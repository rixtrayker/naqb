# TUI Patterns

## Command Registry

All slash commands are registered in `commandRegistry` in `internal/tui/commands.go`.

```go
var commandRegistry = map[string]CommandHandler{
    "write":    handleWrite,
    "w":        handleWrite,  // always add a 1-letter alias
    "research": handleResearch,
    "r":        handleResearch,
}
```

Checklist when adding a new command:
1. Add full name + 1-letter alias to `commandRegistry`
2. Implement handler as a thin wrapper — logic lives in `agents/`, `research/`, etc.
3. Update `keys.go`: add to `BookViewBindings` (footer), `BookViewHelpSections` (help overlay), and `Palette commands` section
4. Add the key case to `book_view.go` `updateMain()` switch

```go
// Handler: thin wrapper only
func handleResearch(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, chNum int) (string, error) {
    return fmt.Sprintf("Run 'nqb research --chapter %d'", chNum), nil
}
```

## Centralized Keybindings

`internal/tui/keys.go` is the single source of truth for all keyboard bindings.

**Never handle a key in `book_view.go` without documenting it in `keys.go`.**

```go
// keys.go — document the binding
var BookViewBindings = []Binding{
    {Key: "r", Desc: "Research"},
    // ...
}

// book_view.go — handle the key
case "r":
    return m, m.runCommand("/research")
```

`keys.go` drives:
- Footer hint bar via `renderHintBar(BookViewBindings)`
- Help overlay via `RenderHelpOverlay(BookViewHelpSections)`

If you skip updating `keys.go`, the key is invisible to the user.

## Sidebar Tabs

New sidebar tabs are added as:
1. A new constant in the `sidebarTab` iota in `sidebar.go`
2. A `tabNames` entry
3. A `renderXxxTab()` function
4. A case in `renderSidebarContent()`
5. An entry in the `Sidebar tabs` section of `BookViewHelpSections`

Tab order in `tabNames` determines cycle order (Tab/Shift+Tab).
