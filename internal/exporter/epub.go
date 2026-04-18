package exporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amr/naqb/pkg/config"
)

// EPUBExporter exports chapters to EPUB using pandoc.
type EPUBExporter struct{}

func (EPUBExporter) Format() string { return "epub" }

func (EPUBExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
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
	outFile := filepath.Join(outputDir, "book.epub")

	args := commonArgs(cfg)

	// Add custom CSS if present
	cssLight := filepath.Join(bookDir, "assets", "themes", "light.css")
	if _, err := os.Stat(cssLight); err == nil {
		args = append(args, "--css="+cssLight)
	}

	if err := runPandoc(bookDir, chapterFiles, args, outFile); err != nil {
		return "", fmt.Errorf("EPUB export: %w", err)
	}
	return outFile, nil
}

// ExportEPUB is the legacy function wrapper for backwards compatibility.
func ExportEPUB(bookDir string, cfg *config.BookConfig) (string, error) {
	return (EPUBExporter{}).Export(bookDir, cfg)
}
