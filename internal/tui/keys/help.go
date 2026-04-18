package keys

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/amr/naqb/internal/tui/theme"
)

// HelpSection groups related bindings under a heading.
type HelpSection struct {
	Title    string
	Bindings []theme.Binding
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
	var sb strings.Builder
	sb.WriteString(helpTitleStyle.Render("نقب  Keyboard Reference") + "\n")

	for _, sec := range sections {
		sb.WriteString(helpSectionTitle.Render(sec.Title) + "\n")
		for _, b := range sec.Bindings {
			sb.WriteString(helpKeyCol.Render("  "+b.Key) + helpDescCol.Render(b.Desc) + "\n")
		}
	}
	sb.WriteString(helpCloseHint.Render("\n  Press [?] or [Esc] to close"))
	return helpOverlayStyle.Render(sb.String())
}

// BookViewHelpSections is the full help overlay content for the book view.
var BookViewHelpSections = []HelpSection{
	{
		Title: "Navigation",
		Bindings: []theme.Binding{
			{Key: "↑ / k", Desc: "Select previous chapter"},
			{Key: "↓ / j", Desc: "Select next chapter"},
			{Key: "Enter", Desc: "Open selected chapter detail"},
		},
	},
	{
		Title: "Actions (selected chapter)",
		Bindings: []theme.Binding{
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
		Bindings: []theme.Binding{
			{Key: "Tab", Desc: "Cycle tab forward  (Actions→Notes→Todos→Stats→QA→Git)"},
			{Key: "Shift+Tab", Desc: "Cycle tab backward"},
		},
	},
	{
		Title: "Views",
		Bindings: []theme.Binding{
			{Key: "o", Desc: "Outline editor  (/outline)"},
			{Key: "s", Desc: "Status summary  (/status)"},
			{Key: "W", Desc: "Watch mode      (/watch)"},
		},
	},
	{
		Title: "Command palette",
		Bindings: []theme.Binding{
			{Key: "/", Desc: "Open palette"},
			{Key: "Enter", Desc: "Run command"},
			{Key: "Esc", Desc: "Close palette"},
		},
	},
	{
		Title: "Palette commands",
		Bindings: []theme.Binding{
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
		Bindings: []theme.Binding{
			{Key: "?", Desc: "Toggle this help overlay"},
			{Key: "Ctrl+C", Desc: "Go back / quit"},
		},
	},
}

// AgentChatHelpSections is the full help content for the agent chat.
var AgentChatHelpSections = []HelpSection{
	{
		Title: "Chat",
		Bindings: []theme.Binding{
			{Key: "Enter", Desc: "Send message to the agent"},
			{Key: "Alt+Enter", Desc: "Insert a newline in the input"},
			{Key: "↑ / ↓", Desc: "Scroll chat history"},
		},
	},
	{
		Title: "Background Tasks",
		Bindings: []theme.Binding{
			{Key: "Ctrl+T", Desc: "Toggle task panel (shows running/completed tasks)"},
		},
	},
	{
		Title: "General",
		Bindings: []theme.Binding{
			{Key: "Ctrl+C", Desc: "Quit agent chat"},
		},
	},
}
