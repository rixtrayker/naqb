package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/agents"
)

// InitFormResult holds the answers from the init interview form.
type InitFormResult struct {
	Answers agents.InterviewAnswers
	Done    bool
	Err     error
}

type initFormStep int

const (
	stepTemplate initFormStep = iota
	stepTitle
	stepAuthor
	stepLanguage
	stepDomain
	stepSynopsis
	stepNumChapters
)

var stepLabels = []string{
	"Template (1=Arabic Research  2=CS Book  3=General)",
	"Book title",
	"Author name",
	"Language (ar/en)",
	"Domain/subject (e.g. 'Arabic history', 'Computer Science')",
	"One-sentence synopsis",
	"Number of chapters",
}

var stepDefaults = []string{"3", "", "", "ar", "", "", "8"}

var (
	initTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	initQuestionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	initHintStyle     = lipgloss.NewStyle().Faint(true)
	initErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	initDoneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

// initFormModel is a multi-step text-input form.
type initFormModel struct {
	step   initFormStep
	inputs []textinput.Model
	err    string
	result *InitFormResult
}

// NewInitForm creates the Bubble Tea model for the init interview.
func NewInitForm() *initFormModel {
	inputs := make([]textinput.Model, len(stepLabels))
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = stepDefaults[i]
		t.CharLimit = 300
		inputs[i] = t
	}
	inputs[0].Focus()
	return &initFormModel{inputs: inputs}
}

func (m *initFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *initFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.result = &InitFormResult{Err: fmt.Errorf("cancelled")}
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.inputs[m.step].Value())
			if val == "" {
				val = stepDefaults[m.step]
			}

			if err := m.validate(m.step, val); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
			m.inputs[m.step].Blur()

			// If template selected, auto-fill language/domain from it
			if m.step == stepTemplate {
				m.applyTemplateDefaults(val)
			}

			m.step++

			if int(m.step) >= len(stepLabels) {
				m.result = &InitFormResult{
					Answers: m.collectAnswers(),
					Done:    true,
				}
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

func (m *initFormModel) applyTemplateDefaults(templateChoice string) {
	switch templateChoice {
	case "1": // arabic-research
		if m.inputs[stepLanguage].Value() == "" {
			m.inputs[stepLanguage].SetValue("ar")
		}
		if m.inputs[stepDomain].Value() == "" {
			m.inputs[stepDomain].Placeholder = "Arabic research and scholarship"
		}
	case "2": // cs-book
		if m.inputs[stepLanguage].Value() == "" {
			m.inputs[stepLanguage].SetValue("en")
		}
		if m.inputs[stepDomain].Value() == "" {
			m.inputs[stepDomain].Placeholder = "Computer Science and Software Engineering"
		}
	}
}

func (m *initFormModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(initTitleStyle.Render("نقب  New Book") + "\n\n")
	sb.WriteString(renderHintBar(InitFormBindings) + "\n\n")

	// Template picker hint
	if m.step == stepTemplate {
		sb.WriteString(initHintStyle.Render(
			"  Templates:\n" +
				"    1 — Arabic Research (كتاب بحثي) — RTL, Amiri font, scholarly\n" +
				"    2 — CS / Technical Book   — Code blocks, English, precise\n" +
				"    3 — General Book          — Flexible, fill in yourself\n",
		) + "\n")
	}

	for i, label := range stepLabels {
		if i < int(m.step) {
			val := m.inputs[i].Value()
			if val == "" {
				val = stepDefaults[i]
			}
			sb.WriteString(initDoneStyle.Render(fmt.Sprintf("  ✓ %s: %s", label, val)) + "\n")
		} else if i == int(m.step) {
			sb.WriteString(initQuestionStyle.Render(fmt.Sprintf("  → %s:", label)) + "\n")
			sb.WriteString("    " + m.inputs[i].View() + "\n")
			if stepDefaults[i] != "" && stepDefaults[i] != m.inputs[i].Value() {
				sb.WriteString(initHintStyle.Render(fmt.Sprintf("    (default: %s)", stepDefaults[i])) + "\n")
			}
		} else {
			sb.WriteString(initHintStyle.Render(fmt.Sprintf("  · %s", label)) + "\n")
		}
	}

	if m.err != "" {
		sb.WriteString("\n" + initErrorStyle.Render("  ✗ "+m.err) + "\n")
	}

	return sb.String()
}

func (m *initFormModel) validate(step initFormStep, val string) error {
	switch step {
	case stepTemplate:
		if val != "1" && val != "2" && val != "3" {
			return fmt.Errorf("choose 1, 2, or 3")
		}
	case stepTitle:
		if val == "" {
			return fmt.Errorf("title cannot be empty")
		}
	case stepLanguage:
		if val != "ar" && val != "en" {
			return fmt.Errorf("language must be 'ar' or 'en'")
		}
	case stepNumChapters:
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 50 {
			return fmt.Errorf("number of chapters must be between 1 and 50")
		}
	}
	return nil
}

func templateIDFromChoice(choice string) string {
	switch choice {
	case "1":
		return "arabic-research"
	case "2":
		return "cs-book"
	default:
		return "general"
	}
}

func (m *initFormModel) collectAnswers() agents.InterviewAnswers {
	get := func(i initFormStep) string {
		v := strings.TrimSpace(m.inputs[i].Value())
		if v == "" {
			return stepDefaults[i]
		}
		return v
	}
	n, _ := strconv.Atoi(get(stepNumChapters))
	if n == 0 {
		n = 8
	}
	return agents.InterviewAnswers{
		Template:    templateIDFromChoice(get(stepTemplate)),
		Title:       get(stepTitle),
		Author:      get(stepAuthor),
		Language:    get(stepLanguage),
		Domain:      get(stepDomain),
		Synopsis:    get(stepSynopsis),
		NumChapters: n,
	}
}

// RunInitForm runs the Bubble Tea init form and returns the answers.
func RunInitForm() (*InitFormResult, error) {
	m := NewInitForm()
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := finalModel.(*initFormModel)
	if fm.result == nil {
		return &InitFormResult{Err: fmt.Errorf("form exited unexpectedly")}, nil
	}
	return fm.result, nil
}
