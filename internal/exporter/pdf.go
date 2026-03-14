// Package exporter wraps pandoc to export book chapters to PDF, EPUB, DOCX, and static web formats.
package exporter

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amr/naqb/internal/config"
)

// ExportPDF exports all chapters to PDF using pandoc + xelatex.
func ExportPDF(bookDir string, cfg *config.BookConfig) (string, error) {
	if err := checkPandoc(); err != nil {
		return "", err
	}
	if err := checkXeLaTeX(); err != nil {
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

	outFile := filepath.Join(outputDir, "book.pdf")

	isRTL := cfg.Language == "ar"
	fontArabic := "Amiri"
	fontLatin := "IBM Plex Sans"

	args := []string{
		"--pdf-engine=xelatex",
		"--toc",
		"--toc-depth=3",
		fmt.Sprintf("--metadata=title:%s", cfg.Title),
		fmt.Sprintf("--metadata=author:%s", cfg.Author),
		fmt.Sprintf("--metadata=lang:%s", cfg.Language),
		"-V", fmt.Sprintf("mainfont=%s", fontArabic),
		"-V", fmt.Sprintf("sansfont=%s", fontLatin),
		"-V", "monofont=JetBrains Mono",
		"-V", "fontsize=12pt",
		"-V", "geometry:margin=2.5cm",
		"-V", "linestretch=1.6",
		"-o", outFile,
	}

	if isRTL {
		args = append(args,
			"-V", "dir=rtl",
			"--variable", "lang=ar",
		)
		// XeLaTeX RTL support via polyglossia
		args = append(args, "--include-in-header="+writeRTLHeader(bookDir, fontArabic))
	}

	args = append(args, chapterFiles...)

	cmd := exec.Command("pandoc", args...)
	cmd.Dir = bookDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pandoc PDF export failed: %w\n%s", err, out)
	}

	return outFile, nil
}

func writeRTLHeader(bookDir, fontArabic string) string {
	header := fmt.Sprintf(`\usepackage{polyglossia}
\setmainlanguage{arabic}
\setotherlanguage{english}
\newfontfamily\arabicfont[Script=Arabic]{%s}
`, fontArabic)
	headerPath := filepath.Join(bookDir, "output", "rtl-header.tex")
	_ = os.MkdirAll(filepath.Dir(headerPath), 0o750)
	_ = os.WriteFile(headerPath, []byte(header), 0o644)
	return headerPath
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

	// Follow the order defined in book.yaml
	for _, ch := range cfg.Chapters {
		path := filepath.Join(chaptersDir, ch.File)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	// If book.yaml has no chapters listed, glob
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
