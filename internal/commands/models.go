package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// ModelsCmd returns the `nqb models` command.
func ModelsCmd() *cobra.Command {
	var allTiers bool
	cmd := &cobra.Command{
		Use:     "models",
		Aliases: []string{"m"},
		Short:   "List available models, costs, and stage defaults",
		Long: `List all known models with pricing, context window, and speed.

Shows the model table with per-million-token costs, context window sizes,
and speed tiers. Also displays stage-to-model mapping and any book-level
overrides from book.yaml.

Pricing tiers (Anthropic direct API):
  standard      Default rate (up to 200K context)
  long-context  Premium rate when input exceeds 200K tokens (beta)
  batch         50% off for async jobs via Batches API
  fast          6x standard rate for latency-sensitive workloads`,
		Example: `  nqb models
  nqb models --all-tiers
  nqb m`,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModels(allTiers)
		},
	}
	cmd.Flags().BoolVar(&allTiers, "all-tiers", false, "Show all pricing tiers (standard, batch, fast, long-context)")
	return cmd
}

func runModels(allTiers bool) error {
	speedLabel := map[llm.SpeedTier]string{
		llm.SpeedFast:   "fast",
		llm.SpeedMedium: "medium",
		llm.SpeedSlow:   "slow",
	}

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

	// ── Known models table ────────────────────────────────────────────────────
	fmt.Println("\nAvailable models  (standard pricing)")
	fmt.Printf("  %-38s %-14s %10s  %11s  %9s  %s\n",
		"MODEL", "PROVIDER", "INPUT $/M", "OUTPUT $/M", "CONTEXT", "SPEED")

	for _, r := range rows {
		provider := providerLabel(r.id)
		ctx := contextLabel(r.caps.ContextWindow)
		reasoning := ""
		if r.caps.Reasoning {
			reasoning = " *"
		}
		stdIn, stdOut := r.caps.Costs.CostForTier(llm.PricingStandard)
		fmt.Printf("  %-38s %-14s %9.2f$  %10.2f$  %9s  %s%s\n",
			r.id, provider, stdIn, stdOut, ctx, speedLabel[r.caps.Speed], reasoning)
	}
	fmt.Println("\n  * model uses internal reasoning (chain-of-thought)")

	// ── Multi-tier breakdown ──────────────────────────────────────────────────
	if allTiers {
		fmt.Println("\nPricing tiers (Anthropic models)")
		tierNames := []struct {
			tier  llm.PricingTier
			label string
			note  string
		}{
			{llm.PricingStandard, "standard", "up to 200K context"},
			{llm.PricingBatch, "batch", "50% off, async Batches API"},
			{llm.PricingFast, "fast", "6× standard, latency-sensitive"},
			{llm.PricingLongContext, "long-context", "input > 200K tokens (beta)"},
		}

		for _, r := range rows {
			// Only show multi-tier breakdown for models that have non-standard pricing.
			c := r.caps.Costs
			if c.BatchIn == 0 && c.FastIn == 0 && c.LongContextIn == 0 {
				continue
			}
			fmt.Printf("\n  %s\n", r.id)
			fmt.Printf("    %-16s  %10s  %11s  %s\n", "TIER", "INPUT $/M", "OUTPUT $/M", "NOTES")
			for _, t := range tierNames {
				in, out := c.CostForTier(t.tier)
				fmt.Printf("    %-16s  %9.2f$  %10.2f$  %s\n", t.label, in, out, t.note)
			}
		}
	} else {
		// Hint that more exists.
		hasTiers := false
		for _, r := range rows {
			if r.caps.Costs.BatchIn > 0 {
				hasTiers = true
				break
			}
		}
		if hasTiers {
			fmt.Println("  (use --all-tiers to see batch / fast / long-context pricing)")
		}
	}

	// ── Stage defaults ────────────────────────────────────────────────────────
	fmt.Println("\nStage defaults")
	stageGroups := []struct {
		label  string
		stages []agents.Stage
	}{
		{"init, qa, gap, conflict, research", []agents.Stage{agents.StageInit, agents.StageQA, agents.StageGap, agents.StageConflict, agents.StageResearch}},
		{"write, fix", []agents.Stage{agents.StageWrite, agents.StageFix}},
		{"chat", []agents.Stage{agents.StageChat}},
	}
	for _, g := range stageGroups {
		model := agents.ModelFor(g.stages[0], nil)
		tier := ""
		if caps, ok := llm.KnownModels[model]; ok {
			tier = fmt.Sprintf("(%s)", speedLabel[caps.Speed])
		}
		fmt.Printf("  %-42s → %-38s %s\n", g.label, model, tier)
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

func providerLabel(modelID string) string {
	// Dot-notation IDs are Bedrock (e.g. "minimax.minimax-m2.1")
	for _, ch := range modelID {
		if ch == '/' {
			return "openrouter"
		}
		if ch == '.' {
			return "bedrock"
		}
	}
	return "openrouter"
}

func contextLabel(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%dM tok", n/1_000_000)
	}
	return fmt.Sprintf("%dK tok", n/1000)
}

func printIfSet(label, model string) {
	if model != "" {
		fmt.Printf("  %-22s → %s\n", label, model)
	}
}
