package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/search"
)

const contextTemplate = `# ROLE
You are an expert Technical Author specializing in {{.Domain}}.

# GLOBAL BOOK RULES
- Tone: Professional, engaging, ADHD-friendly
- Formatting: Clean Markdown; LaTeX for math equations
- Language: {{.LanguageDesc}} with {{.TermsDesc}} technical terms in brackets
- ADHD-friendly: bold keywords, frequent subheadings, color callouts ([!] note, [?] deep dive, [X] warning)
- Target chapter length: ~{{.TargetWords}} words

# PROJECT STATE
- Book Title: {{.Title}}
- Author: {{.Author}}
- Synopsis: {{.Synopsis}}
{{- if .FinishedSummaries}}

## Finished Chapters
{{.FinishedSummaries}}
{{- end}}

# CURRENT TARGET
Chapter {{.ChapterNum}}: {{.ChapterTitle}}

## Chapter Summary/Goal
{{.ChapterSummary}}

## Specific Outline
{{.ChapterOutline}}
{{- if .ResearchNotes}}

## Research Notes
{{.ResearchNotes}}
{{- end}}

# OUTPUT INSTRUCTION
Generate the FULL content for Chapter {{.ChapterNum}}: {{.ChapterTitle}}.
Start directly with the first heading. No preamble or meta-commentary.
Be generous with examples{{if .HasCode}}, code snippets,{{end}} and analogies.
Use [!], [?], [X] callout blocks to highlight important points.
`

type contextData struct {
	Domain            string
	Title             string
	Author            string
	Synopsis          string
	LanguageDesc      string
	TermsDesc         string
	TargetWords       int
	FinishedSummaries string
	ChapterNum        int
	ChapterTitle      string
	ChapterSummary    string
	ChapterOutline    string
	ResearchNotes     string
	HasCode           bool
}

// BuildContext assembles the single-shot context file for chapter n.
func BuildContext(bookDir string, cfg *config.BookConfig, chapterNum int) (string, error) {
	return BuildContextWithCtx(context.Background(), bookDir, cfg, chapterNum)
}

// BuildContextWithCtx assembles the single-shot context file for chapter n.
// It uses the vector store for semantic research retrieval when available.
func BuildContextWithCtx(ctx context.Context, bookDir string, cfg *config.BookConfig, chapterNum int) (string, error) {
	// Find the chapter config
	var ch *config.Chapter
	for i := range cfg.Chapters {
		if cfg.Chapters[i].Number == chapterNum {
			ch = &cfg.Chapters[i]
			break
		}
	}
	if ch == nil {
		return "", fmt.Errorf("chapter %d not found in book.yaml", chapterNum)
	}

	// Read outline.md to extract the chapter's section
	outlineSection := extractOutlineSection(bookDir, chapterNum, ch.Title)

	// Build finished chapter summaries
	finishedSummaries := buildFinishedSummaries(cfg, chapterNum)

	// Research notes: try semantic search first, fall back to file scan.
	researchNotes := buildResearchNotes(ctx, bookDir, ch.Title, ch.Summary)
	if researchNotes == "" {
		// Fallback: read all notes from both dirs (no vector store / no embedder)
		researchNotes = readResearchNotes(filepath.Join(bookDir, "research"))
		autoNotes := readResearchNotes(filepath.Join(bookDir, ".naqb", "research"))
		if autoNotes != "" {
			if researchNotes != "" {
				researchNotes += "\n\n" + autoNotes
			} else {
				researchNotes = autoNotes
			}
		}
	}

	// Determine language descriptions
	langDesc, termsDesc, hasCode := languageDescs(cfg.Language, cfg.Domain)

	data := contextData{
		Domain:            cfg.Domain,
		Title:             cfg.Title,
		Author:            cfg.Author,
		Synopsis:          cfg.Synopsis,
		LanguageDesc:      langDesc,
		TermsDesc:         termsDesc,
		TargetWords:       cfg.TargetWords,
		FinishedSummaries: finishedSummaries,
		ChapterNum:        chapterNum,
		ChapterTitle:      ch.Title,
		ChapterSummary:    ch.Summary,
		ChapterOutline:    outlineSection,
		ResearchNotes:     researchNotes,
		HasCode:           hasCode,
	}

	tmpl, err := template.New("context").Parse(contextTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing context template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("executing context template: %w", err)
	}

	return sb.String(), nil
}

// WriteContextFile builds the context and saves it to contexts/ch-XX-context.md.
func WriteContextFile(bookDir string, cfg *config.BookConfig, chapterNum int) (string, error) {
	content, err := BuildContextWithCtx(context.Background(), bookDir, cfg, chapterNum)
	if err != nil {
		return "", err
	}

	contextsDir := filepath.Join(bookDir, "contexts")
	if err := os.MkdirAll(contextsDir, 0o750); err != nil {
		return "", err
	}

	outPath := filepath.Join(contextsDir, config.ContextFilename(chapterNum))
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing context file: %w", err)
	}
	return outPath, nil
}

func extractOutlineSection(bookDir string, chapterNum int, chapterTitle string) string {
	outlinePath := filepath.Join(bookDir, "outline.md")
	data, err := os.ReadFile(outlinePath)
	if err != nil {
		return fmt.Sprintf("Write a comprehensive chapter on: %s", chapterTitle)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Look for a heading matching this chapter number
	markers := []string{
		fmt.Sprintf("## Chapter %d:", chapterNum),
		fmt.Sprintf("## %d.", chapterNum),
		fmt.Sprintf("### Chapter %d", chapterNum),
		fmt.Sprintf("## Chapter %d", chapterNum),
	}

	startIdx := -1
	for i, line := range lines {
		for _, marker := range markers {
			if strings.HasPrefix(strings.TrimSpace(line), marker) {
				startIdx = i
				break
			}
		}
		if startIdx >= 0 {
			break
		}
	}

	if startIdx < 0 {
		return fmt.Sprintf("Write a comprehensive chapter on: %s", chapterTitle)
	}

	// Collect lines until the next chapter heading
	var section []string
	for i := startIdx; i < len(lines); i++ {
		if i > startIdx && (strings.HasPrefix(lines[i], "## ") || strings.HasPrefix(lines[i], "# ")) {
			break
		}
		section = append(section, lines[i])
	}
	return strings.TrimSpace(strings.Join(section, "\n"))
}

func buildFinishedSummaries(cfg *config.BookConfig, currentChapter int) string {
	var parts []string
	for _, ch := range cfg.Chapters {
		if ch.Number < currentChapter && ch.Summary != "" {
			parts = append(parts, fmt.Sprintf("- **Chapter %d: %s** — %s", ch.Number, ch.Title, ch.Summary))
		}
	}
	return strings.Join(parts, "\n")
}

// buildResearchNotes retrieves relevant research notes using a two-tier strategy:
//
//	Tier 1: Vector semantic search (OPENAI/MISTRAL key present)
//	Tier 2: File-scan keyword search (always available, no API key needed)
func buildResearchNotes(ctx context.Context, bookDir, chapterTitle, chapterSummary string) string {
	query := chapterTitle
	if chapterSummary != "" {
		query = chapterTitle + ". " + chapterSummary
	}

	store, err := search.Open(bookDir)
	if err != nil {
		return ""
	}
	defer store.Close()

	// QueryResearch uses semantic search when an embedder is available,
	// and falls back to file-scan keyword search otherwise.
	results, err := store.QueryResearch(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	return formatResults(results)
}

func formatResults(results []search.SearchResult) string {
	var parts []string
	for _, r := range results {
		name := r.ID
		if r.Path != "" {
			name = filepath.Base(r.Path)
		}
		content := r.Content
		if len(content) > llm.MaxResearchCharsPerNote {
			content = content[:llm.MaxResearchCharsPerNote] + "\n... (truncated)"
		}
		parts = append(parts, fmt.Sprintf("### %s\n%s", name, content))
	}
	return strings.Join(parts, "\n\n")
}

func readResearchNotes(researchDir string) string {
	entries, err := os.ReadDir(researchDir)
	if err != nil {
		return ""
	}

	var notes []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		path := filepath.Join(researchDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		// Truncate long files
		if len(content) > llm.MaxResearchCharsPerNote {
			content = content[:llm.MaxResearchCharsPerNote] + "\n... (truncated)"
		}
		notes = append(notes, fmt.Sprintf("### %s\n%s", name, content))
	}
	return strings.Join(notes, "\n\n")
}

func languageDescs(lang, domain string) (langDesc, termsDesc string, hasCode bool) {
	isCS := strings.Contains(strings.ToLower(domain), "computer") ||
		strings.Contains(strings.ToLower(domain), "programming") ||
		strings.Contains(strings.ToLower(domain), "software") ||
		strings.Contains(strings.ToLower(domain), "tech")

	switch lang {
	case "ar":
		langDesc = "Modern Standard Arabic (MSA)"
		termsDesc = "English"
		hasCode = isCS
	default:
		langDesc = "English"
		termsDesc = "Arabic"
		hasCode = isCS
	}
	return
}
