package theme

import "github.com/charmbracelet/lipgloss"

// Binding is one key → description pair.
type Binding struct {
	Key  string
	Desc string
}

// RenderStatusBar renders a pinned-bottom status bar of width w.
func RenderStatusBar(bindings []Binding, w int) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := StatusKeyStyle.Render("[" + b.Key + "]")
		desc := lipgloss.NewStyle().Faint(true).Render(" " + b.Desc)
		parts = append(parts, key+desc)
	}
	var out string
	for i, p := range parts {
		if i > 0 {
			out += StatusSepStyle.Render("  ·  ")
		}
		out += p
	}
	return StatusBarStyle.Width(w).Render(out)
}

// RenderHintBar renders a compact one-line hint bar from a slice of bindings.
// e.g.  [Enter] Open  [n] New  [/] Search  [q] Quit
func RenderHintBar(bindings []Binding) string {
	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))
	descStyle := lipgloss.NewStyle().
		Faint(true)
	sepStyle := lipgloss.NewStyle().
		Faint(true).
		PaddingLeft(1).
		PaddingRight(1)
	groupStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := keyStyle.Render("[" + b.Key + "]")
		desc := descStyle.Render(" " + b.Desc)
		parts = append(parts, key+desc)
	}
	var out string
	for i, p := range parts {
		if i > 0 {
			out += sepStyle.Render("·")
		}
		out += p
	}
	return groupStyle.Render(out)
}
