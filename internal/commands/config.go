package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/log"
)

// ConfigCmd returns the `book config` command.
func ConfigCmd() *cobra.Command {
	var setKey bool

	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Show or edit global book configuration",
		Long: `Display or modify the global nqb configuration at ~/.naqb/config.yaml.

Without flags, prints the current configuration including API keys (masked),
providers, default model, and editor settings. Use --set-key to interactively
set the Anthropic API key.`,
		Example: `  nqb config
  nqb config --set-key
  nqb cfg`,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadGlobal()
			if err != nil {
				return err
			}

			if setKey {
				return setAPIKey(cfg)
			}

			// Default: show config path and current settings
			fmt.Printf("Config file:  %s\n", config.GlobalConfigPath())
			fmt.Printf("Log file:     %s  (set NQB_DEBUG=1 for verbose logs)\n\n", log.LogPath())
			if cfg.APIKey != "" {
				fmt.Printf("api_key:          %s  (legacy)\n", maskKey(cfg.APIKey))
			} else {
				fmt.Printf("api_key:          (not set — use 'nqb config --set-key' or set ANTHROPIC_API_KEY)\n")
			}
			if cfg.DefaultProvider != "" {
				fmt.Printf("default_provider: %s\n", cfg.DefaultProvider)
			}
			if len(cfg.Providers) > 0 {
				fmt.Printf("providers:\n")
				for name, p := range cfg.Providers {
					keyDisplay := "(no key)"
					if p.APIKey != "" {
						keyDisplay = maskKey(p.APIKey)
					}
					if p.BaseURL != "" {
						fmt.Printf("  %-20s  type=%-14s  key=%s  base_url=%s\n", name, p.Type, keyDisplay, p.BaseURL)
					} else {
						fmt.Printf("  %-20s  type=%-14s  key=%s\n", name, p.Type, keyDisplay)
					}
				}
			}
			if cfg.DefaultModel != "" {
				fmt.Printf("default_model:    %s\n", cfg.DefaultModel)
			}
			if cfg.Editor != "" {
				fmt.Printf("editor:           %s\n", cfg.Editor)
			}

			// Also try to open in editor if EDITOR is set
			editor := os.Getenv("EDITOR")
			if editor != "" && len(args) > 0 && args[0] == "edit" {
				return openInEditor(editor, config.GlobalConfigPath())
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&setKey, "set-key", false, "Interactively set the Anthropic API key")
	return cmd
}

func setAPIKey(cfg *config.GlobalConfig) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter your Anthropic API key (starts with 'sk-ant-'): ")
	key, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if !strings.HasPrefix(key, "sk-ant-") {
		fmt.Printf("Warning: key doesn't start with 'sk-ant-' — saving anyway\n")
	}
	cfg.APIKey = key
	if err := config.SaveGlobal(cfg); err != nil {
		return err
	}
	fmt.Printf("✓ API key saved to %s\n", config.GlobalConfigPath())
	return nil
}

func maskKey(key string) string {
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func openInEditor(editor, path string) error {
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
