package exporter

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/amr/naqb/internal/config"
)

// WebExporter exports chapters to a static multi-page HTML site with sidebar
// navigation, RTL/LTR support, and light/dark mode.
type WebExporter struct{}

func (WebExporter) Format() string { return "web" }

func (e WebExporter) Export(bookDir string, cfg *config.BookConfig) (string, error) {
	return ExportWeb(bookDir, cfg)
}

// ExportWeb produces a multi-page static site under <bookDir>/output/web/.
//
//	output/web/
//	  index.html          ← TOC / landing page
//	  ch-01.html … ch-NN.html
//	  assets/style.css
func ExportWeb(bookDir string, cfg *config.BookConfig) (string, error) {
	outputDir := filepath.Join(bookDir, "output", "web")
	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o750); err != nil {
		return "", fmt.Errorf("web export: creating output dirs: %w", err)
	}

	// Write shared stylesheet
	if err := os.WriteFile(filepath.Join(assetsDir, "style.css"), []byte(webCSS(cfg)), 0o644); err != nil {
		return "", fmt.Errorf("web export: writing stylesheet: %w", err)
	}

	// Build per-chapter pages
	var pages []chapterPage

	for _, ch := range cfg.Chapters {
		srcPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(ch.Number))
		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue // skip unwritten chapters
		}
		body, err := mdToHTML(data)
		if err != nil {
			body = []byte("<pre>" + html.EscapeString(string(data)) + "</pre>")
		}

		fname := fmt.Sprintf("ch-%02d.html", ch.Number)
		nav := buildNav(cfg, ch.Number, cfg.Language)
		page := buildPage(cfg, ch.Title, string(body), nav, "index.html")
		if err := os.WriteFile(filepath.Join(outputDir, fname), []byte(page), 0o644); err != nil {
			return "", fmt.Errorf("web export: writing %s: %w", fname, err)
		}
		pages = append(pages, chapterPage{ch.Number, ch.Title, fname})
	}

	// Build index / TOC page
	idx := buildIndexPage(cfg, pages)
	idxPath := filepath.Join(outputDir, "index.html")
	if err := os.WriteFile(idxPath, []byte(idx), 0o644); err != nil {
		return "", fmt.Errorf("web export: writing index.html: %w", err)
	}

	return idxPath, nil
}

// ── Markdown renderer ─────────────────────────────────────────────────────────

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
		extension.Typographer,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		gmhtml.WithHardWraps(),
		gmhtml.WithXHTML(),
		gmhtml.WithUnsafe(), // allow raw HTML in markdown
	),
)

func mdToHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Page builders ─────────────────────────────────────────────────────────────

// buildNav returns the sidebar <nav> HTML listing all written chapters.
func buildNav(cfg *config.BookConfig, activeNum int, lang string) string {
	var sb strings.Builder
	sb.WriteString(`<nav class="sidebar" aria-label="chapters">` + "\n")
	sb.WriteString(fmt.Sprintf(`  <a class="site-title" href="index.html">%s</a>`+"\n", html.EscapeString(cfg.Title)))
	sb.WriteString("  <ul>\n")
	for _, ch := range cfg.Chapters {
		fname := fmt.Sprintf("ch-%02d.html", ch.Number)
		active := ""
		if ch.Number == activeNum {
			active = ` class="active"`
		}
		sb.WriteString(fmt.Sprintf(`    <li%s><a href="%s"><span class="ch-num">%02d</span> %s</a></li>`+"\n",
			active, fname, ch.Number, html.EscapeString(ch.Title)))
	}
	sb.WriteString("  </ul>\n</nav>\n")
	return sb.String()
}

// buildPage wraps chapter body HTML in the full page shell.
func buildPage(cfg *config.BookConfig, title, bodyHTML, nav, homeHref string) string {
	dir, lang := dirLang(cfg.Language)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s — %s</title>
  <link rel="stylesheet" href="assets/style.css">
</head>
<body>
%s
<main class="content">
  <header class="page-header">
    <h1>%s</h1>
  </header>
  <article>
%s
  </article>
  <footer>
    <a href="%s">← %s</a>
  </footer>
</main>
</body>
</html>`,
		lang, dir,
		html.EscapeString(title), html.EscapeString(cfg.Title),
		nav,
		html.EscapeString(title),
		bodyHTML,
		homeHref, homeText(cfg.Language),
	)
}

// chapterPage holds metadata for a rendered chapter page.
type chapterPage struct {
	Num      int
	Title    string
	Filename string
}

// buildIndexPage produces the landing / TOC page.
func buildIndexPage(cfg *config.BookConfig, pages []chapterPage) string {
	dir, lang := dirLang(cfg.Language)

	var toc strings.Builder
	toc.WriteString(`<ol class="toc">` + "\n")
	for _, p := range pages {
		toc.WriteString(fmt.Sprintf(`  <li><a href="%s"><span class="ch-num">%02d</span> %s</a></li>`+"\n",
			p.Filename, p.Num, html.EscapeString(p.Title)))
	}
	toc.WriteString("</ol>\n")

	synopsisHTML := ""
	if cfg.Synopsis != "" {
		synopsisHTML = fmt.Sprintf(`<p class="synopsis">%s</p>`, html.EscapeString(cfg.Synopsis))
	}

	// Sidebar for index uses all chapters
	var sb strings.Builder
	sb.WriteString(`<nav class="sidebar" aria-label="chapters">` + "\n")
	sb.WriteString(fmt.Sprintf(`  <a class="site-title active" href="index.html">%s</a>`+"\n", html.EscapeString(cfg.Title)))
	sb.WriteString("  <ul>\n")
	for _, p := range pages {
		sb.WriteString(fmt.Sprintf(`    <li><a href="%s"><span class="ch-num">%02d</span> %s</a></li>`+"\n",
			p.Filename, p.Num, html.EscapeString(p.Title)))
	}
	sb.WriteString("  </ul>\n</nav>\n")
	nav := sb.String()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <link rel="stylesheet" href="assets/style.css">
</head>
<body>
%s
<main class="content">
  <header class="page-header">
    <h1>%s</h1>
    <p class="byline">%s %s</p>
  </header>
  <article>
    %s
    <h2>%s</h2>
    %s
  </article>
</main>
</body>
</html>`,
		lang, dir,
		html.EscapeString(cfg.Title),
		nav,
		html.EscapeString(cfg.Title),
		byLabel(cfg.Language), html.EscapeString(cfg.Author),
		synopsisHTML,
		contentsLabel(cfg.Language),
		toc.String(),
	)
}

// ── CSS ───────────────────────────────────────────────────────────────────────

func webCSS(cfg *config.BookConfig) string {
	fontFace := ltrFonts
	if cfg.Language == "ar" {
		fontFace = rtlFonts
	}
	return fontFace + baseCSS
}

const rtlFonts = `
@import url('https://fonts.googleapis.com/css2?family=Amiri:ital,wght@0,400;0,700;1,400&family=IBM+Plex+Mono&display=swap');
:root { --font-body: 'Amiri', serif; --font-code: 'IBM Plex Mono', monospace; --font-size-body: 1.2rem; }
`

const ltrFonts = `
@import url('https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;600&family=IBM+Plex+Mono&display=swap');
:root { --font-body: 'IBM Plex Sans', system-ui, sans-serif; --font-code: 'IBM Plex Mono', monospace; --font-size-body: 1rem; }
`

const baseCSS = `
/* ── Colour tokens ── */
:root {
  --bg:           #ffffff;
  --bg-sidebar:   #f8f9fa;
  --fg:           #1a1a1a;
  --fg-muted:     #6b7280;
  --accent:       #2563eb;
  --accent-hover: #1d4ed8;
  --border:       #e5e7eb;
  --code-bg:      #1e1e2e;
  --code-fg:      #cdd6f4;
  --sidebar-w:    260px;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg:           #1a1a2e;
    --bg-sidebar:   #16213e;
    --fg:           #e8e8f0;
    --fg-muted:     #9ca3af;
    --accent:       #60a5fa;
    --accent-hover: #93c5fd;
    --border:       #2d3748;
    --code-bg:      #0d0d1a;
    --code-fg:      #cdd6f4;
  }
}

/* ── Reset & base ── */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html { scroll-behavior: smooth; }
body {
  font-family: var(--font-body);
  font-size: var(--font-size-body, 1rem);
  background: var(--bg);
  color: var(--fg);
  display: flex;
  min-height: 100vh;
}

/* ── Sidebar ── */
.sidebar {
  position: fixed;
  top: 0;
  inset-inline-start: 0;
  width: var(--sidebar-w);
  height: 100vh;
  overflow-y: auto;
  background: var(--bg-sidebar);
  border-inline-end: 1px solid var(--border);
  padding: 1.5rem 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.sidebar .site-title {
  font-weight: 600;
  font-size: 1rem;
  color: var(--fg);
  text-decoration: none;
  padding: 0 1.25rem 0.75rem;
  border-bottom: 1px solid var(--border);
  display: block;
  line-height: 1.4;
}
.sidebar .site-title.active { color: var(--accent); }
.sidebar ul { list-style: none; padding: 0.5rem 0; }
.sidebar li a {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 0.45rem 1.25rem;
  color: var(--fg-muted);
  text-decoration: none;
  font-size: 0.9rem;
  line-height: 1.4;
  border-inline-start: 3px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}
.sidebar li a:hover { color: var(--accent); }
.sidebar li.active a {
  color: var(--accent);
  border-inline-start-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 8%, transparent);
}
.ch-num {
  font-variant-numeric: tabular-nums;
  opacity: 0.5;
  font-size: 0.78em;
  flex-shrink: 0;
}

/* ── Main content ── */
.content {
  margin-inline-start: var(--sidebar-w);
  flex: 1;
  max-width: 780px;
  padding: 2.5rem 3rem;
  line-height: 1.75;
}
@media (max-width: 768px) {
  .sidebar { display: none; }
  .content { margin-inline-start: 0; padding: 1.5rem; }
}

/* ── Typography ── */
.page-header h1 { font-size: 2rem; margin-bottom: 0.25rem; }
.byline { color: var(--fg-muted); margin-bottom: 2rem; }
.synopsis { color: var(--fg-muted); font-style: italic; margin-bottom: 1.5rem; border-inline-start: 3px solid var(--accent); padding-inline-start: 1rem; }
article h1 { font-size: 1.75rem; margin: 2rem 0 0.75rem; }
article h2 { font-size: 1.4rem; margin: 1.75rem 0 0.6rem; color: var(--accent); }
article h3 { font-size: 1.15rem; margin: 1.5rem 0 0.5rem; }
article p  { margin-bottom: 1rem; }
article ul, article ol { margin: 0.5rem 0 1rem 1.5rem; }
article li { margin-bottom: 0.3rem; }
article a  { color: var(--accent); text-decoration: underline; text-underline-offset: 2px; }
article a:hover { color: var(--accent-hover); }

/* ── Blockquotes ── */
blockquote {
  border-inline-start: 4px solid var(--accent);
  padding-inline-start: 1rem;
  color: var(--fg-muted);
  margin: 1rem 0;
  font-style: italic;
}

/* ── Code ── */
code {
  font-family: var(--font-code);
  font-size: 0.875em;
  background: var(--code-bg);
  color: var(--code-fg);
  padding: 0.15em 0.35em;
  border-radius: 3px;
}
pre {
  background: var(--code-bg);
  color: var(--code-fg);
  padding: 1.25rem;
  border-radius: 8px;
  overflow-x: auto;
  margin: 1rem 0;
  font-family: var(--font-code);
  font-size: 0.875rem;
  line-height: 1.6;
}
pre code { background: none; padding: 0; font-size: inherit; }

/* ── Tables ── */
table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
th, td { border: 1px solid var(--border); padding: 0.5rem 0.75rem; text-align: start; }
th { background: var(--bg-sidebar); font-weight: 600; }

/* ── TOC ── */
.toc { list-style: none; padding: 0; }
.toc li { padding: 0.5rem 0; border-bottom: 1px solid var(--border); }
.toc li a { color: var(--fg); text-decoration: none; display: flex; align-items: baseline; gap: 0.75rem; font-size: 1.05rem; }
.toc li a:hover { color: var(--accent); }

/* ── Footer ── */
footer { margin-top: 3rem; padding-top: 1rem; border-top: 1px solid var(--border); color: var(--fg-muted); font-size: 0.9rem; }
footer a { color: var(--accent); text-decoration: none; }
`

// ── i18n helpers ──────────────────────────────────────────────────────────────

func dirLang(language string) (dir, lang string) {
	if language == "ar" {
		return "rtl", "ar"
	}
	return "ltr", "en"
}

func homeText(language string) string {
	if language == "ar" {
		return "الفهرس"
	}
	return "Table of Contents"
}

func byLabel(language string) string {
	if language == "ar" {
		return "تأليف"
	}
	return "by"
}

func contentsLabel(language string) string {
	if language == "ar" {
		return "المحتويات"
	}
	return "Contents"
}
