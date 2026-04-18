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

// QACmd returns the `book qa` command.
func QACmd() *cobra.Command {
	var chapterNum int
	var deterministicOnly bool
	var providerFlag string
	var runConflict bool
	var runGap bool

	cmd := &cobra.Command{
		Use:     "qa",
		Aliases: []string{"check"},
		Short:   "Run QA checks on a chapter (deterministic + LLM)",
		Long: `Run quality assurance checks on a written chapter.

Checks include:
  - Deterministic: heading hierarchy, code block tags, word count, callout syntax
  - LLM audit:     consistency, ADHD formatting, outline coverage, terminology
  - Conflict check: cross-chapter contradiction detection (--conflict)
  - Gap analysis:   outline-vs-content coverage check (--gap)

Results are appended to pipeline-report.md. Use --deterministic-only to skip
the LLM audit when no API key is available.`,
		Example: `  nqb qa --chapter 3
  nqb qa -c 3 --deterministic-only
  nqb qa -c 3 --conflict --gap
  nqb check -c 1`,
		GroupID: "quality",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N")
			}
			if err := RunPreflight("qa"); err != nil {
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
			rules, _ := config.LoadRules(bookDir)

			ctx := context.Background()

			// Resolve LLM client — required for LLM-based checks.
			client, clientErr := providerFor(providerFlag, cfg.LLM.QAProvider)
			needsLLM := !deterministicOnly || runConflict || runGap
			if needsLLM && clientErr != nil {
				return clientErr
			}

			// --- Standard QA ---
			var result *agents.QAResult
			label := fmt.Sprintf("Running QA on Chapter %d", chapterNum)
			err = components.RunWithSpinner(label, func() error {
				var qaErr error
				if deterministicOnly {
					result, qaErr = agents.RunQA(ctx, nil, bookDir, cfg, chapterNum)
				} else {
					result, qaErr = agents.RunQA(ctx, client, bookDir, cfg, chapterNum)
				}
				return qaErr
			}, os.Stdout)
			if err != nil {
				return err
			}

			// Print QA summary
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
			switch {
			case deterministicOnly:
				fmt.Printf("\nLLM audit: skipped (--deterministic-only)\n")
			case clientErr != nil:
				fmt.Printf("\nLLM audit: skipped (provider unavailable: %v)\n", clientErr)
			case result.LLMReport != "":
				fmt.Printf("\nLLM Audit:\n%s\n", result.LLMReport)
			default:
				fmt.Printf("\nLLM audit: no findings\n")
			}
			if err := agents.WriteQAReport(bookDir, result); err != nil {
				fmt.Printf("(could not write QA report: %v)\n", err)
			} else {
				fmt.Printf("\nReport appended to pipeline-report.md\n")
			}

			// --- Conflict check ---
			conflictLevel := rules.QA.ConflictLevel
			if runConflict && conflictLevel == "off" {
				conflictLevel = "light" // --conflict flag overrides "off"
			}
			if runConflict || conflictLevel != "off" {
				fmt.Printf("\nRunning conflict check (level: %s)...\n", conflictLevel)
				cr, cerr := agents.RunConflictCheck(ctx, client, bookDir, cfg, chapterNum, conflictLevel)
				if cerr != nil {
					fmt.Fprintf(os.Stderr, "Conflict check failed: %v\n", cerr)
				} else {
					_ = agents.WriteConflictReport(bookDir, cr)
					if cr.HasIssues {
						fmt.Printf("Conflict: ⚠ potential issues found — see pipeline-report.md\n")
					} else {
						fmt.Printf("Conflict: ✓ none detected\n")
					}
				}
			}

			// --- Gap analysis ---
			gapLevel := rules.QA.GapLevel
			if runGap && gapLevel == "off" {
				gapLevel = "light"
			}
			if runGap || gapLevel != "off" {
				fmt.Printf("\nRunning gap analysis (level: %s)...\n", gapLevel)
				gr, gerr := agents.RunGapAnalysis(ctx, client, bookDir, cfg, chapterNum, gapLevel)
				if gerr != nil {
					fmt.Fprintf(os.Stderr, "Gap analysis failed: %v\n", gerr)
				} else {
					_ = agents.WriteGapReport(bookDir, gr)
					if gr.HasGaps {
						fmt.Printf("Gaps: ⚠ coverage gaps found — see pipeline-report.md\n")
					} else {
						fmt.Printf("Gaps: ✓ outline well-covered\n")
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&deterministicOnly, "deterministic-only", "d", false, "Skip LLM audit")
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Named provider from ~/.naqb/config.yaml (overrides book.yaml)")
	cmd.Flags().BoolVar(&runConflict, "conflict", false, "Run cross-chapter conflict check (uses rules.yaml level)")
	cmd.Flags().BoolVar(&runGap, "gap", false, "Run outline gap analysis (uses rules.yaml level)")
	return cmd
}
