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

// FixCmd returns the `nqb fix` command.
func FixCmd() *cobra.Command {
	var (
		chapterNum int
		modeQA     bool
		modeGap    bool
		modeStyle  bool
		modeRefresh bool
		provider   string
	)

	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Rewrite a chapter to fix QA issues, coverage gaps, or style inconsistencies",
		Long: `nqb fix rewrites a broken or thin chapter using QA issues, gap analysis,
or style markers as correction context.

Modes:
  --qa        (default) read pipeline-report.md issues → rewrite
  --gap       run gap analysis vs outline → rewrite
  --style     compare adjacent chapters for style consistency → rewrite
  --refresh   rebuild context + re-run QA only (no rewrite)

Examples:
  nqb fix --chapter 3
  nqb fix --chapter 3 --gap
  nqb fix --chapter 3 --style
  nqb fix --chapter 3 --refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chapterNum <= 0 {
				return fmt.Errorf("--chapter is required and must be a positive integer")
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
			if err != nil || apiKey == "" {
				return fmt.Errorf("ANTHROPIC_API_KEY not found — set it in your environment or Keychain")
			}

			client := llm.New(apiKey)

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

			_ = provider // reserved for future --provider flag

			var result *agents.FixResult
			label := fmt.Sprintf("fix chapter %d [%s]", chapterNum, mode)

			err = tui.RunWithSpinner(label, func() error {
				ctx := context.Background()
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
	cmd.Flags().StringVarP(&provider, "provider", "p", "anthropic", "LLM provider (reserved for future use)")

	_ = cmd.MarkFlagRequired("chapter")

	return cmd
}
