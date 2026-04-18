package exporter

import (
	"fmt"
	"path/filepath"

	"github.com/amr/naqb/pkg/config"
)

// DOCXExporter exports chapters to DOCX using pandoc.
type DOCXExporter struct{}

func (DOCXExporter) Format() string { return "docx" }

func (DOCXExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
	if err := checkPandoc(); err != nil {
		return "", err
	}

	chapterFiles, err := collectChapterFiles(bookDir, cfg)
	if err != nil {
		return "", err
	}
	if len(chapterFiles) == 0 {
		return "", fmt.Errorf("no chapter files found — run 'nqb write' first")
	}

	outputDir, err := ensureOutputDir(bookDir)
	if err != nil {
		return "", err
	}
	outFile := filepath.Join(outputDir, "book.docx")

	if err := runPandoc(bookDir, chapterFiles, commonArgs(cfg), outFile); err != nil {
		return "", fmt.Errorf("DOCX export: %w", err)
	}
	return outFile, nil
}

// ExportDOCX is the legacy function wrapper for backwards compatibility.
func ExportDOCX(bookDir string, cfg *config.BookConfig) (string, error) {
	return (DOCXExporter{}).Export(bookDir, cfg)
}
