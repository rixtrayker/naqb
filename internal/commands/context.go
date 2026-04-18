package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/keycheck"
)

// ContextCmd returns the `book context` command.
func ContextCmd() *cobra.Command {
	var chapterNum int
	var all bool

	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Build context file(s) for one or all chapters",
		Long: `Assemble the "golden prompt" context file for a chapter.

Gathers the book outline, adjacent chapter summaries, research notes, and
style constraints into a single contexts/ch-XX-context.md file. This file
is used by the write and fix stages to produce coherent, well-informed drafts.

Can run in deterministic mode without an LLM key.`,
		Example: `  nqb context --chapter 3
  nqb context --all
  nqb ctx -c 5`,
		GroupID: "writing",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Soft preflight: context can work in deterministic mode without LLM
			if pf := keycheck.CheckCommand("context"); !pf.OK {
				fmt.Fprintf(os.Stderr, "Note: no LLM key found; context will use deterministic mode only.\n")
			}
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
