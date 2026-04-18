// Package exporter wraps pandoc to export book chapters to various formats.
package exporter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/pkg/config"
)

// Exporter exports a book project to a specific format.
type Exporter interface {
	// Export runs the export and returns the path to the generated file.
	Export(bookDir string, cfg *config.BookConfig) (string, error)
	// Format returns the canonical format name (e.g. "pdf", "epub", "docx", "web").
	Format() string
}

// Registry maps format names to their Exporter implementations.
var Registry = map[string]Exporter{
	"pdf":  PDFExporter{},
	"epub": EPUBExporter{},
	"docx": DOCXExporter{},
	"web":  WebExporter{},
}

// Export dispatches to the correct Exporter for the given format name.
// Use "all" to run all registered exporters.
func Export(format, bookDir string, cfg *config.BookConfig) ([]string, error) {
	if format == "all" {
		var paths []string
		for name, exp := range Registry {
			path, err := exp.Export(bookDir, cfg)
			if err != nil {
				return paths, fmt.Errorf("%s export failed: %w", name, err)
			}
			paths = append(paths, path)
		}
		return paths, nil
	}
	exp, ok := Registry[format]
	if !ok {
		return nil, fmt.Errorf("unknown export format %q — supported: pdf, epub, docx, web, all", format)
	}
	path, err := exp.Export(bookDir, cfg)
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// ── Shared helpers ───────────────────────────────────────────────────────────

// pandocArgs holds common pandoc arguments shared across formats.
type pandocArgs struct {
	extra   []string // format-specific extra flags
	outFile string
}

// runPandoc executes pandoc with the given chapter files and args.
func runPandoc(bookDir string, chapterFiles []string, args []string, outFile string) error {
	fullArgs := append(args, "-o", outFile)
	fullArgs = append(fullArgs, chapterFiles...)

	cmd := exec.Command("pandoc", fullArgs...)
	cmd.Dir = bookDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pandoc failed: %w\n%s", err, out)
	}
	return nil
}

// commonArgs returns the metadata flags shared by all pandoc-based formats.
func commonArgs(cfg *config.BookConfig) []string {
	return []string{
		"--toc",
		fmt.Sprintf("--metadata=title:%s", cfg.Title),
		fmt.Sprintf("--metadata=author:%s", cfg.Author),
		fmt.Sprintf("--metadata=lang:%s", cfg.Language),
	}
}

// ensureOutputDir creates <bookDir>/output and returns its path.
func ensureOutputDir(bookDir string) (string, error) {
	dir := filepath.Join(bookDir, "output")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	return dir, nil
}

func checkPandoc() error {
	if _, err := exec.LookPath("pandoc"); err != nil {
		return fmt.Errorf("pandoc not found — install with: brew install pandoc (macOS) or apt install pandoc (Ubuntu)")
	}
	return nil
}

func checkXeLaTeX() error {
	if _, err := exec.LookPath("xelatex"); err != nil {
		return fmt.Errorf("xelatex not found — install TeX Live: https://tug.org/texlive/")
	}
	return nil
}

func collectChapterFiles(bookDir string, cfg *config.BookConfig) ([]string, error) {
	chaptersDir := filepath.Join(bookDir, "chapters")
	var files []string

	for _, ch := range cfg.Chapters {
		path := filepath.Join(chaptersDir, ch.File)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	if len(files) == 0 {
		entries, err := filepath.Glob(filepath.Join(chaptersDir, "ch-*.md"))
		if err != nil {
			return nil, err
		}
		files = entries
	}

	return files, nil
}

// CombineChapters concatenates all chapter markdown files into a single string.
func CombineChapters(bookDir string, cfg *config.BookConfig) (string, error) {
	files, err := collectChapterFiles(bookDir, cfg)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
