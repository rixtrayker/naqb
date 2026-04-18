package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/vault"
)

// CompletionCmd returns the `nqb completion` command with shell-specific subcommands
// and a carapace spec subcommand for carapace-sh integration.
func CompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for bash, zsh, fish, or carapace.

Install completions for your shell to get tab-completion for commands,
flags, chapter numbers, and project names.`,
		Example: `  nqb completion bash > ~/.bash_completion.d/nqb
  nqb completion zsh > "${fpath[1]}/_nqb"
  nqb completion fish > ~/.config/fish/completions/nqb.fish`,
		GroupID: "utility",
	}

	cmd.AddCommand(
		completionBashCmd(root),
		completionZshCmd(root),
		completionFishCmd(root),
		completionCarapaceCmd(root),
	)
	return cmd
}

func completionBashCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long: `Generate a bash completion script. Source it in your .bashrc or
.bash_profile to enable tab completion for nqb commands and flags.`,
		Example: `  nqb completion bash > ~/.bash_completion.d/nqb
  source <(nqb completion bash)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenBashCompletion(os.Stdout)
		},
	}
}

func completionZshCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long: `Generate a zsh completion script. Place it in your fpath to enable
tab completion for nqb commands and flags.`,
		Example: `  nqb completion zsh > "${fpath[1]}/_nqb"
  source <(nqb completion zsh)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenZshCompletion(os.Stdout)
		},
	}
}

func completionFishCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long: `Generate a fish completion script. Place it in your fish completions
directory to enable tab completion for nqb commands and flags.`,
		Example: `  nqb completion fish > ~/.config/fish/completions/nqb.fish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenFishCompletion(os.Stdout, true)
		},
	}
}

func completionCarapaceCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "carapace [shell]",
		Short: "Generate carapace-sh completion spec",
		Long: `Generate a carapace-sh completion spec for nqb.

To install:
  1. Install carapace: https://carapace.sh
  2. Add to your shell config:
       export CARAPACE_BRIDGES='zsh,fish,bash'
       source <(carapace _carapace)
  3. Register nqb:
       nqb completion carapace zsh  # or fish, bash, etc.`,
		Example: `  nqb completion carapace zsh
  nqb completion carapace fish
  nqb completion carapace bash`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := "zsh"
			if len(args) > 0 {
				shell = args[0]
			}
			return printCarapaceSpec(shell)
		},
	}
}

// printCarapaceSpec prints a carapace completion spec for the given shell.
// This uses cobra's built-in completion with dynamic completors registered below.
func printCarapaceSpec(shell string) error {
	// We leverage cobra's __complete mechanism + dynamic completions.
	// Print install instructions for carapace integration.
	fmt.Printf("# nqb (نقب) shell completion via carapace-sh\n")
	fmt.Printf("# Shell: %s\n\n", shell)

	switch shell {
	case "zsh":
		fmt.Printf(`# Add to ~/.zshrc:
# export CARAPACE_BRIDGES='zsh,fish,bash'
# source <(carapace _carapace)
# carapace nqb zsh | source /dev/stdin
`)
	case "bash":
		fmt.Printf(`# Add to ~/.bashrc:
# source <(carapace _carapace bash)
# carapace nqb bash | source /dev/stdin
`)
	case "fish":
		fmt.Printf(`# Add to ~/.config/fish/config.fish:
# carapace _carapace | source
# carapace nqb fish | source
`)
	}

	fmt.Printf("\n# Dynamic completions are registered via cobra's ValidArgsFunction.\n")
	fmt.Printf("# Run: carapace nqb %s\n", shell)
	return nil
}

// RegisterDynamicCompletions adds ValidArgsFunction and RegisterFlagCompletionFunc
// to commands that benefit from dynamic completion.
// Call this after building the command tree, before Execute().
func RegisterDynamicCompletions(root *cobra.Command) {
	// Find subcommands by use
	for _, sub := range root.Commands() {
		if sub.Use == "open [name|path]" {
			sub.ValidArgsFunction = completeProjectNames
		}
		// Register flag completions on all subcommands
		registerFlagCompletions(sub)
	}
}

func registerFlagCompletions(cmd *cobra.Command) {
	// --chapter flag: complete with chapter numbers from current book
	if cmd.Flags().Lookup("chapter") != nil {
		_ = cmd.RegisterFlagCompletionFunc("chapter", completeChapterNumbers)
	}
	// --format flag: complete with export formats
	if cmd.Flags().Lookup("format") != nil {
		_ = cmd.RegisterFlagCompletionFunc("format", completeExportFormats)
	}
}

func completeProjectNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	names := vault.ProjectNames()
	var filtered []string
	for _, n := range names {
		if strings.HasPrefix(n, toComplete) {
			filtered = append(filtered, n)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}

func completeChapterNumbers(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	bookDir, err := config.FindBookRoot()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var nums []string
	for _, ch := range cfg.Chapters {
		n := strconv.Itoa(ch.Number)
		if strings.HasPrefix(n, toComplete) {
			// Include chapter title as description
			nums = append(nums, fmt.Sprintf("%d\t%s", ch.Number, ch.Title))
		}
	}
	return nums, cobra.ShellCompDirectiveNoFileComp
}

func completeExportFormats(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	formats := []string{
		"pdf\tPDF via pandoc + XeLaTeX",
		"epub\tEPUB via pandoc",
		"docx\tWord document via pandoc",
		"web\tStatic HTML site",
		"all\tAll formats",
	}
	var filtered []string
	for _, f := range formats {
		if strings.HasPrefix(f, toComplete) {
			filtered = append(filtered, f)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}
