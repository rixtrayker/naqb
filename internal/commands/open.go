// Package commands implements all nqb CLI subcommands.
package commands

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/agent"
	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/internal/tui"
	"github.com/amr/naqb/internal/vault"
)

// OpenCmd returns the `nqb open <name>` command.
func OpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "open [name|path]",
		Aliases: []string{"o"},
		Short:   "Open a book project by name (from vault) or path",
		Long: `Open a book project in the interactive TUI.

Accepts a vault project name or a filesystem path. Without arguments,
launches the project picker home screen.`,
		Example: `  nqb open my-book
  nqb open ~/Books/project
  nqb open
  nqb o my-book`,
		GroupID: "management",
		Args:    cobra.MaximumNArgs(1),
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
// On first run or after a version upgrade it runs the setup wizard first.
func LaunchHome() error {
	// First-run / upgrade check — non-blocking (errors are soft-warned).
	if cfg, err := config.LoadGlobal(); err == nil && config.NeedsSetup(cfg) {
		if wizardErr := RunSetupWizard(false); wizardErr != nil {
			fmt.Fprintf(os.Stderr, "setup wizard error: %v\n", wizardErr)
		}
	}

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

// OpenDot opens the current directory as a book project.
// Uses the interactive agent chat when a Fantasy provider is available,
// falling back to BookView otherwise.
func OpenDot() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	bookYAML := filepath.Join(cwd, "book.yaml")
	if _, err := os.Stat(bookYAML); err == nil {
		return openBookAtAgentChat(cwd)
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
// Always uses BookView (nqb open <name>).
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

// openBookAt opens the book in BookView (chapter list TUI).
// Used by `nqb open <name>` and the home screen project picker.
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

// openBookAtAgentChat opens the book in the interactive agent chat.
// Falls back to BookView if the Fantasy provider is not available.
func openBookAtAgentChat(bookDir string) error {
	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return err
	}

	_ = vault.RecordRecent(bookDir, cfg.Title, cfg.Language)

	// Try to build Fantasy provider for agent chat
	fantasyProvider, _, err := agent.NewProviderFromGlobalConfig()
	if err != nil {
		log.Debug("agent chat unavailable, falling back to book view", "err", err)
		var client llm.Provider
		if p, err := providerFor("", cfg.LLM.WriteProvider); err == nil {
			client = p
		}
		return tui.RunBookView(bookDir, cfg, client)
	}

	// Open database for session persistence
	dbPath, err := db.DefaultPath()
	if err != nil {
		log.Debug("db unavailable for agent chat", "err", err)
		dbPath = ""
	}
	var sqlDB *sql.DB
	if dbPath != "" {
		if d, err := db.Open(dbPath); err == nil {
			sqlDB = d
			defer sqlDB.Close()
		}
	}

	// Resolve model
	modelID := agents.ModelFor(agents.StageChat, cfg)

	// Build an LLM provider for background tasks (QA, pipeline, research)
	var llmClient llm.Provider
	if p, err := providerFor("", cfg.LLM.WriteProvider); err == nil {
		llmClient = p
	}

	return tui.RunAgentChat(bookDir, cfg, fantasyProvider, modelID, sqlDB, llmClient)
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
