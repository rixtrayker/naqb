# Export

`nqb` uses **pandoc** to convert your Markdown chapters into publication-ready formats.

---

## Supported Formats

| Format | Command | Requirements |
|--------|---------|-------------|
| PDF (Arabic RTL) | `nqb export --format pdf` | `pandoc` + `xelatex` + Amiri font |
| PDF (English) | `nqb export --format pdf` | `pandoc` + `xelatex` |
| EPUB | `nqb export --format epub` | `pandoc` |
| DOCX | `nqb export --format docx` | `pandoc` |
| Web (HTML) | `nqb export --format web` | Built-in (no deps) |
| All formats | `nqb export --format all` | All of the above |

Output goes to `output/` in your book directory (gitignored by default).

---

## Installing Dependencies

### macOS

```bash
brew install pandoc
brew install --cask mactex        # full TeX Live, includes xelatex

# Amiri font (for Arabic PDF)
# Download from https://fonts.google.com/specimen/Amiri
# Double-click .ttf files to install via Font Book
```

### Ubuntu / Debian

```bash
apt install pandoc texlive-xetex fonts-arabeyes
```

### Arch Linux

```bash
pacman -S pandoc texlive-xetex
# AUR: yay -S ttf-amiri
```

---

## Arabic RTL PDF

The PDF exporter uses `polyglossia` + `Amiri` font for correct right-to-left rendering.

The LaTeX header injected by `nqb` looks like:

```latex
\usepackage{polyglossia}
\setmainlanguage{arabic}
\setotherlanguage{english}
\setmainfont{Amiri}
\newfontfamily\arabicfont{Amiri}[Script=Arabic]
```

If `Amiri` is not installed, pandoc will error. Install it as described above.

---

## Output Directory

```
your-book/
└── output/
    ├── book.pdf
    ├── book.epub
    ├── book.docx
    └── web/
        ├── index.html
        └── style.css
```

The `output/` directory is added to `.gitignore` by `nqb init`.

---

## From the TUI

In the book view, press `e` or type `/export --format pdf` in the command palette.

The TUI export command currently prints a message directing you to the CLI for full export
(pandoc requires a real terminal, not an alt-screen TUI). Run:

```bash
nqb export --format pdf
```

---

## Watch Mode

`nqb watch` monitors your `chapters/` directory and re-runs export automatically
when a `.md` file changes (500ms debounce):

```bash
nqb watch   # keeps running, Ctrl+C to stop
```
