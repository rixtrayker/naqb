// Package commands implements all nqb CLI subcommands.
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/tui"
	"github.com/amr/naqb/internal/vault"
)

// OpenCmd returns the `nqb open <name>` command.
func OpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [name|path]",
		Short: "Open a book project by name (from vault) or path",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return LaunchHome()
			}
			target := args[0]
			return openProject(target)
		},
	}
}

// LaunchHome opens the TUI home screen and handles the result.
func LaunchHome() error {
	if err := vault.EnsureDefaultVault(); err != nil {
		return err
	}

	result, err := tui.RunHome()
	if err != nil {
		return err
	}

	switch result.Action {
	case tui.HomeQuit:
		return nil

	case tui.HomeNew:
		// Run init as sub-process — hybrid approach
		return RunInitInteractive()

	case tui.HomeVaults:
		listVaults()
		return nil

	case tui.HomeOpen:
		if result.Project == nil {
			return fmt.Errorf("no project selected")
		}
		return openBookAt(result.Project.Path)
	}
	return nil
}

// OpenDot opens the current directory as a book project (or prompts to init).
func OpenDot() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	bookYAML := filepath.Join(cwd, "book.yaml")
	if _, err := os.Stat(bookYAML); err == nil {
		// Already a book — open directly
		return openBookAt(cwd)
	}

	// Not a book project — ask
	fmt.Printf("No book.yaml found in %s\n", cwd)
	fmt.Printf("Initialize a new book project here? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer == "y" || answer == "Y" {
		return RunInitInteractive()
	}
	return nil
}

// openProject opens a project by name (from vault) or by filesystem path.
func openProject(nameOrPath string) error {
	// Try as filesystem path first
	abs, err := filepath.Abs(nameOrPath)
	if err == nil {
		if _, err := os.Stat(filepath.Join(abs, "book.yaml")); err == nil {
			return openBookAt(abs)
		}
	}

	// Try as vault project name
	p, err := vault.FindProject(nameOrPath)
	if err != nil {
		return err
	}
	return openBookAt(p.Path)
}

// openBookAt opens the book TUI for a project at a given path.
func openBookAt(bookDir string) error {
	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return err
	}

	// Record in recents
	_ = vault.RecordRecent(bookDir, cfg.Title, cfg.Language)

	// Get API client (optional — book view works without it for navigation)
	var client llm.Provider
	if p, err := providerFor("", cfg.LLM.WriteProvider); err == nil {
		client = p
	}

	return tui.RunBookView(bookDir, cfg, client)
}

// listVaults prints all registered vaults.
func listVaults() {
	reg, err := vault.LoadRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Printf("Registered vaults:\n")
	for _, v := range reg.Vaults {
		fmt.Printf("  %-16s  %s\n", v.Name, v.Path)
	}
}

// RunInitInteractive runs the init flow from the TUI context.
func RunInitInteractive() error {
	// Delegate to init command logic
	result, err := tui.RunInitForm()
	if err != nil {
		return err
	}
	if result.Err != nil || !result.Done {
		return result.Err
	}
	return runInitWithAnswers(result.Answers)
}
