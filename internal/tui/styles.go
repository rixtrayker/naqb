package tui

import "github.com/charmbracelet/lipgloss"

// ── Brand colors ─────────────────────────────────────────────────────────────

var (
	ColorPrimary   = lipgloss.Color("213") // pink/magenta
	ColorSecondary = lipgloss.Color("99")  // purple
	ColorAccent    = lipgloss.Color("86")  // cyan/green
	ColorDim       = lipgloss.Color("241")
	ColorBorder    = lipgloss.Color("238")
	ColorBg        = lipgloss.Color("235")
)

// ── Brand header ──────────────────────────────────────────────────────────────

var (
	BrandStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	VersionStyle  = lipgloss.NewStyle().Faint(true).Foreground(ColorSecondary)
	SubtitleStyle = lipgloss.NewStyle().Faint(true).Foreground(ColorDim)
)

// ── Panels ───────────────────────────────────────────────────────────────────

var (
	PanelStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder)
	ActivePanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorSecondary)
)

// ── Status bar (pinned bottom) ───────────────────────────────────────────────

var (
	StatusBarStyle = lipgloss.NewStyle().Background(ColorBg).Padding(0, 1)
	StatusKeyStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	StatusSepStyle = lipgloss.NewStyle().Faint(true)
)

// ── Chapter status pills ──────────────────────────────────────────────────────

var (
	PillWritten  = lipgloss.NewStyle().Background(lipgloss.Color("22")).Foreground(lipgloss.Color("82")).Padding(0, 1)
	PillPending  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("243")).Padding(0, 1)
	PillQAFail   = lipgloss.NewStyle().Background(lipgloss.Color("52")).Foreground(lipgloss.Color("9")).Padding(0, 1)
	PillImported = lipgloss.NewStyle().Background(lipgloss.Color("58")).Foreground(lipgloss.Color("214")).Padding(0, 1)
)

// ── Language badges ───────────────────────────────────────────────────────────

var (
	BadgeAR = lipgloss.NewStyle().Background(lipgloss.Color("214")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	BadgeEN = lipgloss.NewStyle().Background(lipgloss.Color("86")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
)

// ── renderStatusBar renders a pinned bottom bar with key hint pairs ───────────

// renderStatusBar renders a pinned-bottom status bar of width w.
// It reuses the Binding type from keys.go.
func renderStatusBar(bindings []Binding, w int) string {
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
