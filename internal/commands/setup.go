package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/keycheck"
	"github.com/amr/naqb/internal/vault"
)

// SetupCmd returns the `nqb setup` command.
func SetupCmd() *cobra.Command {
	var forceRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive setup wizard: vault path + API keys",
		Long: `Walk through the nqb configuration interactively.

Run automatically on first launch and after version upgrades that add new
configuration fields. Guides you through vault path selection and API key
setup. Re-run manually at any time.`,
		Example: `  nqb setup
  nqb setup --force`,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunSetupWizard(forceRun)
		},
	}
	cmd.Flags().BoolVar(&forceRun, "force", false, "Re-run even if already configured for this version")
	return cmd
}

// RunSetupWizard runs the interactive first-run / upgrade setup wizard.
// If skip is false and config is already up to date, it prints a confirmation
// and returns without prompting (used for the auto-trigger path).
// Pass force=true (from `nqb setup --force` or manual invocation) to always run.
func RunSetupWizard(force bool) error {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if !force && !config.NeedsSetup(cfg) {
		fmt.Printf("nqb is already configured (version %s). Run `nqb setup --force` to re-run.\n", cfg.SetupVersion)
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)

	isFirstRun := cfg.SetupVersion == ""
	if isFirstRun {
		fmt.Println()
		fmt.Println("  Welcome to نقب (nqb) — your LLM-powered book writing tool.")
		fmt.Println("  Let's set up a few things before you start writing.")
		fmt.Println()
	} else {
		fmt.Printf("\n  nqb %s — new configuration options available.\n\n", config.NaqbVersion)
	}

	// ── Step 1: vault path ────────────────────────────────────────────────
	if err := setupVaultPath(scanner, cfg); err != nil {
		return err
	}

	// ── Step 2: API keys ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  API Keys")
	fmt.Println("  ─────────────────────────────────────────────────────────")
	fmt.Println("  Keys are stored in macOS Keychain (never in plaintext files).")
	fmt.Println()

	statuses := keycheck.ResolveAll()
	for _, k := range statuses {
		s := keycheck.Resolve(k)
		label := s.Name
		if s.Set {
			fmt.Printf("  %-26s  ✓ already set (%s via %s)\n", label, s.Masked, s.Source)
			continue
		}
		// Skip non-essential keys silently; only prompt for commonly needed ones.
		if !isEssentialKey(s.Name) {
			fmt.Printf("  %-26s  — skipped (optional)\n", label)
			continue
		}
		fmt.Printf("  %-26s  not set\n", label)
		fmt.Printf("  Enter value (or press Enter to skip): ")
		if !scanner.Scan() {
			break
		}
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			fmt.Printf("  → skipped\n")
			continue
		}
		if err := keycheck.KeychainSet(s.Name, val); err != nil {
			fmt.Printf("  → error saving: %v\n", err)
		} else {
			fmt.Printf("  → saved to Keychain ✓\n")
		}
	}

	// ── Step 3: mark complete ─────────────────────────────────────────────
	cfg.SetupVersion = config.NaqbVersion
	if err := config.SaveGlobal(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("  Setup complete. Run `nqb keys` at any time to review key status.")
	fmt.Println("  Run `nqb setup` to revisit these settings.")
	fmt.Println()
	return nil
}

// setupVaultPath asks the user to confirm or change the default vault path.
func setupVaultPath(scanner *bufio.Scanner, cfg *config.GlobalConfig) error {
	suggestedPath := vault.DefaultVaultPath()

	// If the user already set a custom path, show it.
	currentPath := cfg.DefaultVaultPath
	if currentPath == "" {
		currentPath = suggestedPath
	}

	fmt.Println("  Vault Location")
	fmt.Println("  ─────────────────────────────────────────────────────────")
	fmt.Println("  New book projects will be saved to your vault directory.")
	fmt.Printf("  Current: %s\n", currentPath)

	// Suggest ~/Documents/naqb-books if ~/Documents exists and we're not already there.
	docsPath := documentsVaultSuggestion()
	if docsPath != "" && docsPath != currentPath {
		fmt.Printf("  Suggestion: %s  (recommended on macOS/Linux)\n", docsPath)
	}

	fmt.Print("\n  Keep this path? [Y/n or type a new path]: ")
	if !scanner.Scan() {
		return nil // EOF — keep current
	}
	answer := strings.TrimSpace(scanner.Text())

	switch {
	case answer == "" || strings.EqualFold(answer, "y"):
		// Keep current — nothing to do
		fmt.Printf("  → vault: %s\n", currentPath)
	case strings.EqualFold(answer, "n"):
		// Default to suggestion if available, else ask for path
		if docsPath != "" {
			cfg.DefaultVaultPath = docsPath
			fmt.Printf("  → vault: %s\n", docsPath)
		} else {
			fmt.Print("  Enter vault path: ")
			if !scanner.Scan() {
				return nil
			}
			p := strings.TrimSpace(scanner.Text())
			if p == "" {
				fmt.Printf("  → keeping %s\n", currentPath)
				return nil
			}
			expanded := expandHome(p)
			abs, err := filepath.Abs(expanded)
			if err != nil {
				fmt.Printf("  → invalid path, keeping %s\n", currentPath)
				return nil
			}
			cfg.DefaultVaultPath = abs
			fmt.Printf("  → vault: %s\n", abs)
		}
	default:
		// Treat non-y/n input as a direct path entry
		expanded := expandHome(answer)
		abs, err := filepath.Abs(expanded)
		if err != nil {
			fmt.Printf("  → invalid path, keeping %s\n", currentPath)
			return nil
		}
		cfg.DefaultVaultPath = abs
		fmt.Printf("  → vault: %s\n", abs)
	}

	// Ensure the chosen vault directory exists
	chosen := cfg.DefaultVaultPath
	if chosen == "" {
		chosen = suggestedPath
	}
	if err := os.MkdirAll(chosen, 0o750); err != nil {
		fmt.Printf("  → warning: could not create %s: %v\n", chosen, err)
	}
	return nil
}

// documentsVaultSuggestion returns ~/Documents/naqb-books if ~/Documents exists.
func documentsVaultSuggestion() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	docs := filepath.Join(home, "Documents")
	if info, err := os.Stat(docs); err == nil && info.IsDir() {
		return filepath.Join(docs, "naqb-books")
	}
	return ""
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// isEssentialKey returns true for keys that most users need to set on first run.
func isEssentialKey(name string) bool {
	switch name {
	case "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY":
		return true
	}
	return false
}
