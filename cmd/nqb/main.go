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
  nqb open <name>  Open a book from your vault by name`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "." {
				return commands.OpenDot()
			}
			return commands.LaunchHome()
		},
	}

	rootCmd.AddCommand(
		commands.InitCmd(),
		commands.OpenCmd(),
		commands.ContextCmd(),
		commands.WriteCmd(),
		commands.PipelineCmd(),
		commands.QACmd(),
		commands.ExportCmd(),
		commands.WatchCmd(),
		commands.ChatCmd(),
		commands.StatusCmd(),
		commands.ConfigCmd(),
		commands.ResearchCmd(),
		commands.IndexCmd(),
		vault.Cmd(),
		commands.CompletionCmd(rootCmd),
	)

	// Register dynamic completions (for --chapter, --format, open <name>)
	commands.RegisterDynamicCompletions(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
