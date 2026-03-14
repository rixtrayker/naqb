package exporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amr/naqb/internal/config"
)

// WebExporter exports chapters to a static HTML site.
// Full MkDocs Material integration is Phase 2; this produces a simple index.html.
type WebExporter struct{}

func (WebExporter) Format() string { return "web" }

func (e WebExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
	return ExportWeb(bookDir, cfg)
}

// ExportWeb creates a simple static HTML site from chapter markdown files.
func ExportWeb(bookDir string, cfg *config.BookConfig) (string, error) {
	combined, err := CombineChapters(bookDir, cfg)
	if err != nil {
		return "", err
	}

	outputDir := filepath.Join(bookDir, "output", "web")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return "", err
	}

	dir := "ltr"
	if cfg.Language == "ar" {
		dir = "rtl"
	}

	html := buildHTML(cfg.Title, cfg.Author, combined, dir)
	outFile := filepath.Join(outputDir, "index.html")
	if err := os.WriteFile(outFile, []byte(html), 0o644); err != nil {
		return "", fmt.Errorf("writing web output: %w", err)
	}

	return outFile, nil
}

func buildHTML(title, author, markdownContent, dir string) string {
	// Escape HTML entities in the content for the meta description
	_ = markdownContent // content would be processed by a markdown library in production

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <meta name="author" content="%s">
  <style>
    :root { --bg: #fafafa; --fg: #1a1a1a; --accent: #2563eb; }
    @media (prefers-color-scheme: dark) {
      :root { --bg: #1a1a1a; --fg: #f0f0f0; --accent: #60a5fa; }
    }
    body {
      font-family: "IBM Plex Sans", "Amiri", system-ui, sans-serif;
      background: var(--bg); color: var(--fg);
      max-width: 800px; margin: 0 auto; padding: 2rem;
      line-height: 1.6;
    }
    h1, h2, h3 { color: var(--accent); }
    code { font-family: "JetBrains Mono", monospace; }
    pre { background: #1e1e2e; color: #cdd6f4; padding: 1rem; border-radius: 6px; overflow-x: auto; }
    blockquote { border-inline-start: 4px solid var(--accent); padding-inline-start: 1rem; }
  </style>
</head>
<body>
  <h1>%s</h1>
  <p><em>by %s</em></p>
  <hr>
  <p><em>Note: This is a raw Markdown export. A full rendered version will be available in Phase 2.</em></p>
  <pre>%s</pre>
</body>
</html>`, toLang(dir), dir, title, author, title, author, escapeHTML(markdownContent))
}

func toLang(dir string) string {
	if dir == "rtl" {
		return "ar"
	}
	return "en"
}

func escapeHTML(s string) string {
	s2 := ""
	for _, c := range s {
		switch c {
		case '<':
			s2 += "&lt;"
		case '>':
			s2 += "&gt;"
		case '&':
			s2 += "&amp;"
		default:
			s2 += string(c)
		}
	}
	return s2
}
