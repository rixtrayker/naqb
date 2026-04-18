// Package tui provides all Bubble Tea UI components for nqb.
//
// The package is organized into sub-packages by concern:
//   - theme/   colors, styles, and shared visual primitives
//   - keys/    canonical keybindings and help overlays
//   - components/ reusable widgets (spinner, tracker, chat, wizard)
//
// Screen implementations live at the package root and are wired together
// by the public API in this file.
package tui

import (
	"context"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// SetPaletteHandler injects the palette dispatcher into a BookViewModel.
// This is called by the application layer before running the TUI.
func (m *BookViewModel) SetPaletteHandler(h PaletteHandler) {
	m.palette = h
}

// EnsurePalette configures a default palette on the model if none is set.
func (m *BookViewModel) EnsurePalette(registry map[string]CommandHandler) {
	if m.palette == nil {
		m.palette = NewDefaultPalette(registry)
	}
}

// Public re-exports for backward compatibility with commands package.
// These types are referenced by internal/commands/open.go and others.

// ChapterStatus mirrors the old config.Chapter usage in views.
type ChapterStatus = config.Chapter

// LLMProvider re-exports llm.Provider for callers that previously
// imported it through the tui package indirectly.
type LLMProvider = llm.Provider

// BookConfig re-exports config.BookConfig.
type BookConfig = config.BookConfig

// Context is a re-export for backward compatibility.
type Context = context.Context
