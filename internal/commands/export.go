package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/exporter"
	"github.com/amr/naqb/internal/tui"
)

// ExportCmd returns the `book export` command.
func ExportCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the book to PDF, EPUB, DOCX, or Web",
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

			for _, f := range formats {
				f := f
				label := fmt.Sprintf("Exporting %s", strings.ToUpper(f))
				err := tui.RunWithSpinner(label, func() error {
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
				}
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
