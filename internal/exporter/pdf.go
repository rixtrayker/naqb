package exporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amr/naqb/pkg/config"
)

// PDFExporter exports chapters to PDF using pandoc + xelatex.
type PDFExporter struct{}

func (PDFExporter) Format() string { return "pdf" }

func (PDFExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
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

	outputDir, err := ensureOutputDir(bookDir)
	if err != nil {
		return "", err
	}
	outFile := filepath.Join(outputDir, "book.pdf")

	const fontArabic = "Amiri"
	const fontLatin = "IBM Plex Sans"

	args := append(commonArgs(cfg),
		"--pdf-engine=xelatex",
		"--toc-depth=3",
		"-V", fmt.Sprintf("mainfont=%s", fontArabic),
		"-V", fmt.Sprintf("sansfont=%s", fontLatin),
		"-V", "monofont=JetBrains Mono",
		"-V", "fontsize=12pt",
		"-V", "geometry:margin=2.5cm",
		"-V", "linestretch=1.6",
	)

	if cfg.Language == "ar" {
		headerPath, err := writeRTLHeader(bookDir, fontArabic)
		if err != nil {
			return "", err
		}
		args = append(args,
			"-V", "dir=rtl",
			"--variable", "lang=ar",
			"--include-in-header="+headerPath,
		)
	}

	if err := runPandoc(bookDir, chapterFiles, args, outFile); err != nil {
		return "", fmt.Errorf("PDF export: %w", err)
	}
	return outFile, nil
}

// ExportPDF is the legacy function wrapper for backwards compatibility.
func ExportPDF(bookDir string, cfg *config.BookConfig) (string, error) {
	return (PDFExporter{}).Export(bookDir, cfg)
}

func writeRTLHeader(bookDir, fontArabic string) (string, error) {
	header := fmt.Sprintf(`\usepackage{polyglossia}
\setmainlanguage{arabic}
\setotherlanguage{english}
\newfontfamily\arabicfont[Script=Arabic]{%s}
`, fontArabic)
	headerPath := filepath.Join(bookDir, "output", "rtl-header.tex")
	if err := os.MkdirAll(filepath.Dir(headerPath), 0o750); err != nil {
		return "", fmt.Errorf("creating RTL header dir: %w", err)
	}
	if err := os.WriteFile(headerPath, []byte(header), 0o644); err != nil {
		return "", fmt.Errorf("writing RTL header: %w", err)
	}
	return headerPath, nil
}
