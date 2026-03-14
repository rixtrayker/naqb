package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/tui"
)

// ChatCmd returns the `book chat` command.
func ChatCmd() *cobra.Command {
	var chapterNum int

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Open an interactive chat REPL for book editing (Claude Opus)",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			apiKey, err := config.APIKey()
			if err != nil {
				return err
			}
			client := llm.New(apiKey)

			// Build system prompt with book context
			system := buildChatSystem(cfg, chapterNum, bookDir)

			model := cfg.LLM.ChatModel
			if model == "" {
				model = llm.ModelOpus
			}

			fmt.Fprintf(os.Stderr, "Opening chat with %s...\n", model)

			return tui.RunChat(context.Background(), client, model, system)
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Focus on a specific chapter (optional)")
	return cmd
}

func buildChatSystem(cfg *config.BookConfig, chapterNum int, bookDir string) string {
	base := fmt.Sprintf(`You are an expert editor helping to write and refine the book "%s" by %s.
Domain: %s
Language: %s
Synopsis: %s

You have deep knowledge of the book's content and style. Help the author:
- Refine prose and chapter content
- Brainstorm ideas and structure
- Answer questions about consistency
- Suggest improvements

When the author asks you to update a chapter, provide the complete revised content.`,
		cfg.Title, cfg.Author, cfg.Domain, cfg.Language, cfg.Synopsis)

	if chapterNum > 0 {
		title := chapterTitle(cfg, chapterNum)
		base += fmt.Sprintf("\n\nFocus: Chapter %d — %s", chapterNum, title)

		// Try to include chapter content
		chapPath := bookDir + "/chapters/" + config.ChapterFilename(chapterNum)
		if data, err := os.ReadFile(chapPath); err == nil {
			content := string(data)
			if len(content) > 8000 {
				content = content[:8000] + "\n... (truncated)"
			}
			base += "\n\n## Current Chapter Content\n" + content
		}
	}

	return base
}
