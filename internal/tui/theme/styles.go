package theme

import "github.com/charmbracelet/lipgloss"

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
