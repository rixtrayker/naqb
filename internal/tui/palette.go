package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// PaletteHandler handles a parsed slash command from the book view palette.
// Returns (output string, error). Output is displayed in the TUI output pane.
type PaletteHandler interface {
	// Dispatch parses and runs a slash command.
	Dispatch(ctx context.Context, input string, bookDir string, cfg *config.BookConfig, client llm.Provider, defaultChapter int) (string, error)
}

// DefaultPalette is the built-in palette handler that routes to command handlers.
type DefaultPalette struct {
	registry map[string]CommandHandler
}

// CommandHandler is a function that handles a parsed slash command.
type CommandHandler func(ctx context.Context, args []string, bookDir string, cfg *config.BookConfig, client llm.Provider, chNum int) (string, error)

// NewDefaultPalette creates a palette with the built-in command registry.
func NewDefaultPalette(registry map[string]CommandHandler) *DefaultPalette {
	return &DefaultPalette{registry: registry}
}

// Dispatch parses a slash command string and runs the matching handler.
func (p *DefaultPalette) Dispatch(ctx context.Context, input, bookDir string, cfg *config.BookConfig, client llm.Provider, defaultChapter int) (string, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	name := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	// Resolve --chapter / -c flag from args; pass remaining args to handler.
	chNum := defaultChapter
	format := "pdf"
	var remaining []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--chapter", "-c":
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &chNum)
				i++
			}
		case "--format", "-f":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		default:
			remaining = append(remaining, args[i])
		}
	}
	// Inject resolved format into remaining so handlers can read it.
	remaining = append([]string{"--format=" + format}, remaining...)

	handler, ok := p.registry[name]
	if !ok {
		return "", fmt.Errorf("unknown command: /%s  (try /help)", name)
	}
	return handler(ctx, remaining, bookDir, cfg, client, chNum)
}
