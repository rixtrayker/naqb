package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
	"github.com/amr/naqb/internal/pipeline"
	"github.com/amr/naqb/internal/tui"
	"github.com/amr/naqb/internal/vault"
)

// InitCmd returns the `nqb init` command.
func InitCmd() *cobra.Command {
	var noGit bool
	var dir string
	var inVault bool

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new book project via LLM interview",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				dir = args[0]
			}

			// Run the init form
			result, err := tui.RunInitForm()
			if err != nil {
				return fmt.Errorf("init form error: %w", err)
			}
			if result.Err != nil {
				return result.Err
			}
			if !result.Done {
				return fmt.Errorf("init cancelled")
			}

			// Override dir from flag; if --vault, place inside default vault
			if inVault && dir == "" {
				dir = filepath.Join(vault.DefaultVaultPath(), slugify(result.Answers.Title))
			}

			return runInitWithAnswersAt(result.Answers, dir, noGit)
		},
	}

	cmd.Flags().BoolVar(&noGit, "no-git", false, "Skip git initialization")
	cmd.Flags().StringVarP(&dir, "output", "o", "", "Output directory (default: cwd/slugified-title)")
	cmd.Flags().BoolVar(&inVault, "vault", false, "Create book inside the default vault (~/.naqb/projects/)")
	return cmd
}

// runInitWithAnswers is called from open.go after the home screen's N flow.
// It places the book in the default vault.
func runInitWithAnswers(answers agents.InterviewAnswers) error {
	dir := filepath.Join(vault.DefaultVaultPath(), slugify(answers.Title))
	return runInitWithAnswersAt(answers, dir, false)
}

func runInitWithAnswersAt(answers agents.InterviewAnswers, dir string, noGit bool) error {
	bookDir := dir
	if bookDir == "" {
		bookDir = slugify(answers.Title)
	}
	var err error
	bookDir, err = filepath.Abs(bookDir)
	if err != nil {
		return err
	}

	log.Info("init start", "title", answers.Title, "dir", bookDir, "template", answers.Template, "language", answers.Language)
	fmt.Printf("\nGenerating book plan with Claude Haiku...\n")

	apiKey, err := config.APIKey()
	if err != nil {
		return err
	}
	client := llm.New(apiKey)

	// Apply template overrides if language matches
	tmpl := config.TemplateByID(answers.Template)
	if tmpl != nil && answers.Language == "" {
		answers.Language = tmpl.Language
	}
	if tmpl != nil && answers.Domain == "" {
		answers.Domain = tmpl.Domain
	}

	planResult, err := agents.RunPlanner(context.Background(), client, answers)
	if err != nil {
		log.Error("planner failed", "err", err)
		return fmt.Errorf("planner failed: %w", err)
	}
	log.Info("planner complete", "chapters", len(planResult.BookConfig.Chapters))

	fmt.Printf("Creating project at: %s\n", bookDir)

	// Apply template's custom rules + prompts if available
	if tmpl != nil {
		planResult.BookConfig.Domain = tmpl.Domain
		if planResult.BookConfig.Language == "" {
			planResult.BookConfig.Language = tmpl.Language
		}
	}

	if err := config.InitBookDir(bookDir, planResult.BookConfig); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	// Override rules.yaml and prompts with template if provided
	if tmpl != nil {
		if err := writeTemplateFiles(bookDir, tmpl); err != nil {
			fmt.Printf("  (template customization warning: %v)\n", err)
		}
	}

	outlinePath := filepath.Join(bookDir, "outline.md")
	if err := os.WriteFile(outlinePath, []byte(planResult.OutlineMD), 0o644); err != nil {
		return fmt.Errorf("writing outline: %w", err)
	}

	if !noGit {
		if err := pipeline.GitInit(bookDir); err != nil {
			fmt.Printf("(git init failed: %v)\n", err)
		}
		if err := pipeline.GitCommit(bookDir, fmt.Sprintf("init: \"%s\" — book initialized", answers.Title)); err != nil {
			fmt.Printf("(initial commit failed: %v)\n", err)
		}
	}

	// Record in vault recents
	_ = vault.RecordRecent(bookDir, answers.Title, answers.Language)

	log.Info("init complete", "dir", bookDir, "chapters", len(planResult.BookConfig.Chapters))
	fmt.Printf("\n✓ Book project created!\n")
	fmt.Printf("  Directory:  %s\n", bookDir)
	fmt.Printf("  Chapters:   %d\n", len(planResult.BookConfig.Chapters))
	fmt.Printf("  Language:   %s\n", planResult.BookConfig.Language)
	if tmpl != nil {
		fmt.Printf("  Template:   %s\n", tmpl.Name)
	}
	fmt.Printf("\nOpen it with:  nqb open %s\n", slugify(answers.Title))
	fmt.Printf("Or directly:   nqb .\n")

	return nil
}

func writeTemplateFiles(bookDir string, tmpl *config.Template) error {
	rulesPath := filepath.Join(bookDir, "config", "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(tmpl.RulesYAML), 0o644); err != nil {
		return err
	}
	if tmpl.WritePrompt != "" {
		p := filepath.Join(bookDir, "config", "prompts", "write.md")
		if err := os.WriteFile(p, []byte(tmpl.WritePrompt), 0o644); err != nil {
			return err
		}
	}
	if tmpl.QAPrompt != "" {
		p := filepath.Join(bookDir, "config", "prompts", "qa.md")
		if err := os.WriteFile(p, []byte(tmpl.QAPrompt), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func slugify(s string) string {
	var result []rune
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, r)
		case r >= 'A' && r <= 'Z':
			result = append(result, r+32)
		case r >= '0' && r <= '9':
			result = append(result, r)
		case r == ' ' || r == '-' || r == '_':
			result = append(result, '-')
		}
	}
	if len(result) == 0 {
		return "my-book"
	}
	return string(result)
}
