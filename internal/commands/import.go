package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/gdocs"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/tui"
)

// ImportCmd returns the `nqb import` command.
// With no subcommand, it launches the interactive TUI wizard.
func ImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import content from external sources into the book",
		Long: `Import content into the current book project.

Running 'nqb import' with no subcommand launches the interactive wizard.

Wizard import types:
  1 — notes      Copy file → .naqb/research/ with YAML frontmatter
  2 — draft      Replace chapters/ch-XX.md (with .bak backup)
  3 — template   Merge config file (book.yaml / rules.yaml / context.md)
  4 — to-outline Convert raw notes to outline.md via LLM

Subcommands:
  gdoc   Import from a Google Doc (requires COMPOSIO_API_KEY)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportWizard()
		},
	}
	cmd.AddCommand(importGDocCmd())
	return cmd
}

// runImportWizard launches the TUI form and executes the import.
func runImportWizard() error {
	bookDir, err := config.FindBookRoot()
	if err != nil {
		return err
	}
	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return err
	}

	result, err := tui.RunImportForm()
	if err != nil {
		return err
	}
	if result.Err != nil {
		fmt.Println("Import cancelled.")
		return nil
	}
	if !result.Done {
		return nil
	}

	// LLM client (needed only for to-outline)
	var client *llm.Client
	if result.Type == "to-outline" {
		apiKey, keyErr := config.APIKey()
		if keyErr != nil || apiKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not found — required for to-outline import")
		}
		client = llm.New(apiKey)
	}

	return execImport(context.Background(), result, bookDir, cfg, client)
}

// execImport dispatches the import operation based on wizard result.
func execImport(ctx context.Context, result *tui.ImportFormResult, bookDir string, cfg *config.BookConfig, client *llm.Client) error {
	switch result.Type {
	case "notes":
		dest, err := importAsNote(result.FilePath, bookDir)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Imported note → %s\n", dest)

	case "draft":
		if err := importAsDraft(result.FilePath, bookDir, cfg, result.ChapterNum); err != nil {
			return err
		}
		fmt.Printf("✓ Imported draft for chapter %d\n", result.ChapterNum)

	case "template":
		if err := mergeBookYAML(result.FilePath, bookDir, cfg); err != nil {
			return err
		}
		fmt.Printf("✓ Merged template config from %s\n", result.FilePath)

	case "to-outline":
		if err := importToOutline(ctx, client, result.FilePath, bookDir, cfg); err != nil {
			return err
		}
		fmt.Println("✓ outline.md updated from notes")

	default:
		return fmt.Errorf("unknown import type: %s", result.Type)
	}
	return nil
}

// importAsNote copies src to .naqb/research/ with YAML frontmatter.
func importAsNote(src, bookDir string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("importAsNote: reading %s: %w", src, err)
	}
	content := string(data)

	// Extract title from first ## heading
	title := filepath.Base(src)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			title = strings.TrimPrefix(line, "## ")
			break
		} else if strings.HasPrefix(line, "# ") {
			title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	date := time.Now().Format("2006-01-02")
	frontmatter := fmt.Sprintf("---\ntitle: %q\ntags: [research, imported, local]\nsource: %q\ndate: %q\n---\n\n",
		title, src, date)

	notesDir := filepath.Join(bookDir, ".naqb", "research")
	if err := os.MkdirAll(notesDir, 0o750); err != nil {
		return "", fmt.Errorf("importAsNote: creating dir: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	ts := time.Now().Format("0102-150405")
	fname := fmt.Sprintf("%s-%s.md", base, ts)
	dest := filepath.Join(notesDir, fname)

	if err := os.WriteFile(dest, []byte(frontmatter+content), 0o644); err != nil {
		return "", fmt.Errorf("importAsNote: writing note: %w", err)
	}
	return dest, nil
}

// importAsDraft replaces chapters/ch-XX.md with src (backing up the original).
func importAsDraft(src, bookDir string, cfg *config.BookConfig, chapterNum int) error {
	chaptersDir := filepath.Join(bookDir, "chapters")
	if err := os.MkdirAll(chaptersDir, 0o750); err != nil {
		return fmt.Errorf("importAsDraft: creating chapters dir: %w", err)
	}

	dest := filepath.Join(chaptersDir, config.ChapterFilename(chapterNum))

	// Backup existing chapter if it exists
	if _, err := os.Stat(dest); err == nil {
		backup := dest + ".bak"
		data, err := os.ReadFile(dest)
		if err != nil {
			return fmt.Errorf("importAsDraft: reading existing chapter: %w", err)
		}
		if err := os.WriteFile(backup, data, 0o644); err != nil {
			return fmt.Errorf("importAsDraft: writing backup: %w", err)
		}
		fmt.Printf("  Backed up existing chapter to %s\n", backup)
	}

	// Copy src → chapters/ch-XX.md
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("importAsDraft: reading source: %w", err)
	}
	if err := os.WriteFile(dest, srcData, 0o644); err != nil {
		return fmt.Errorf("importAsDraft: writing chapter: %w", err)
	}

	// Update chapter status in book.yaml
	for i, ch := range cfg.Chapters {
		if ch.Number == chapterNum {
			cfg.Chapters[i].Status = "imported"
			break
		}
	}
	if err := config.SaveBook(bookDir, cfg); err != nil {
		fmt.Printf("  ⚠ Warning: could not update book.yaml status: %v\n", err)
	}

	return nil
}

// mergeBookYAML fills zero-valued fields from src YAML into the book config.
// It NEVER overwrites Title, Author, or Chapters.
func mergeBookYAML(src, bookDir string, cfg *config.BookConfig) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("mergeBookYAML: reading %s: %w", src, err)
	}

	var srcCfg config.BookConfig
	if err := yaml.Unmarshal(srcData, &srcCfg); err != nil {
		return fmt.Errorf("mergeBookYAML: parsing source YAML: %w", err)
	}

	// Merge only zero-valued fields (never overwrite title/author/chapters)
	if cfg.Domain == "" && srcCfg.Domain != "" {
		cfg.Domain = srcCfg.Domain
	}
	if cfg.Synopsis == "" && srcCfg.Synopsis != "" {
		cfg.Synopsis = srcCfg.Synopsis
	}
	if cfg.Language == "" && srcCfg.Language != "" {
		cfg.Language = srcCfg.Language
	}
	if cfg.TargetWords == 0 && srcCfg.TargetWords != 0 {
		cfg.TargetWords = srcCfg.TargetWords
	}
	if cfg.LLM.WriteModel == "" && srcCfg.LLM.WriteModel != "" {
		cfg.LLM.WriteModel = srcCfg.LLM.WriteModel
	}
	if cfg.LLM.QAModel == "" && srcCfg.LLM.QAModel != "" {
		cfg.LLM.QAModel = srcCfg.LLM.QAModel
	}

	return config.SaveBook(bookDir, cfg)
}

// importToOutline converts raw notes to outline.md using the LLM.
func importToOutline(ctx context.Context, client *llm.Client, src, bookDir string, cfg *config.BookConfig) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("importToOutline: reading %s: %w", src, err)
	}

	systemPrompt := `Convert raw brainstorm notes into a clean book outline.
Format: # Book Outline, then ## Chapter N: Title with bullet subsections.
Preserve all key ideas. Output ONLY the outline.`

	userMsg := fmt.Sprintf("Book: %s\n\nRaw notes:\n%s", cfg.Title, string(data))

	model := cfg.LLM.WriteModel
	if model == "" {
		model = llm.ModelSonnet
	}

	outline, err := client.Complete(ctx, model, systemPrompt, []llm.Message{
		{Role: "user", Content: userMsg},
	}, 4096)
	if err != nil {
		return fmt.Errorf("importToOutline: LLM call: %w", err)
	}

	outlinePath := filepath.Join(bookDir, "outline.md")

	// Backup existing outline
	if _, err := os.Stat(outlinePath); err == nil {
		backup := outlinePath + ".bak"
		existing, _ := os.ReadFile(outlinePath)
		_ = os.WriteFile(backup, existing, 0o644)
		fmt.Printf("  Backed up existing outline.md → outline.md.bak\n")
	}

	if err := os.WriteFile(outlinePath, []byte(outline), 0o644); err != nil {
		return fmt.Errorf("importToOutline: writing outline: %w", err)
	}
	return nil
}

// ── Google Doc import (existing, unchanged) ─────────────────────────────────

func importGDocCmd() *cobra.Command {
	var asOutline bool
	var asNote bool
	var url string

	cmd := &cobra.Command{
		Use:   "gdoc",
		Short: "Import a Google Doc into the book as outline or research note",
		Long: `Fetches a Google Doc and imports its content into the book.

By default writes to research/ as a frontmatter-tagged note.
Use --as-outline to overwrite outline.md instead.

Examples:
  nqb import gdoc --url https://docs.google.com/document/d/DOC_ID/edit
  nqb import gdoc --url https://docs.google.com/document/d/DOC_ID/edit --as-outline`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				return fmt.Errorf("--url is required")
			}

			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			apiKey := config.ComposioAPIKey()
			if apiKey == "" {
				return fmt.Errorf("COMPOSIO_API_KEY not found in Keychain")
			}

			userID := cfg.Sync.ComposioUserID
			if userID == "" {
				userID = "pg-test-f3eaa561-6583-4190-9d84-06e15fd4b522"
			}

			docID := extractDocID(url)
			if docID == "" {
				return fmt.Errorf("could not extract document ID from URL: %s", url)
			}

			client := gdocs.New(apiKey, userID)
			ctx := context.Background()

			fmt.Printf("  Fetching Google Doc %s... ", docID)
			doc, err := client.GetDocument(ctx, docID)
			if err != nil {
				return fmt.Errorf("fetch doc: %w", err)
			}
			fmt.Printf("✓ \"%s\"\n", doc.Title)

			// Get the document content via export as plain text
			content, err := client.GetDocumentText(ctx, docID)
			if err != nil {
				return fmt.Errorf("get doc text: %w", err)
			}

			if asOutline {
				outPath := filepath.Join(bookDir, "outline.md")
				// Backup existing outline
				if _, err := os.Stat(outPath); err == nil {
					backup := filepath.Join(bookDir, "outline.md.bak")
					_ = os.Rename(outPath, backup)
					fmt.Printf("  Existing outline.md backed up to outline.md.bak\n")
				}
				if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
					return fmt.Errorf("writing outline.md: %w", err)
				}
				fmt.Printf("✓ Imported to outline.md (%d bytes)\n", len(content))
				return nil
			}

			// Default: save as research note with frontmatter
			notesDir := filepath.Join(bookDir, ".naqb", "research")
			if err := os.MkdirAll(notesDir, 0o750); err != nil {
				return fmt.Errorf("creating research dir: %w", err)
			}

			title := doc.Title
			if title == "" {
				title = "Imported from Google Docs"
			}
			date := time.Now().Format("2006-01-02")
			frontmatter := fmt.Sprintf("---\ntitle: %q\ntags: [research, imported, gdocs]\nsource: %q\ndate: %q\n---\n\n",
				title, url, date)

			ts := time.Now().Format("0102-150405")
			fname := fmt.Sprintf("gdoc-%s.md", ts)
			outPath := filepath.Join(notesDir, fname)
			if err := os.WriteFile(outPath, []byte(frontmatter+content), 0o644); err != nil {
				return fmt.Errorf("writing note: %w", err)
			}

			_ = asNote
			fmt.Printf("✓ Imported to .naqb/research/%s (%d bytes)\n", fname, len(content))
			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "Google Doc URL (required)")
	cmd.Flags().BoolVar(&asOutline, "as-outline", false, "Import as outline.md instead of a research note")
	cmd.Flags().BoolVar(&asNote, "as-note", false, "Import as research note (default)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

// extractDocID parses a Google Docs URL and returns the document ID.
// Handles formats: /document/d/ID/edit, /document/d/ID, etc.
func extractDocID(rawURL string) string {
	re := regexp.MustCompile(`/document/d/([a-zA-Z0-9_-]+)`)
	m := re.FindStringSubmatch(rawURL)
	if len(m) >= 2 {
		return m[1]
	}
	// Maybe it's just a bare ID
	if !strings.Contains(rawURL, "/") && len(rawURL) > 10 {
		return rawURL
	}
	return ""
}
