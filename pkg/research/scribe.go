package research

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// buildFrontmatter constructs a YAML frontmatter block for a research note.
// It extracts the title from the first ## heading, a source URL from a
// "Source:" line or bare http URL, and tags based on the chapter number.
func buildFrontmatter(chapterNum int, content string) string {
	title := fmt.Sprintf("Chapter %d Research", chapterNum)
	source := ""

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Extract title from first ## heading
		if strings.HasPrefix(line, "## ") && title == fmt.Sprintf("Chapter %d Research", chapterNum) {
			title = strings.TrimPrefix(line, "## ")
			title = strings.TrimSpace(title)
		}
		// Extract source URL
		if source == "" {
			if after, ok := strings.CutPrefix(line, "Source:"); ok {
				source = strings.TrimSpace(after)
			} else if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				source = line
			}
		}
	}

	// Sanitize title for YAML (escape double quotes)
	title = strings.ReplaceAll(title, `"`, `'`)

	tag := fmt.Sprintf("ch-%02d", chapterNum)
	date := time.Now().Format("2006-01-02")

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", title))
	sb.WriteString(fmt.Sprintf("chapter: %d\n", chapterNum))
	sb.WriteString(fmt.Sprintf("tags: [research, %s]\n", tag))
	if source != "" {
		sb.WriteString(fmt.Sprintf("source: %q\n", source))
	}
	sb.WriteString(fmt.Sprintf("date: %q\n", date))
	sb.WriteString("---\n\n")
	return sb.String()
}

// Note is a synthesised atomic research note.
type Note struct {
	Filename string
	Content  string
}

// Scribe takes raw search results and synthesises atomic markdown notes
// using the LLM. Each note is saved to .naqb/research/ inside bookDir.
func Scribe(ctx context.Context, client llm.Provider, cfg *config.BookConfig, chapterNum int, raw []RawResult, bookDir string) ([]Note, error) {
	if len(raw) == 0 || countResults(raw) == 0 {
		log.Info("research scribe skipped: no raw results", "chapter", chapterNum)
		return nil, nil
	}

	log.Info("research scribe start", "chapter", chapterNum)

	system := `You are a research assistant synthesising web search results into concise atomic notes for a book author.
Write in clean Markdown. Each note should cover one key concept, fact, or source.
Keep each note focused and useful. Include the source URL as a reference.
Do NOT add preamble or post-amble — output only the note content.`

	rawText := FormatRaw(raw)
	if len(rawText) > 24000 {
		rawText = rawText[:24000] + "\n… (truncated)"
	}

	// Find chapter title
	chTitle := fmt.Sprintf("Chapter %d", chapterNum)
	for _, ch := range cfg.Chapters {
		if ch.Number == chapterNum {
			chTitle = fmt.Sprintf("Chapter %d: %s", ch.Number, ch.Title)
			break
		}
	}

	userMsg := fmt.Sprintf(`Book: %s | Domain: %s | Language: %s

Synthesise the following search results into useful atomic research notes for:
%s

Write 3–6 notes. Each note should start with a ## heading (the topic).

---
%s`, cfg.Title, cfg.Domain, cfg.Language, chTitle, rawText)

	model := cfg.LLM.WriteModel
	if model == "" {
		model = llm.ModelSonnet
	}

	resp, err := client.Complete(ctx, model, system, []llm.Message{
		{Role: "user", Content: userMsg},
	}, 4096)
	if err != nil {
		return nil, fmt.Errorf("scribe LLM failed: %w", err)
	}

	// Split LLM output by ## headings into individual notes
	notes := splitNotes(resp, chapterNum)

	// Save to .naqb/research/
	notesDir := filepath.Join(bookDir, ".naqb", "research")
	if err := os.MkdirAll(notesDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating research dir: %w", err)
	}

	ts := time.Now().Format("0102-150405") // MMDD-HHMMSS
	for i, note := range notes {
		fname := fmt.Sprintf("ch%02d-%s-%02d.md", chapterNum, ts, i+1)
		path := filepath.Join(notesDir, fname)
		// Prepend YAML frontmatter
		frontmatter := buildFrontmatter(chapterNum, note.Content)
		full := frontmatter + note.Content
		if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
			log.Warn("scribe: failed to write note", "path", path, "err", err)
			continue
		}
		notes[i].Filename = fname
		log.Debug("scribe: note saved", "file", fname)
	}

	log.Info("research scribe done", "chapter", chapterNum, "notes", len(notes))
	return notes, nil
}

// splitNotes splits a multi-note LLM response at ## headings.
func splitNotes(resp string, chapterNum int) []Note {
	var notes []Note
	sections := strings.Split(resp, "\n## ")
	for i, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}
		// Re-add the ## for all but the first (which may or may not have it)
		if i > 0 {
			sec = "## " + sec
		} else if !strings.HasPrefix(sec, "#") {
			sec = "## Research Note\n\n" + sec
		}
		notes = append(notes, Note{Content: sec})
	}
	if len(notes) == 0 && strings.TrimSpace(resp) != "" {
		notes = []Note{{Content: resp}}
	}
	return notes
}
