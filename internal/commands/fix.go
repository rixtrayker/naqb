package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/tui/components"
)

// FixCmd returns the `nqb fix` command.
func FixCmd() *cobra.Command {
	var (
		chapterNum  int
		modeQA      bool
		modeGap     bool
		modeStyle   bool
		modeRefresh bool
		smartMode   bool
		provider    string
	)

	cmd := &cobra.Command{
		Use:     "fix",
		Aliases: []string{"f"},
		Short:   "Rewrite a chapter to fix QA issues, coverage gaps, or style inconsistencies",
		Long: `Rewrite a chapter using QA issues, gap analysis, or style markers as
correction context. The original chapter is backed up before rewriting.

Modes:
  --qa        (default) read pipeline-report.md issues and rewrite
  --gap       run gap analysis vs outline and rewrite
  --style     compare adjacent chapters for style consistency and rewrite
  --refresh   rebuild context + re-run QA only (no rewrite)
  --smart     classify issue complexity and route to appropriate model tier`,
		Example: `  nqb fix --chapter 3
  nqb fix -c 3 --gap
  nqb fix -c 3 --style --smart
  nqb fix -c 3 --refresh
  nqb f -c 5`,
		GroupID: "writing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chapterNum <= 0 {
				return fmt.Errorf("--chapter is required and must be a positive integer")
			}

			if err := RunPreflight("fix"); err != nil {
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

				client, err := providerFor(provider, cfg.LLM.FixProvider)
			if err != nil {
				return err
			}

			// Determine mode — default to QA
			mode := agents.FixModeQA
			switch {
			case modeRefresh:
				mode = agents.FixModeRefresh
			case modeGap:
				mode = agents.FixModeGap
			case modeStyle:
				mode = agents.FixModeStyle
			}

			var result *agents.FixResult
			label := fmt.Sprintf("fix chapter %d [%s]", chapterNum, mode)

			err = components.RunWithSpinner(label, func() error {
				ctx := context.Background()

				// --smart: classify issue complexity and route to appropriate model tier
				if smartMode && mode != agents.FixModeRefresh {
					issues := agents.ReadQAIssues(bookDir, chapterNum)
					if len(issues) > 0 {
						desc := fmt.Sprintf("Fix %d issue(s) in chapter %d: %s", len(issues), chapterNum, issues[0])
						complexity := agents.ClassifyTask(ctx, client, desc)
						smartModel := agents.ModelForComplexity(complexity)
						fmt.Fprintf(os.Stdout, "  smart: complexity=%d → model=%s\n", complexity, smartModel)
						cfg.LLM.FixModel = smartModel
					}
				}

				var ferr error
				result, ferr = agents.FixChapter(ctx, client, bookDir, cfg, chapterNum, mode)
				return ferr
			}, os.Stdout)
			if err != nil {
				return fmt.Errorf("fix: %w", err)
			}

			// ── Print result summary ──────────────────────────────────────────
			if result.Skipped {
				fmt.Println("  ↺  Refresh mode — context rebuilt, QA re-run, no rewrite.")
			} else {
				fmt.Printf("  ↓  Backup:   %s\n", result.BackupPath)
				fmt.Printf("  ✓  Rewritten: %s\n", result.NewPath)
				if result.Warning != "" {
					fmt.Printf("  ⚠  Warning: %s\n", result.Warning)
				}
			}

			if result.QAResult != nil {
				if result.QAResult.Passed {
					fmt.Printf("  ✓  QA: passed (%s)\n", result.QAResult.DeterministicMsg)
				} else {
					fmt.Printf("  ✗  QA: %d issue(s) remaining\n", len(result.QAResult.Issues))
					for _, issue := range result.QAResult.Issues {
						fmt.Printf("       - %s\n", issue)
					}
				}
			}

			if len(result.IssuesFound) > 0 {
				fmt.Printf("  ℹ  %d issue(s) addressed\n", len(result.IssuesFound))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number to fix (required)")
	cmd.Flags().BoolVar(&modeQA, "qa", false, "Fix mode: use pipeline-report.md QA issues (default)")
	cmd.Flags().BoolVar(&modeGap, "gap", false, "Fix mode: run gap analysis vs outline")
	cmd.Flags().BoolVar(&modeStyle, "style", false, "Fix mode: style consistency with adjacent chapters")
	cmd.Flags().BoolVar(&modeRefresh, "refresh", false, "Refresh mode: rebuild context + QA only, no rewrite")
	cmd.Flags().BoolVar(&smartMode, "smart", false, "Smart mode: classify issue complexity and route to appropriate model tier")
	cmd.Flags().StringVarP(&provider, "provider", "p", "anthropic", "LLM provider (reserved for future use)")

	_ = cmd.MarkFlagRequired("chapter")

	return cmd
}
