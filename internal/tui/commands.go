package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
)

// CommandHandler is a function that handles a parsed slash command.
// Returns (output string, error). Output is displayed in the TUI output pane.
type CommandHandler func(ctx context.Context, args []string, bookDir string, cfg *config.BookConfig, client llm.Provider, chNum int) (string, error)

// commandRegistry maps command names (without slash) to their handlers.
// Aliases map to the same handler.
var commandRegistry = map[string]CommandHandler{
	"write":   handleWrite,
	"w":       handleWrite,
	"qa":      handleQA,
	"q":       handleQA,
	"export":  handleExport,
	"e":       handleExport,
	"status":  handleStatus,
	"s":       handleStatus,
	"context": handleContext,
	"preview": handlePreview,
	"p":       handlePreview,
	"help":    handleHelp,
	"?":       handleHelp,
	"watch":   handleWatch,
	"W":       handleWatch,
	"chat":    handleChat,
	"~":       handleChat,
	"outline": handleOutline,
	"o":       handleOutline,
}

// dispatchCommand parses a slash command string and runs the matching handler.
func dispatchCommand(input, bookDir string, cfg *config.BookConfig, client llm.Provider, defaultChapter int) (string, error) {
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
				fmt.Sscanf(args[i+1], "%d", &chNum)
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
	_ = format
	remaining = append([]string{"--format=" + format}, remaining...)

	handler, ok := commandRegistry[name]
	if !ok {
		return "", fmt.Errorf("unknown command: /%s  (try /help)", name)
	}
	return handler(context.Background(), remaining, bookDir, cfg, client, chNum)
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func handleWrite(ctx context.Context, args []string, bookDir string, cfg *config.BookConfig, client llm.Provider, chNum int) (string, error) {
	path, err := agents.WriteContextFile(bookDir, cfg, chNum)
	if err != nil {
		return "", fmt.Errorf("context: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Context → %s\n", path))
	chPath, err := agents.WriteChapter(ctx, client, bookDir, cfg, chNum, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		return sb.String(), fmt.Errorf("write: %w", err)
	}
	sb.WriteString(fmt.Sprintf("\n\nChapter → %s", chPath))
	return sb.String(), nil
}

func handleQA(ctx context.Context, _ []string, bookDir string, cfg *config.BookConfig, client llm.Provider, chNum int) (string, error) {
	result, err := agents.RunQA(ctx, client, bookDir, cfg, chNum)
	if err != nil {
		return "", err
	}
	_ = agents.WriteQAReport(bookDir, result)
	if result.Passed {
		return "QA passed: " + result.DeterministicMsg, nil
	}
	return "QA issues:\n" + strings.Join(result.Issues, "\n"), nil
}

func handleExport(_ context.Context, args []string, bookDir string, cfg *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	format := "pdf"
	for _, a := range args {
		if strings.HasPrefix(a, "--format=") {
			format = strings.TrimPrefix(a, "--format=")
		}
	}
	return fmt.Sprintf("Export %s — run 'nqb export --format %s' from CLI for full export", strings.ToUpper(format), format), nil
}

func handleStatus(_ context.Context, _ []string, bookDir string, cfg *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return buildStatusText(bookDir, cfg), nil
}

func handleContext(_ context.Context, _ []string, bookDir string, cfg *config.BookConfig, _ llm.Provider, chNum int) (string, error) {
	path, err := agents.WriteContextFile(bookDir, cfg, chNum)
	if err != nil {
		return "", err
	}
	return "Context → " + path, nil
}

func handlePreview(_ context.Context, _ []string, bookDir string, cfg *config.BookConfig, _ llm.Provider, chNum int) (string, error) {
	return renderPreview(bookDir, cfg, chNum)
}

func handleHelp(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	var sb strings.Builder
	sb.WriteString("Palette commands:\n")
	for _, sec := range BookViewHelpSections {
		if sec.Title == "Palette commands" {
			for _, b := range sec.Bindings {
				sb.WriteString(fmt.Sprintf("  %-30s  %s\n", b.Key, b.Desc))
			}
		}
	}
	sb.WriteString("\nPress [?] for full keybinding reference.\n")
	return sb.String(), nil
}

func handleWatch(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return "(Watch mode — use 'nqb watch' from CLI for daemon mode)", nil
}

func handleChat(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return "(Opening chat — use 'nqb chat' from CLI)", nil
}

func handleOutline(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return "(Opening outline editor — use 'nqb outline' from CLI)", nil
}

// ── View helpers (kept here to co-locate with handlers) ───────────────────────

func buildStatusText(bookDir string, cfg *config.BookConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📚 %s\n", cfg.Title))
	for _, ch := range cfg.Chapters {
		icon := "○"
		chapPath := filepath.Join(bookDir, "chapters", ch.File)
		if _, err := os.Stat(chapPath); err == nil {
			icon = "●"
		}
		sb.WriteString(fmt.Sprintf("  %s Ch%02d: %s\n", icon, ch.Number, ch.Title))
	}
	return sb.String()
}
