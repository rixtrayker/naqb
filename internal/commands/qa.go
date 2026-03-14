package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/tui"
)

// QACmd returns the `book qa` command.
func QACmd() *cobra.Command {
	var chapterNum int
	var deterministicOnly bool

	cmd := &cobra.Command{
		Use:   "qa",
		Short: "Run QA checks on a chapter (deterministic + LLM)",
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

			apiKey, err := config.APIKey()
			if err != nil {
				return err
			}
			client := llm.New(apiKey)

			var result *agents.QAResult
			label := fmt.Sprintf("Running QA on Chapter %d", chapterNum)

			err = tui.RunWithSpinner(label, func() error {
				var qaErr error
				ctx := context.Background()
				if deterministicOnly {
					// Skip LLM: pass nil client effectively
					result, qaErr = agents.RunQA(ctx, nil, bookDir, cfg, chapterNum)
				} else {
					result, qaErr = agents.RunQA(ctx, client, bookDir, cfg, chapterNum)
				}
				return qaErr
			}, os.Stdout)
			if err != nil {
				return err
			}

			// Print summary
			fmt.Printf("\nQA Results — Chapter %d\n", chapterNum)
			fmt.Printf("─────────────────────────────────────\n")
			if result.DeterministicOK {
				fmt.Printf("Deterministic: ✓ PASSED\n")
			} else {
				fmt.Printf("Deterministic: ✗ ISSUES\n")
				for _, issue := range result.Issues {
					fmt.Printf("  - %s\n", issue)
				}
			}

			if !deterministicOnly && result.LLMReport != "" {
				fmt.Printf("\nLLM Audit:\n%s\n", result.LLMReport)
			}

			// Write report
			if err := agents.WriteQAReport(bookDir, result); err != nil {
				fmt.Printf("(could not write QA report: %v)\n", err)
			} else {
				fmt.Printf("\nReport appended to pipeline-report.md\n")
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&deterministicOnly, "deterministic-only", "d", false, "Skip LLM audit")
	return cmd
}
