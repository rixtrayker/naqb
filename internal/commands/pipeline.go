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
	var providerFlag string
	var budgetUSD float64

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

			if budgetUSD > 0 {
				llm.SessionBudget.SetLimit(budgetUSD)
				fmt.Printf("  Budget limit: $%.2f\n", budgetUSD)
			}

			// Pipeline uses the write provider as the default for all stages.
			// Individual stage providers can be overridden in book.yaml if needed.
			client, err := providerWithFallback(providerFlag, cfg.LLM.WriteProvider, cfg.LLM.FallbackProvider)
			if err != nil {
				return err
			}
			ctx := context.Background()

			if all {
				for _, ch := range cfg.Chapters {
					fmt.Printf("\nPipeline — Chapter %d: %s\n", ch.Number, ch.Title)
					if _, err := pipeline.RunChapterPipeline(ctx, client, bookDir, cfg, ch.Number, os.Stdout); err != nil {
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
			result, runErr := pipeline.RunChapterPipeline(ctx, client, bookDir, cfg, chapterNum, os.Stdout)
			if result != nil {
				printPipelineStats(result)
			}
			return runErr
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Run pipeline for all chapters")
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Named provider from ~/.naqb/config.yaml (overrides book.yaml)")
	cmd.Flags().Float64Var(&budgetUSD, "budget", 0, "Session budget limit in USD (0 = unlimited); expensive stages auto-degrade when limit crossed")
	return cmd
}

// printPipelineStats prints a per-stage stats table if any token data is available.
func printPipelineStats(result *pipeline.PipelineResult) {
	hasTokens := false
	for _, s := range result.Stages {
		if s.TokensIn > 0 || s.TokensOut > 0 {
			hasTokens = true
			break
		}
	}
	if !hasTokens {
		return
	}

	fmt.Println()
	fmt.Printf("  %-12s  %10s  %10s  %8s  %8s\n", "Stage", "Tokens In", "Tokens Out", "Cost", "Duration")
	fmt.Printf("  %s\n", "────────────────────────────────────────────────────────")
	for _, s := range result.Stages {
		dur := fmt.Sprintf("%.1fs", s.Duration.Seconds())
		if s.TokensIn == 0 && s.TokensOut == 0 {
			fmt.Printf("  %-12s  %10s  %10s  %8s  %8s\n", s.StageName, "—", "—", "—", dur)
		} else {
			cost := fmt.Sprintf("$%.4f", s.EstimatedCost)
			fmt.Printf("  %-12s  %10s  %10s  %8s  %8s\n",
				s.StageName,
				fmt.Sprintf("%d", s.TokensIn),
				fmt.Sprintf("%d", s.TokensOut),
				cost, dur)
		}
	}
	fmt.Printf("  %s\n", "────────────────────────────────────────────────────────")
	fmt.Printf("  %-12s  %10d  %10d  %8s  %8s\n",
		"TOTAL",
		result.TotalTokensIn(), result.TotalTokensOut(),
		fmt.Sprintf("$%.4f", result.TotalCost()),
		fmt.Sprintf("%.1fs", result.TotalDuration().Seconds()))

	if llm.SessionBudget.Degraded() {
		fmt.Printf("\n  ⚠ Budget limit crossed — subsequent stages used cheaper model tier\n")
	}
	fmt.Println()
}
