package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
)

// version is set via -ldflags at build time (see Makefile).
var version = "dev"

// VersionCmd returns the `nqb version` command.
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Print nqb version, Go runtime, and build info",
		GroupID: "utility",
		Long:    `Print the nqb version number, Go runtime version, and build metadata.`,
		Example: `  nqb version
  nqb v`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nqb v%s (build %s)\n", config.NaqbVersion, version)
			fmt.Printf("go  %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
