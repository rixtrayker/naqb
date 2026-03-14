package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/exporter"
	"github.com/amr/naqb/internal/watcher"
)

// WatchCmd returns the `book watch` command.
func WatchCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for file changes and auto-rebuild exports",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			if format == "" {
				format = "web"
			}

			rebuild := func(changedPath string) error {
				fmt.Fprintf(os.Stdout, "Rebuilding %s export...\n", strings.ToUpper(format))
				switch format {
				case "pdf":
					outPath, err := exporter.ExportPDF(bookDir, cfg)
					if err == nil {
						fmt.Fprintf(os.Stdout, "  ✓ PDF → %s\n", outPath)
					}
					return err
				case "epub":
					outPath, err := exporter.ExportEPUB(bookDir, cfg)
					if err == nil {
						fmt.Fprintf(os.Stdout, "  ✓ EPUB → %s\n", outPath)
					}
					return err
				case "docx":
					outPath, err := exporter.ExportDOCX(bookDir, cfg)
					if err == nil {
						fmt.Fprintf(os.Stdout, "  ✓ DOCX → %s\n", outPath)
					}
					return err
				case "web":
					outPath, err := exporter.ExportWeb(bookDir, cfg)
					if err == nil {
						fmt.Fprintf(os.Stdout, "  ✓ Web → %s\n", filepath.Dir(outPath))
					}
					return err
				}
				return fmt.Errorf("unknown format: %s", format)
			}

			return watcher.Watch(bookDir, rebuild, os.Stdout)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "web", "Format to rebuild on change: pdf, epub, docx, web")
	return cmd
}
