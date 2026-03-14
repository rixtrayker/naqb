package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/pipeline"
)

// PipelineCmd returns the `book pipeline` command.
func PipelineCmd() *cobra.Command {
	var chapterNum int
	var all bool

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run the full pipeline (context → write → qa) for a chapter",
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
			ctx := context.Background()

			if all {
				for _, ch := range cfg.Chapters {
					fmt.Printf("\nPipeline — Chapter %d: %s\n", ch.Number, ch.Title)
					if err := pipeline.RunChapterPipeline(ctx, client, bookDir, cfg, ch.Number, os.Stdout); err != nil {
						fmt.Fprintf(os.Stderr, "  Chapter %d failed: %v\n", ch.Number, err)
					}
				}
				return nil
			}

			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N or --all")
			}

			title := chapterTitle(cfg, chapterNum)
			fmt.Printf("Pipeline — Chapter %d: %s\n", chapterNum, title)
			return pipeline.RunChapterPipeline(ctx, client, bookDir, cfg, chapterNum, os.Stdout)
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Run pipeline for all chapters")
	return cmd
}
