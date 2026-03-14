// Package vault provides the vault registry and the `nqb vault` subcommand group.
package vault

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Cmd returns the `nqb vault` subcommand group.
func Cmd() *cobra.Command {
	vaultCmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vaults (collections of book projects)",
	}

	vaultCmd.AddCommand(
		vaultListCmd(),
		vaultAddCmd(),
		vaultRemoveCmd(),
	)
	return vaultCmd
}

func vaultListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all registered vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := LoadRegistry()
			if err != nil {
				return err
			}
			fmt.Printf("%-16s  %s\n", "NAME", "PATH")
			fmt.Printf("%-16s  %s\n", "────", "────")
			for _, v := range reg.Vaults {
				marker := ""
				if v.Name == DefaultVaultName {
					marker = " (default)"
				}
				fmt.Printf("%-16s  %s%s\n", v.Name, v.Path, marker)
			}
			return nil
		},
	}
}

func vaultAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Register a directory as a named vault",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := AddVault(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("✓ Vault %q added → %s\n", args[0], args[1])
			return nil
		},
	}
}

func vaultRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Unregister a vault (does not delete files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RemoveVault(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ Vault %q removed\n", args[0])
			return nil
		},
	}
}
