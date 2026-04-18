// Package theme provides the centralized color palette and visual primitives
// for all TUI screens.
package theme

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
