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
		Long: `Manage vault registrations. A vault is a directory that contains
one or more book projects. The default vault is ~/Documents/naqb-books
(or ~/.naqb/projects as fallback).

Use subcommands to list, add, or remove vaults.`,
		Example: `  nqb vault list
  nqb vault add work ~/Work/books
  nqb vault remove work`,
		GroupID: "management",
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
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all registered vaults",
		Long:    `Display all registered vaults with their names and filesystem paths.`,
		Example: `  nqb vault list
  nqb vault ls`,
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
		Long: `Register a directory as a named vault. Books inside this directory
will appear in the project picker and can be opened by name.`,
		Example: `  nqb vault add work ~/Work/books
  nqb vault add research /data/research-books`,
		Args: cobra.ExactArgs(2),
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
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Unregister a vault (does not delete files)",
		Long: `Remove a vault from the registry. This only unregisters the vault;
no files or book projects are deleted from disk.`,
		Example: `  nqb vault remove work
  nqb vault rm research`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RemoveVault(args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ Vault %q removed\n", args[0])
			return nil
		},
	}
}
