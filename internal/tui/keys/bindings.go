// Package keys provides all canonical keybindings for the TUI.
//
// Key bindings are defined centrally here and reused across every model so
// that the rendered hints always stay in sync with the actual handlers.
package keys

import "github.com/amr/naqb/internal/tui/theme"

// HomeBindings are the keybindings shown in the home screen footer.
var HomeBindings = []theme.Binding{
	{Key: "↑/↓", Desc: "Navigate"},
	{Key: "Enter", Desc: "Open"},
	{Key: "n", Desc: "New book"},
	{Key: "v", Desc: "Vaults"},
	{Key: "q", Desc: "Quit"},
}

// HomeSearchHint is the search-bar hint line.
var HomeSearchHint = []theme.Binding{
	{Key: "type", Desc: "Filter by title / lang / domain"},
	{Key: "Esc", Desc: "Clear search"},
}

// BookViewBindings are the sidebar keybindings shown in the book TUI footer.
var BookViewBindings = []theme.Binding{
	{Key: "↑/↓", Desc: "Chapter"},
	{Key: "Tab", Desc: "Sidebar tab"},
	{Key: "/", Desc: "Palette"},
	{Key: "w", Desc: "Write"},
	{Key: "q", Desc: "QA"},
	{Key: "r", Desc: "Research"},
	{Key: "e", Desc: "Export"},
	{Key: "p", Desc: "Preview"},
	{Key: "~", Desc: "Chat"},
	{Key: "?", Desc: "Help"},
	{Key: "Ctrl+C", Desc: "Back"},
}

// BookViewPaletteBindings are shown inside the command palette.
var BookViewPaletteBindings = []theme.Binding{
	{Key: "Enter", Desc: "Run"},
	{Key: "Esc", Desc: "Cancel"},
	{Key: "Tab", Desc: "Complete"},
}

// OutlineEditorBindings are the keybindings shown in the outline editor footer.
var OutlineEditorBindings = []theme.Binding{
	{Key: "↑/↓", Desc: "Navigate"},
	{Key: "t/Enter", Desc: "Edit title"},
	{Key: "s", Desc: "Edit summary"},
	{Key: "U", Desc: "Move up"},
	{Key: "D", Desc: "Move down"},
	{Key: "Ctrl+S", Desc: "Save"},
	{Key: "q/Esc", Desc: "Back"},
}

// PreviewBindings are the keybindings shown in the preview footer.
var PreviewBindings = []theme.Binding{
	{Key: "↑/↓", Desc: "Scroll"},
	{Key: "PgUp/PgDn", Desc: "Page"},
	{Key: "g", Desc: "Top"},
	{Key: "G", Desc: "Bottom"},
	{Key: "q/Esc", Desc: "Back"},
}

// ChatBindings are the keybindings shown in the chat REPL footer.
var ChatBindings = []theme.Binding{
	{Key: "Enter", Desc: "Send"},
	{Key: "Alt+Enter", Desc: "Newline"},
	{Key: "↑/↓", Desc: "Scroll history"},
	{Key: "Ctrl+C", Desc: "Quit"},
}

// AgentChatBindings are the keybindings for the interactive agent chat.
var AgentChatBindings = []theme.Binding{
	{Key: "Enter", Desc: "Send"},
	{Key: "Alt+Enter", Desc: "Newline"},
	{Key: "Ctrl+T", Desc: "Tasks"},
	{Key: "↑/↓", Desc: "Scroll"},
	{Key: "Ctrl+C", Desc: "Quit"},
}

// InitFormBindings are the keybindings shown in the init form footer.
var InitFormBindings = []theme.Binding{
	{Key: "Enter", Desc: "Confirm"},
	{Key: "Ctrl+C", Desc: "Cancel"},
}

// ImportFormBindings are the keybindings shown in the import wizard footer.
var ImportFormBindings = []theme.Binding{
	{Key: "Enter", Desc: "Confirm"},
	{Key: "Ctrl+C", Desc: "Cancel"},
}
