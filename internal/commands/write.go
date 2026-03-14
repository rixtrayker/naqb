package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/tui"
)

// WriteCmd returns the `book write` command.
func WriteCmd() *cobra.Command {
	var chapterNum int
	var stream bool
	var providerFlag string

	cmd := &cobra.Command{
		Use:   "write",
		Short: "Write a chapter using Claude Sonnet (with spinner)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N")
			}

			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			client, err := providerFor(providerFlag, cfg.LLM.WriteProvider)
			if err != nil {
				return err
			}

			if stream {
				// Streaming mode: print tokens as they arrive
				fmt.Printf("Writing chapter %d (streaming)...\n\n", chapterNum)
				path, err := agents.WriteChapter(context.Background(), client, bookDir, cfg, chapterNum, func(delta string) error {
					fmt.Print(delta)
					return nil
				})
				fmt.Println()
				if err != nil {
					return err
				}
				fmt.Printf("\n✓ Chapter written → %s\n", path)
				return nil
			}

			// Spinner mode (default)
			label := fmt.Sprintf("Writing Chapter %d: %s", chapterNum, chapterTitle(cfg, chapterNum))
			var path string
			err = tui.RunWithSpinner(label, func() error {
				var writeErr error
				path, writeErr = agents.WriteChapter(context.Background(), client, bookDir, cfg, chapterNum, nil)
				return writeErr
			}, os.Stdout)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Chapter written → %s\n", path)
			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&stream, "stream", "s", false, "Stream output to terminal instead of spinner")
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Named provider from ~/.naqb/config.yaml (overrides book.yaml)")
	return cmd
}

func chapterTitle(cfg *config.BookConfig, n int) string {
	for _, ch := range cfg.Chapters {
		if ch.Number == n {
			return ch.Title
		}
	}
	return fmt.Sprintf("Chapter %d", n)
}
