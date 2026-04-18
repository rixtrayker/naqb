package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/changelog"
	"github.com/amr/naqb/pkg/config"
)

// ChangelogCmd returns the `nqb changelog` command.
func ChangelogCmd() *cobra.Command {
	var limit int
	var output string

	cmd := &cobra.Command{
		Use:     "changelog",
		Aliases: []string{"changes", "log"},
		Short:   "Generate a human-readable changelog from recent git commits",
		Long: `Reads the git commit history for the current book project and produces
na user-friendly session diary categorised by activity type (writing, research,
QA, publishing, etc.). Useful as a post-session summary or for appending to
pipeline-report.md.`,
		Example: `  nqb changelog
  nqb changelog --limit 10
  nqb changelog -o session-summary.md`,
		GroupID: "management",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}

			report, err := changelog.Generate(bookDir, limit)
			if err != nil {
				return err
			}

			md := changelog.FormatMarkdown(report)

			if output != "" {
				outPath := filepath.Join(bookDir, output)
				if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
					return fmt.Errorf("failed to write changelog: %w", err)
				}
				fmt.Printf("✓ Changelog saved to %s\n", outPath)
				return nil
			}

			fmt.Println(md)
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Number of recent commits to include")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write changelog to a file inside the book directory")
	return cmd
}
