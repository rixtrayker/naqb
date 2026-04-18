package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/internal/tui/keys"
)

// DefaultCommandRegistry maps command names (without slash) to their handlers.
// This is wired into BookViewModel by RunBookView unless overridden.
var DefaultCommandRegistry = map[string]CommandHandler{
	"write":    handleWrite,
	"w":        handleWrite,
	"qa":       handleQA,
	"q":        handleQA,
	"export":   handleExport,
	"e":        handleExport,
	"status":   handleStatus,
	"s":        handleStatus,
	"context":  handleContext,
	"preview":  handlePreview,
	"p":        handlePreview,
	"help":     handleHelp,
	"?":        handleHelp,
	"watch":    handleWatch,
	"W":        handleWatch,
	"chat":     handleChat,
	"~":        handleChat,
	"outline":  handleOutline,
	"o":        handleOutline,
	"research": handleResearch,
	"r":        handleResearch,
	"index":    handleIndex,
	"i":        handleIndex,
}

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

func handleExport(_ context.Context, args []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
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
	for _, sec := range keys.BookViewHelpSections {
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
	return "Watch mode — run 'nqb watch' from CLI for daemon mode", nil
}

func handleChat(_ context.Context, _ []string, _ string, cfg *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	model := "claude-opus-4-20250514"
	if cfg != nil && cfg.LLM.WriteModel != "" {
		model = cfg.LLM.WriteModel
	}
	return fmt.Sprintf("Chat — run 'nqb chat' from CLI (model: %s)", model), nil
}

func handleOutline(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return "Outline editor — run 'nqb outline' from CLI", nil
}

func handleResearch(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, chNum int) (string, error) {
	return fmt.Sprintf("Research — run 'nqb research --chapter %d' from CLI\n  Add --deep for Gemini-powered deep search", chNum), nil
}

func handleIndex(_ context.Context, _ []string, _ string, _ *config.BookConfig, _ llm.Provider, _ int) (string, error) {
	return "Index — run 'nqb index' from CLI to index chapters and research notes\n  Add --reindex to force re-indexing", nil
}

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
