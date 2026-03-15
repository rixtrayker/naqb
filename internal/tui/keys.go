// Package tui provides all Bubble Tea UI components for nqb.
//
// Key bindings are defined centrally in this file and reused across every
// model so that the rendered hints always stay in sync with the actual
// handlers.
package tui

import "github.com/charmbracelet/lipgloss"

// ── Shared hint renderer ─────────────────────────────────────────────────────

var (
	hintKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	hintDescStyle = lipgloss.NewStyle().
			Faint(true)

	hintSepStyle = lipgloss.NewStyle().
			Faint(true).
			PaddingLeft(1).
			PaddingRight(1)

	hintGroupStyle = lipgloss.NewStyle().
			PaddingLeft(2)
)

// Binding is one key → description pair.
type Binding struct {
	Key  string
	Desc string
}

// renderHintBar renders a compact one-line hint bar from a slice of bindings.
// e.g.  [Enter] Open  [n] New  [/] Search  [q] Quit
func renderHintBar(bindings []Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := hintKeyStyle.Render("[" + b.Key + "]")
		desc := hintDescStyle.Render(" " + b.Desc)
		parts = append(parts, key+desc)
	}
	var out string
	for i, p := range parts {
		if i > 0 {
			out += hintSepStyle.Render("·")
		}
		out += p
	}
	return hintGroupStyle.Render(out)
}

// ── Per-screen canonical binding sets ────────────────────────────────────────

// HomeBindings are the keybindings shown in the home screen footer.
var HomeBindings = []Binding{
	{Key: "↑/↓", Desc: "Navigate"},
	{Key: "Enter", Desc: "Open"},
	{Key: "n", Desc: "New book"},
	{Key: "v", Desc: "Vaults"},
	{Key: "q", Desc: "Quit"},
}

// HomeSearchHint is the search-bar hint line.
var HomeSearchHint = []Binding{
	{Key: "type", Desc: "Filter by title / lang / domain"},
	{Key: "Esc", Desc: "Clear search"},
}

// BookViewBindings are the sidebar keybindings shown in the book TUI footer.
var BookViewBindings = []Binding{
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

// BookViewPaletteHint is shown inside the command palette.
var BookViewPaletteBindings = []Binding{
	{Key: "Enter", Desc: "Run"},
	{Key: "Esc", Desc: "Cancel"},
	{Key: "Tab", Desc: "Complete"},
}

// BookViewHelpSections is the full help overlay content.
var BookViewHelpSections = []HelpSection{
	{
		Title: "Navigation",
		Bindings: []Binding{
			{Key: "↑ / k", Desc: "Select previous chapter"},
			{Key: "↓ / j", Desc: "Select next chapter"},
			{Key: "Enter", Desc: "Open selected chapter detail"},
		},
	},
	{
		Title: "Actions (selected chapter)",
		Bindings: []Binding{
			{Key: "w", Desc: "Write chapter      (/write)"},
			{Key: "q", Desc: "Run QA             (/qa)"},
			{Key: "r", Desc: "Run research       (/research)"},
			{Key: "p", Desc: "Preview chapter    (/preview)"},
			{Key: "e", Desc: "Export book        (/export)"},
			{Key: "~", Desc: "Chat with Opus     (/chat)"},
		},
	},
	{
		Title: "Sidebar tabs",
		Bindings: []Binding{
			{Key: "Tab", Desc: "Cycle tab forward  (Actions→Notes→Todos→Stats→QA→Git)"},
			{Key: "Shift+Tab", Desc: "Cycle tab backward"},
		},
	},
	{
		Title: "Views",
		Bindings: []Binding{
			{Key: "o", Desc: "Outline editor  (/outline)"},
			{Key: "s", Desc: "Status summary  (/status)"},
			{Key: "W", Desc: "Watch mode      (/watch)"},
		},
	},
	{
		Title: "Command palette",
		Bindings: []Binding{
			{Key: "/", Desc: "Open palette"},
			{Key: "Enter", Desc: "Run command"},
			{Key: "Esc", Desc: "Close palette"},
		},
	},
	{
		Title: "Palette commands",
		Bindings: []Binding{
			{Key: "/write --chapter N", Desc: "Write chapter N"},
			{Key: "/qa --chapter N", Desc: "QA chapter N"},
			{Key: "/export --format pdf", Desc: "Export as PDF"},
			{Key: "/preview --chapter N", Desc: "Preview chapter N"},
			{Key: "/context --chapter N", Desc: "Build context file"},
			{Key: "/pipeline --chapter N", Desc: "Full pipeline (ctx+write+qa+conflict+gap)"},
			{Key: "/research --chapter N", Desc: "Run research pipeline for chapter N"},
			{Key: "/index", Desc: "Re-index all chapters + notes into vector store"},
			{Key: "/status", Desc: "Show chapter status"},
			{Key: "/help", Desc: "Show this help"},
		},
	},
	{
		Title: "General",
		Bindings: []Binding{
			{Key: "?", Desc: "Toggle this help overlay"},
			{Key: "Ctrl+C", Desc: "Go back / quit"},
		},
	},
}

// OutlineEditorBindings are the keybindings shown in the outline editor footer.
var OutlineEditorBindings = []Binding{
	{Key: "↑/↓", Desc: "Navigate"},
	{Key: "t/Enter", Desc: "Edit title"},
	{Key: "s", Desc: "Edit summary"},
	{Key: "U", Desc: "Move up"},
	{Key: "D", Desc: "Move down"},
	{Key: "Ctrl+S", Desc: "Save"},
	{Key: "q/Esc", Desc: "Back"},
}

// PreviewBindings are the keybindings shown in the preview footer.
var PreviewBindings = []Binding{
	{Key: "↑/↓", Desc: "Scroll"},
	{Key: "PgUp/PgDn", Desc: "Page"},
	{Key: "g", Desc: "Top"},
	{Key: "G", Desc: "Bottom"},
	{Key: "q/Esc", Desc: "Back"},
}

// ChatBindings are the keybindings shown in the chat REPL footer.
var ChatBindings = []Binding{
	{Key: "Enter", Desc: "Send"},
	{Key: "Alt+Enter", Desc: "Newline"},
	{Key: "↑/↓", Desc: "Scroll history"},
	{Key: "Ctrl+C", Desc: "Quit"},
}

// InitFormBindings are the keybindings shown in the init form footer.
var InitFormBindings = []Binding{
	{Key: "Enter", Desc: "Confirm"},
	{Key: "Ctrl+C", Desc: "Cancel"},
}

// ImportFormBindings are the keybindings shown in the import wizard footer.
var ImportFormBindings = []Binding{
	{Key: "Enter", Desc: "Confirm"},
	{Key: "Ctrl+C", Desc: "Cancel"},
}

// ── Help overlay ─────────────────────────────────────────────────────────────

// HelpSection groups related bindings under a heading.
type HelpSection struct {
	Title    string
	Bindings []Binding
}

var (
	helpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("99")).
				Padding(1, 3).
				Background(lipgloss.Color("235"))

	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			PaddingBottom(1)

	helpSectionTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212")).
				PaddingTop(1)

	helpKeyCol = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Width(28)

	helpDescCol = lipgloss.NewStyle().
			Faint(true)

	helpCloseHint = lipgloss.NewStyle().
			Faint(true).
			PaddingTop(1)
)

// RenderHelpOverlay renders the full help overlay for the book TUI.
func RenderHelpOverlay(sections []HelpSection) string {
	var sb string
	sb += helpTitleStyle.Render("نقب  Keyboard Reference") + "\n"

	for _, sec := range sections {
		sb += helpSectionTitle.Render(sec.Title) + "\n"
		for _, b := range sec.Bindings {
			sb += helpKeyCol.Render("  "+b.Key) + helpDescCol.Render(b.Desc) + "\n"
		}
	}
	sb += helpCloseHint.Render("\n  Press [?] or [Esc] to close")
	return helpOverlayStyle.Render(sb)
}
