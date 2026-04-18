// Package main is the entry point for nqb (نقب), the LLM-powered book writing CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/commands"
	"github.com/amr/naqb/internal/vault"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nqb",
		Short: "نقب — excavate your ideas, give them depth",
		Long: `nqb (نقب) — LLM-powered book writing tool.

  nqb              Launch interactive project picker
  nqb .            Open current directory as a book project
  nqb open <name>  Open a book from your vault by name

Use 'nqb <command> --help' for detailed help on any command.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "." {
				return commands.OpenDot()
			}
			return commands.LaunchHome()
		},
	}

	// ── Persistent output flags ─────────────────────────────────────────────
	rootCmd.PersistentFlags().BoolVarP(&commands.Output.Verbose, "verbose", "v", false, "Show debug info (LLM requests, token counts, timing)")
	rootCmd.PersistentFlags().BoolVarP(&commands.Output.Quiet, "quiet", "q", false, "Suppress non-essential output (only errors and final results)")
	rootCmd.PersistentFlags().BoolVar(&commands.Output.NoColor, "no-color", false, "Disable color output (also respects NO_COLOR env var)")

	// ── Command groups ──────────────────────────────────────────────────────
	rootCmd.AddGroup(
		&cobra.Group{ID: "writing", Title: "Writing:"},
		&cobra.Group{ID: "quality", Title: "Quality:"},
		&cobra.Group{ID: "publishing", Title: "Publishing:"},
		&cobra.Group{ID: "management", Title: "Management:"},
		&cobra.Group{ID: "config", Title: "Configuration:"},
		&cobra.Group{ID: "utility", Title: "Utility:"},
	)

	rootCmd.AddCommand(
		// Writing
		commands.InitCmd(),
		commands.WriteCmd(),
		commands.ContextCmd(),
		commands.PipelineCmd(),
		commands.FixCmd(),
		commands.ChatCmd(),

		// Quality
		commands.QACmd(),
		commands.ResearchCmd(),
		commands.IndexCmd(),

		// Publishing
		commands.ExportCmd(),
		commands.WatchCmd(),
		commands.StatusCmd(),

		// Management
		commands.OpenCmd(),
		commands.BatchCmd(),
		commands.SessionCmd(),
		vault.Cmd(),
		commands.ImportCmd(),
		commands.SyncCmd(),
		commands.ListCmd(),
		commands.ChangelogCmd(),

		// Configuration
		commands.ConfigCmd(),
		commands.KeysCmd(),
		commands.SetupCmd(),
		commands.ModelsCmd(),
		commands.MCPCmd(),

		// Utility
		commands.VersionCmd(),
		commands.DoctorCmd(),
		commands.CompletionCmd(rootCmd),
	)

	// Register dynamic completions (for --chapter, --format, open <name>)
	commands.RegisterDynamicCompletions(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
