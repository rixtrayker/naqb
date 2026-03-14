package exporter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/amr/naqb/internal/config"
)

// ExportDOCX exports all chapters to DOCX using pandoc.
func ExportDOCX(bookDir string, cfg *config.BookConfig) (string, error) {
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

	outputDir := filepath.Join(bookDir, "output")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return "", err
	}
	outFile := filepath.Join(outputDir, "book.docx")

	args := []string{
		"--toc",
		fmt.Sprintf("--metadata=title:%s", cfg.Title),
		fmt.Sprintf("--metadata=author:%s", cfg.Author),
		fmt.Sprintf("--metadata=lang:%s", cfg.Language),
		"-o", outFile,
	}

	args = append(args, chapterFiles...)

	cmd := exec.Command("pandoc", args...)
	cmd.Dir = bookDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pandoc DOCX export failed: %w\n%s", err, out)
	}

	return outFile, nil
}
