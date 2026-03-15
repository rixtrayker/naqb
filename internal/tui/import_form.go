package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type importStep int

const (
	importStepType importStep = iota
	importStepFile
	importStepExtra // chapter num for draft, subtype for template (skipped for notes/to-outline)
)

var (
	importTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	importQuestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	importHintStyle     = lipgloss.NewStyle().Faint(true)
	importErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	importDoneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

// importFormModel is a multi-step wizard for the nqb import command.
type importFormModel struct {
	step   importStep
	inputs []textinput.Model
	err    string
	result *ImportFormResult

	// resolved state
	importType string // filled after step 0
}

// NewImportForm creates the import wizard model.
func NewImportForm() *importFormModel {
	inputs := make([]textinput.Model, 3)
	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 400
		inputs[i] = t
	}
	inputs[importStepType].Placeholder = "1"
	inputs[importStepFile].Placeholder = "brainstrom.md"
	inputs[importStepExtra].Placeholder = "1"
	inputs[importStepType].Focus()
	return &importFormModel{inputs: inputs}
}

func (m *importFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *importFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.result = &ImportFormResult{Err: fmt.Errorf("cancelled")}
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.inputs[m.step].Value())

			if err := m.validate(m.step, val); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
			m.inputs[m.step].Blur()

			// Record type after step 0
			if m.step == importStepType {
				m.importType = importTypeFromChoice(val)
			}

			m.step++

			// Determine if we need the extra step
			if int(m.step) == int(importStepExtra) {
				// Skip extra step for "notes" and "to-outline"
				if m.importType == "notes" || m.importType == "to-outline" {
					m.result = m.collectResult()
					return m, tea.Quit
				}
				// For draft: placeholder is "chapter number"
				// For template: placeholder is "1=book.yaml 2=rules.yaml 3=context.md 4=all"
				switch m.importType {
				case "draft":
					m.inputs[importStepExtra].Placeholder = "chapter number (e.g. 3)"
				case "template":
					m.inputs[importStepExtra].Placeholder = "1=book.yaml  2=rules.yaml  3=context.md  4=all"
				}
				m.inputs[importStepExtra].Focus()
				return m, textinput.Blink
			}

			if int(m.step) > int(importStepExtra) {
				m.result = m.collectResult()
				return m, tea.Quit
			}

			m.inputs[m.step].Focus()
			return m, textinput.Blink
		}
	}

	if int(m.step) < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *importFormModel) validate(step importStep, val string) error {
	switch step {
	case importStepType:
		if val == "" {
			val = "1"
		}
		if val != "1" && val != "2" && val != "3" && val != "4" {
			return fmt.Errorf("choose 1 (notes), 2 (draft), 3 (template), or 4 (to-outline)")
		}
	case importStepFile:
		if val == "" {
			return fmt.Errorf("file path cannot be empty")
		}
		resolved := resolveFilePath(val)
		if resolved == "" {
			return fmt.Errorf("file not found: %q", val)
		}
	case importStepExtra:
		if m.importType == "draft" {
			if val == "" {
				return fmt.Errorf("chapter number is required for draft import")
			}
			for _, c := range val {
				if c < '0' || c > '9' {
					return fmt.Errorf("chapter number must be a positive integer")
				}
			}
		} else if m.importType == "template" {
			if val != "1" && val != "2" && val != "3" && val != "4" {
				return fmt.Errorf("choose 1–4")
			}
		}
	}
	return nil
}

func (m *importFormModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(importTitleStyle.Render("نقب  Import") + "\n\n")
	sb.WriteString(renderHintBar(ImportFormBindings) + "\n\n")

	labels := []string{
		"Import type  (1=notes  2=draft  3=template  4=to-outline)",
		"File path (or filename — will be searched automatically)",
		m.extraLabel(),
	}

	stepsShown := 3
	if m.importType == "notes" || m.importType == "to-outline" {
		stepsShown = 2
	}

	for i := 0; i < stepsShown; i++ {
		label := labels[i]
		step := importStep(i)

		if int(step) < int(m.step) {
			val := m.inputs[i].Value()
			if val == "" {
				val = m.inputs[i].Placeholder
			}
			sb.WriteString(importDoneStyle.Render(fmt.Sprintf("  ✓ %s: %s", label, val)) + "\n")
		} else if step == m.step {
			sb.WriteString(importQuestionStyle.Render(fmt.Sprintf("  → %s:", label)) + "\n")
			sb.WriteString("    " + m.inputs[i].View() + "\n")
		} else {
			sb.WriteString(importHintStyle.Render(fmt.Sprintf("  · %s", label)) + "\n")
		}
	}

	if m.step == importStepType {
		sb.WriteString("\n")
		sb.WriteString(importHintStyle.Render(
			"  Types:\n"+
				"    1 — notes      Copy file → .naqb/research/ with frontmatter\n"+
				"    2 — draft      Replace chapters/ch-XX.md (with backup)\n"+
				"    3 — template   Merge config file into book config\n"+
				"    4 — to-outline Convert notes to outline.md via LLM\n",
		) + "\n")
	}

	if m.err != "" {
		sb.WriteString("\n" + importErrorStyle.Render("  ✗ "+m.err) + "\n")
	}

	return sb.String()
}

func (m *importFormModel) extraLabel() string {
	switch m.importType {
	case "draft":
		return "Chapter number"
	case "template":
		return "Config subtype (1=book.yaml  2=rules.yaml  3=context.md  4=all)"
	default:
		return ""
	}
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

func (m *importFormModel) collectResult() *ImportFormResult {
	fileVal := strings.TrimSpace(m.inputs[importStepFile].Value())
	resolved := resolveFilePath(fileVal)
	if resolved == "" {
		resolved = fileVal
	}

	result := &ImportFormResult{
		Type:     m.importType,
		FilePath: resolved,
		Done:     true,
	}

	if m.importType == "draft" {
		extraVal := strings.TrimSpace(m.inputs[importStepExtra].Value())
		n := 0
		for _, c := range extraVal {
			n = n*10 + int(c-'0')
		}
		result.ChapterNum = n
	} else if m.importType == "template" {
		extraVal := strings.TrimSpace(m.inputs[importStepExtra].Value())
		switch extraVal {
		case "1":
			result.SubType = "book.yaml"
		case "2":
			result.SubType = "rules.yaml"
		case "3":
			result.SubType = "context.md"
		default:
			result.SubType = "all"
		}
	}

	return result
}

// resolveFilePath resolves user input to an absolute file path.
// Order: absolute path → relative-to-cwd → filepath.Glob → recursive basename scan.
func resolveFilePath(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Absolute path
	if filepath.IsAbs(input) {
		if _, err := os.Stat(input); err == nil {
			return input
		}
		return ""
	}

	// Relative to cwd
	cwd, _ := os.Getwd()
	rel := filepath.Join(cwd, input)
	if _, err := os.Stat(rel); err == nil {
		return rel
	}

	// Glob pattern (e.g. "*.md" or "brainstrom*")
	matches, err := filepath.Glob(filepath.Join(cwd, input))
	if err == nil && len(matches) > 0 {
		return matches[0]
	}

	// Recursive basename scan — walk up to 3 directory levels
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

// RunImportForm runs the Bubble Tea import wizard and returns the result.
func RunImportForm() (*ImportFormResult, error) {
	m := NewImportForm()
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := finalModel.(*importFormModel)
	if fm.result == nil {
		return &ImportFormResult{Err: fmt.Errorf("form exited unexpectedly")}, nil
	}
	return fm.result, nil
}
