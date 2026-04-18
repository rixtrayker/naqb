package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/exporter"
	"github.com/amr/naqb/internal/tui/components"
)

// ExportCmd returns the `book export` command.
func ExportCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "export",
		Aliases: []string{"exp"},
		Short:   "Export the book to PDF, EPUB, DOCX, or Web",
		Long: `Export the book to one or more output formats using pandoc.

Supported formats: pdf (via XeLaTeX), epub, docx, web (static HTML).
Use --format all to export to all formats at once. Requires pandoc to
be installed; PDF also requires XeLaTeX.`,
		Example: `  nqb export --format pdf
  nqb export -f epub
  nqb export -f all
  nqb exp -f docx`,
		GroupID: "publishing",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			formats := parseFormats(format)
			if len(formats) == 0 {
				return fmt.Errorf("specify --format pdf|epub|docx|web|all")
			}

			var failures int
			for _, f := range formats {
				f := f
				label := fmt.Sprintf("Exporting %s", strings.ToUpper(f))
				err := components.RunWithSpinner(label, func() error {
					var outPath string
					var exportErr error
					switch f {
					case "pdf":
						outPath, exportErr = exporter.ExportPDF(bookDir, cfg)
					case "epub":
						outPath, exportErr = exporter.ExportEPUB(bookDir, cfg)
					case "docx":
						outPath, exportErr = exporter.ExportDOCX(bookDir, cfg)
					case "web":
						outPath, exportErr = exporter.ExportWeb(bookDir, cfg)
					default:
						exportErr = fmt.Errorf("unknown format: %s", f)
					}
					if exportErr == nil {
						fmt.Fprintf(os.Stdout, "  → %s\n", outPath)
					}
					return exportErr
				}, os.Stdout)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s export failed: %v\n", f, err)
					failures++
				}
			}
			if failures > 0 {
				return fmt.Errorf("%d of %d exports failed", failures, len(formats))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "", "Output format: pdf, epub, docx, web, or all")
	return cmd
}

func parseFormats(f string) []string {
	f = strings.ToLower(strings.TrimSpace(f))
	if f == "all" {
		return []string{"pdf", "epub", "docx", "web"}
	}
	if f == "" {
		return nil
	}
	return []string{f}
}
