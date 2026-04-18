package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/runtime"
)

// PipelineCmd returns the `book pipeline` command.
func PipelineCmd() *cobra.Command {
	var chapterNum int
	var all bool
	var providerFlag string
	var budgetUSD float64
	var parallel int

	cmd := &cobra.Command{
		Use:     "pipeline",
		Aliases: []string{"pipe", "p"},
		Short:   "Run the full pipeline (context → write → qa) for a chapter",
		Long: `Run the complete chapter pipeline: context → write → QA.

Executes all stages in sequence for a single chapter or all chapters.
Optional conflict detection and gap analysis stages are added when enabled
in config/rules.yaml. Supports budget limits to auto-degrade to cheaper
models when spending exceeds the cap.

Prints a per-stage summary table with token counts, cost, and duration.`,
		Example: `  nqb pipeline --chapter 3
  nqb pipeline --all
  nqb pipeline --all --parallel 4
  nqb pipe -c 3 --budget 0.50
  nqb p -c 1 --provider anthropic`,
		GroupID: "writing",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RunPreflight("pipeline"); err != nil {
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
				if parallel > 0 {
					var nums []int
					for _, ch := range cfg.Chapters {
						nums = append(nums, ch.Number)
					}
					res, err := pipeline.RunSwarmWithRules(ctx, pipeline.SwarmInput{
						BookDir:     bookDir,
						Cfg:         cfg,
						Client:      client,
						ChapterNums: nums,
						Out:         os.Stdout,
						Concurrency: parallel,
					})
					if err != nil {
						return err
					}
					for _, ch := range cfg.Chapters {
						if r, ok := res.Results[ch.Number]; ok {
							printPipelineStats(r, ch.Number)
						}
					}
					if len(res.Errors) > 0 {
						fmt.Printf("\n  %d chapter(s) failed\n", len(res.Errors))
					}
					return nil
				}
				for _, ch := range cfg.Chapters {
					fmt.Printf("\nPipeline — Chapter %d: %s\n", ch.Number, ch.Title)
					if err := runPipelineWithGates(ctx, client, bookDir, cfg, ch.Number); err != nil {
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
			return runPipelineWithGates(ctx, client, bookDir, cfg, chapterNum)
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Run pipeline for all chapters")
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Named provider from ~/.naqb/config.yaml (overrides book.yaml)")
	cmd.Flags().Float64Var(&budgetUSD, "budget", 0, "Session budget limit in USD (0 = unlimited); expensive stages auto-degrade when limit crossed")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Run --all chapters in parallel with the given concurrency limit (0 = serial)")
	return cmd
}

// pipeline table styles
var (
	pipeHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	pipeCellStyle   = lipgloss.NewStyle().PaddingRight(1)
	pipeTotalStyle  = lipgloss.NewStyle().Bold(true)
)

// printPipelineStats prints a per-stage stats table if any token data is available.
func printPipelineStats(result *pipeline.PipelineResult, chapterNum int) {
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

	fmt.Printf("\n  Pipeline Complete — Chapter %d\n", chapterNum)

	rows := make([][]string, 0, len(result.Stages)+1)
	for _, s := range result.Stages {
		dur := fmt.Sprintf("%.1fs", s.Duration.Seconds())
		if s.TokensIn == 0 && s.TokensOut == 0 {
			rows = append(rows, []string{s.StageName, "—", "—", "—", dur})
		} else {
			rows = append(rows, []string{
				s.StageName,
				fmtInt(s.TokensIn),
				fmtInt(s.TokensOut),
				fmt.Sprintf("$%.4f", s.EstimatedCost),
				dur,
			})
		}
	}

	// Total row
	rows = append(rows, []string{
		"TOTAL",
		fmtInt(result.TotalTokensIn()),
		fmtInt(result.TotalTokensOut()),
		fmt.Sprintf("$%.4f", result.TotalCost()),
		fmt.Sprintf("%.1fs", result.TotalDuration().Seconds()),
	})

	t := table.New().
		Headers("Stage", "Tokens In", "Tokens Out", "Cost", "Duration").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return pipeHeaderStyle.Padding(0, 1)
			}
			s := pipeCellStyle
			// Bold the total row
			if row == len(rows)-1 {
				s = pipeTotalStyle
			}
			return s.Padding(0, 1)
		})

	fmt.Println(lipgloss.NewStyle().PaddingLeft(2).Render(t.Render()))

	if llm.SessionBudget.Degraded() {
		fmt.Printf("\n  Budget limit crossed — subsequent stages used cheaper model tier\n")
	}
	fmt.Println()
}

// fmtInt formats an int with comma separators.
func fmtInt(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1000)%1000, n%1000)
}

// runPipelineWithGates runs the pipeline loop, prompting the user when a
// human-in-the-loop gate is encountered.
func runPipelineWithGates(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int) error {
	dbPath, err := db.DefaultPath()
	if err != nil {
		return err
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	rules, _ := config.LoadRules(bookDir)
	stages := pipeline.DefaultStagesFor(rules)
	jobID := fmt.Sprintf("pipeline-cli-%d-%d", chapterNum, time.Now().Unix())

	var gatesPassed []string
	for {
		result, runErr := pipeline.Run(ctx, stages, pipeline.StageInput{
			BookDir:     bookDir,
			Cfg:         cfg,
			Client:      client,
			ChapterNum:  chapterNum,
			Out:         os.Stdout,
			DB:          sqlDB,
			JobID:       jobID,
			GatesPassed: gatesPassed,
		})
		if result != nil {
			printPipelineStats(result, chapterNum)
		}
		if interrupted, ok := runtime.IsInterrupted(runErr); ok {
			fmt.Printf("\n⏸ Gate: %s\n", interrupted.Reason)
			if !promptYesNo("Approve and continue? (y/n) ") {
				return fmt.Errorf("pipeline aborted at gate %s", interrupted.NodeID)
			}
			gatesPassed = append(gatesPassed, interrupted.NodeID)
			continue
		}
		return runErr
	}
}

func promptYesNo(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))
		if text == "y" || text == "yes" {
			return true
		}
		if text == "n" || text == "no" {
			return false
		}
		fmt.Println("Please answer y or n")
	}
}
