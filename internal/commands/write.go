package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/search"
	"github.com/amr/naqb/internal/tui/components"
)

// WriteCmd returns the `book write` command.
func WriteCmd() *cobra.Command {
	var chapterNum int
	var stream bool
	var providerFlag string

	cmd := &cobra.Command{
		Use:     "write",
		Aliases: []string{"w"},
		Short:   "Write a chapter using Claude Sonnet (with spinner)",
		Long: `Generate a chapter draft using the configured LLM provider.

Reads the book outline, context files, and research notes, then writes a
complete chapter draft to chapters/ch-XX.md. The chapter is automatically
indexed into the local vector store after writing.

Use --stream to watch tokens arrive in real time instead of the spinner.`,
		Example: `  nqb write --chapter 3
  nqb write -c 3 --stream
  nqb write -c 1 --provider anthropic
  nqb w -c 5`,
		GroupID: "writing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N")
			}
			if err := RunPreflight("write"); err != nil {
				return err
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

			ctx := context.Background()

			if stream {
				// Streaming mode: print tokens as they arrive
				fmt.Printf("Writing chapter %d (streaming)...\n\n", chapterNum)
				path, err := agents.WriteChapter(ctx, client, bookDir, cfg, chapterNum, func(delta string) error {
					fmt.Print(delta)
					return nil
				})
				fmt.Println()
				if err != nil {
					return err
				}
				fmt.Printf("\n✓ Chapter written → %s\n", path)
				indexChapter(ctx, bookDir, chapterNum, path)
				return nil
			}

			// Spinner mode (default)
			label := fmt.Sprintf("Writing Chapter %d: %s", chapterNum, chapterTitle(cfg, chapterNum))
			var path string
			err = components.RunWithSpinner(label, func() error {
				var writeErr error
				path, writeErr = agents.WriteChapter(ctx, client, bookDir, cfg, chapterNum, nil)
				return writeErr
			}, os.Stdout)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Chapter written → %s\n", path)
			indexChapter(ctx, bookDir, chapterNum, path)
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

// indexChapter adds the written chapter to the vector store (best-effort).
func indexChapter(ctx context.Context, bookDir string, chapterNum int, filePath string) {
	store, err := search.Open(bookDir)
	if err != nil {
		log.Warn("vector index: failed to open store", "err", err)
		return
	}
	defer store.Close()
	if indexErr := store.IndexChapter(ctx, bookDir, chapterNum, filePath); indexErr != nil {
		log.Warn("vector index: failed to index chapter", "chapter", chapterNum, "err", indexErr)
		return
	}
	log.Info("vector index: chapter indexed", "chapter", chapterNum)
}
