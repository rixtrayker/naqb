package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Chapter represents a single chapter in the book's table of contents.
type Chapter struct {
	Number  int    `yaml:"number"`
	Title   string `yaml:"title"`
	File    string `yaml:"file"`
	Summary string `yaml:"summary,omitempty"`
	Status  string `yaml:"status,omitempty"` // pending, written, reviewed
}

// LLMSettings holds per-book LLM preferences.
type LLMSettings struct {
	WriteModel string `yaml:"write_model,omitempty"`
	QAModel    string `yaml:"qa_model,omitempty"`
	ChatModel  string `yaml:"chat_model,omitempty"`
	InitModel  string `yaml:"init_model,omitempty"`
}

// BookConfig is the book.yaml manifest.
type BookConfig struct {
	Title       string      `yaml:"title"`
	Author      string      `yaml:"author"`
	Language    string      `yaml:"language"`   // "ar" or "en"
	Domain      string      `yaml:"domain"`     // e.g. "Arabic culture", "Computer Science"
	Synopsis    string      `yaml:"synopsis"`
	TargetWords int         `yaml:"target_words,omitempty"` // per chapter
	Chapters    []Chapter   `yaml:"chapters"`
	LLM         LLMSettings `yaml:"llm,omitempty"`
	CreatedAt   time.Time   `yaml:"created_at,omitempty"`
	Version     string      `yaml:"version,omitempty"`
}

const bookYAMLName = "book.yaml"

// FindBookRoot walks up from cwd to find a directory containing book.yaml.
func FindBookRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, bookYAMLName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no book.yaml found in current directory or any parent")
}

// LoadBook reads the book.yaml from a given directory (or CWD if empty).
func LoadBook(dir string) (*BookConfig, error) {
	if dir == "" {
		var err error
		dir, err = FindBookRoot()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, bookYAMLName))
	if err != nil {
		return nil, fmt.Errorf("reading book.yaml: %w", err)
	}
	cfg := &BookConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing book.yaml: %w", err)
	}
	return cfg, nil
}

// SaveBook writes book.yaml into the given directory.
func SaveBook(dir string, cfg *BookConfig) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling book.yaml: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, bookYAMLName), data, 0o644)
}

// InitBookDir creates the full project layout in dir.
func InitBookDir(dir string, cfg *BookConfig) error {
	dirs := []string{
		filepath.Join(dir, "chapters"),
		filepath.Join(dir, "contexts"),
		filepath.Join(dir, "research"),
		filepath.Join(dir, "assets", "themes"),
		filepath.Join(dir, "output"),
		filepath.Join(dir, "config", "prompts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return err
		}
	}

	// Write book.yaml
	if err := SaveBook(dir, cfg); err != nil {
		return err
	}

	// Write default rules.yaml
	if err := writeDefaultRules(filepath.Join(dir, "config", "rules.yaml"), cfg.Language); err != nil {
		return err
	}

	// Write default prompts
	if err := writeDefaultPrompts(filepath.Join(dir, "config", "prompts")); err != nil {
		return err
	}

	// Write .gitignore
	gitignore := "output/\ncontexts/\n*.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return err
	}

	return nil
}

func writeDefaultRules(path, lang string) error {
	fontArabic := "Amiri"
	if lang != "ar" {
		fontArabic = "Liberation Serif"
	}
	content := fmt.Sprintf(`# Book formatting and QA rules

language: %s

word_count:
  min: 1500
  max: 5000
  target: 3000

formatting:
  line_height: 1.6
  font_arabic: "%s"
  font_latin: "IBM Plex Sans"
  font_code: "JetBrains Mono"
  code_theme: "dracula"
  callouts:
    note:    { prefix: "[!]", bg: "#FFF9C4" }
    deep:    { prefix: "[?]", bg: "#BBDEFB" }
    warning: { prefix: "[X]", bg: "#FFCDD2" }
  margins: "normal"

export:
  pdf: true
  epub: true
  docx: false
  web: false
  pdf_engine: xelatex
  rtl: %v

glossary: {}
`, lang, fontArabic, lang == "ar")
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeDefaultPrompts(dir string) error {
	prompts := map[string]string{
		"init.md": `You are an expert book planner and editorial consultant.
Your job is to interview the author and help them create a structured book plan.
Ask clear, focused questions about their book idea, target audience, and goals.
After gathering information, produce a structured book.yaml and outline.md.
Be encouraging, professional, and creative.`,

		"write.md": `You are an expert Technical Author specializing in the book's domain.
Write rich, detailed, ADHD-friendly content following these rules:
- Use frequent subheadings (H2, H3) to break up text
- Bold keywords and important concepts on first use
- Use callout blocks: [!] for notes, [?] for deep dives, [X] for warnings
- Include concrete examples, analogies, and code snippets where relevant
- Write in a clear, engaging style appropriate for the target audience
- Follow the chapter outline provided
- Start directly with the first heading — no preamble or meta-commentary`,

		"qa.md": `You are a professional book editor and QA reviewer.
Review the provided chapter against the book's rules and previous chapters.
Check for:
- Consistency with previous chapter summaries and the book's tone/style
- ADHD-friendly formatting (subheadings, bold keywords, callouts)
- Coverage of all outline points
- Language/terminology consistency with the glossary
- Appropriate depth and detail for the target audience
Provide specific, actionable feedback. Rate each dimension 1-5.`,
	}

	for name, content := range prompts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ChapterFilename returns the canonical filename for chapter N.
func ChapterFilename(n int) string {
	return fmt.Sprintf("ch-%02d.md", n)
}

// ContextFilename returns the canonical context filename for chapter N.
func ContextFilename(n int) string {
	return fmt.Sprintf("ch-%02d-context.md", n)
}
