package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
)

// ContextCmd returns the `book context` command.
func ContextCmd() *cobra.Command {
	var chapterNum int
	var all bool

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Build context file(s) for one or all chapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			if all {
				for _, ch := range cfg.Chapters {
					if err := buildContext(bookDir, cfg, ch.Number); err != nil {
						fmt.Printf("  ✗ Chapter %d: %v\n", ch.Number, err)
					}
				}
				return nil
			}

			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N or --all")
			}
			return buildContext(bookDir, cfg, chapterNum)
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Build context for all chapters")
	return cmd
}

func buildContext(bookDir string, cfg *config.BookConfig, n int) error {
	path, err := agents.WriteContextFile(bookDir, cfg, n)
	if err != nil {
		return err
	}
	fmt.Printf("  ✓ Chapter %d context → %s\n", n, path)
	return nil
}
