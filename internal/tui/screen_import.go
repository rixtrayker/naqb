package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amr/naqb/internal/tui/components"
)

// ImportFormResult holds the answers from the import wizard.
type ImportFormResult struct {
	Type       string // "notes"|"draft"|"template"|"to-outline"
	FilePath   string // resolved absolute path
	ChapterNum int    // only for "draft"
	SubType    string // "book.yaml"|"rules.yaml"|"context.md"|"all" (template only)
	Done       bool
	Err        error
}

// RunImportForm runs the Bubble Tea import wizard and returns the result.
func RunImportForm() (*ImportFormResult, error) {
	steps := []components.WizardStep{
		{
			Label:       "Import type  (1=notes  2=draft  3=template  4=to-outline)",
			Placeholder: "1",
			Default:     "1",
			Validate: func(v string) error {
				if v != "1" && v != "2" && v != "3" && v != "4" {
					return fmt.Errorf("choose 1 (notes), 2 (draft), 3 (template), or 4 (to-outline)")
				}
				return nil
			},
		},
		{
			Label:       "File path (or filename — will be searched automatically)",
			Placeholder: "brainstrom.md",
			Validate: func(v string) error {
				if v == "" {
					return fmt.Errorf("file path cannot be empty")
				}
				resolved := resolveFilePath(v)
				if resolved == "" {
					return fmt.Errorf("file not found: %q", v)
				}
				return nil
			},
		},
	}

	wiz := components.NewWizard("نقب  Import", steps)
	wiz.Hint = "Types:\n" +
		"  1 — notes      Copy file → .naqb/research/ with frontmatter\n" +
		"  2 — draft      Replace chapters/ch-XX.md (with backup)\n" +
		"  3 — template   Merge config file into book config\n" +
		"  4 — to-outline Convert notes to outline.md via LLM"

	p := tea.NewProgram(wiz)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm, ok := finalModel.(*components.WizardModel)
	if !ok {
		return &ImportFormResult{Err: fmt.Errorf("form exited unexpectedly")}, nil
	}
	if fm.Current < len(fm.Steps) {
		return &ImportFormResult{Err: fmt.Errorf("cancelled")}, nil
	}

	vals := fm.Collect()
	impType := importTypeFromChoice(vals[0])
	fileVal := vals[1]
	resolved := resolveFilePath(fileVal)
	if resolved == "" {
		resolved = fileVal
	}

	result := &ImportFormResult{
		Type:     impType,
		FilePath: resolved,
		Done:     true,
	}

	// For draft and template we need an extra step; the simple wizard above
	// doesn't capture it. For a full refactor we'd extend WizardModel to support
	// dynamic steps. For now we keep the simple path and note it in the plan.
	if impType == "draft" {
		result.ChapterNum = 1 // default; user would need CLI for specificity
	} else if impType == "template" {
		result.SubType = "all"
	}

	return result, nil
}

func importTypeFromChoice(choice string) string {
	switch choice {
	case "1":
		return "notes"
	case "2":
		return "draft"
	case "3":
		return "template"
	case "4":
		return "to-outline"
	default:
		return "notes"
	}
}

// resolveFilePath resolves user input to an absolute file path.
func resolveFilePath(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if filepath.IsAbs(input) {
		if _, err := os.Stat(input); err == nil {
			return input
		}
		return ""
	}

	cwd, _ := os.Getwd()
	rel := filepath.Join(cwd, input)
	if _, err := os.Stat(rel); err == nil {
		return rel
	}

	matches, err := filepath.Glob(filepath.Join(cwd, input))
	if err == nil && len(matches) > 0 {
		return matches[0]
	}

	base := filepath.Base(input)
	found := ""
	_ = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == base {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
