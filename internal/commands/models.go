package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
)

// ModelsCmd returns the `nqb models` command.
func ModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models, costs, and stage defaults",
		RunE:  runModels,
	}
}

func runModels(_ *cobra.Command, _ []string) error {
	// ── Known models table ────────────────────────────────────────────────────
	fmt.Println("\nAvailable models")
	fmt.Printf("  %-38s %-14s %10s  %11s  %9s  %s\n",
		"MODEL", "PROVIDER", "INPUT $/M", "OUTPUT $/M", "CONTEXT", "SPEED")

	// Sort by speed then name for stable output.
	type row struct {
		id   string
		caps llm.ModelCapabilities
	}
	var rows []row
	for id, caps := range llm.KnownModels {
		rows = append(rows, row{id, caps})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].caps.Speed != rows[j].caps.Speed {
			return rows[i].caps.Speed < rows[j].caps.Speed
		}
		return rows[i].id < rows[j].id
	})

	speedLabel := map[llm.SpeedTier]string{
		llm.SpeedFast:   "fast",
		llm.SpeedMedium: "medium",
		llm.SpeedSlow:   "slow",
	}

	for _, r := range rows {
		provider := "openrouter"
		if len(r.id) > 0 && r.id[0] != '/' {
			// IDs without a slash prefix are Bedrock model IDs
			if r.caps.Speed == llm.SpeedFast && r.id != "minimax/minimax-m2.5" {
				provider = "bedrock"
			}
		}
		ctx := fmt.Sprintf("%dK tok", r.caps.ContextWindow/1000)
		if r.caps.ContextWindow >= 1_000_000 {
			ctx = fmt.Sprintf("%dM tok", r.caps.ContextWindow/1_000_000)
		}
		reasoning := ""
		if r.caps.Reasoning {
			reasoning = " *"
		}
		fmt.Printf("  %-38s %-14s %9.2f$  %10.2f$  %9s  %s%s\n",
			r.id, provider,
			r.caps.InputCostPerMTok, r.caps.OutputCostPerMTok,
			ctx, speedLabel[r.caps.Speed], reasoning)
	}
	fmt.Println("\n  * model uses internal reasoning (chain-of-thought)")

	// ── Stage defaults ────────────────────────────────────────────────────────
	fmt.Println("\nStage defaults")

	stages := []agents.Stage{
		agents.StageInit, agents.StageQA, agents.StageGap,
		agents.StageConflict, agents.StageResearch,
		agents.StageWrite, agents.StageFix,
		agents.StageChat,
	}

	stageGroups := map[string][]agents.Stage{
		"init, qa, gap, conflict, research": {agents.StageInit, agents.StageQA, agents.StageGap, agents.StageConflict, agents.StageResearch},
		"write, fix":                        {agents.StageWrite, agents.StageFix},
		"chat":                              {agents.StageChat},
	}
	groupOrder := []string{"init, qa, gap, conflict, research", "write, fix", "chat"}

	_ = stages // used for reference above
	for _, label := range groupOrder {
		grp := stageGroups[label]
		model := agents.ModelFor(grp[0], nil)
		caps, hasCaps := llm.KnownModels[model]
		tier := ""
		if hasCaps {
			tier = fmt.Sprintf("(%s)", speedLabel[caps.Speed])
		}
		fmt.Printf("  %-42s → %-38s %s\n", label, model, tier)
	}

	// ── Book overrides ────────────────────────────────────────────────────────
	bookDir, err := config.FindBookRoot()
	if err == nil {
		cfg, err := config.LoadBook(bookDir)
		if err == nil && cfg != nil {
			hasOverrides := cfg.LLM.WriteModel != "" || cfg.LLM.QAModel != "" ||
				cfg.LLM.ChatModel != "" || cfg.LLM.InitModel != "" || cfg.LLM.FixModel != ""
			if hasOverrides {
				fmt.Printf("\nBook overrides (%s)\n", cfg.Title)
				printIfSet("  write", cfg.LLM.WriteModel)
				printIfSet("  qa/gap/conflict", cfg.LLM.QAModel)
				printIfSet("  chat", cfg.LLM.ChatModel)
				printIfSet("  init", cfg.LLM.InitModel)
				printIfSet("  fix", cfg.LLM.FixModel)
			}
		}
	}

	fmt.Fprintln(os.Stdout)
	return nil
}

func printIfSet(label, model string) {
	if model != "" {
		fmt.Printf("  %-22s → %s\n", label, model)
	}
}
