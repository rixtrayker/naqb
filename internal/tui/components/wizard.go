package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/amr/naqb/internal/tui/theme"
)

// WizardStep represents one step in a multi-step form.
type WizardStep struct {
	Label       string
	Placeholder string
	Default     string
	Validate    func(string) error
}

// WizardModel is a reusable multi-step text-input form.
type WizardModel struct {
	Steps      []WizardStep
	Inputs     []textinput.Model
	Current    int
	Err        string
	Title      string
	Hint       string
	ExtraHints map[int]string // step index -> extra hint text
}

// NewWizard creates a wizard with the given steps.
func NewWizard(title string, steps []WizardStep) *WizardModel {
	inputs := make([]textinput.Model, len(steps))
	for i, s := range steps {
		t := textinput.New()
		t.Placeholder = s.Placeholder
		t.CharLimit = 400
		inputs[i] = t
	}
	inputs[0].Focus()

	return &WizardModel{
		Steps:      steps,
		Inputs:     inputs,
		Title:      title,
		ExtraHints: make(map[int]string),
	}
}

func (m *WizardModel) Init() tea.Cmd {
	return textinput.Blink
}

// WizardResult is returned when the wizard finishes or is cancelled.
type WizardResult struct {
	Done   bool
	Values []string
	Err    error
}

// Update handles navigation through the wizard steps.
func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.Inputs[m.Current].Value())
			if val == "" {
				val = m.Steps[m.Current].Default
			}
			if m.Steps[m.Current].Validate != nil {
				if err := m.Steps[m.Current].Validate(val); err != nil {
					m.Err = err.Error()
					return m, nil
				}
			}
			m.Err = ""
			m.Inputs[m.Current].Blur()
			m.Current++
			if m.Current >= len(m.Steps) {
				return m, tea.Quit
			}
			m.Inputs[m.Current].Focus()
			return m, textinput.Blink
		}
	}

	if m.Current < len(m.Inputs) {
		var cmd tea.Cmd
		m.Inputs[m.Current], cmd = m.Inputs[m.Current].Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the wizard UI.
func (m *WizardModel) View() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary)
	questionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	hintStyle := lipgloss.NewStyle().Faint(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	doneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render(m.Title) + "\n\n")
	sb.WriteString(theme.RenderHintBar([]theme.Binding{{Key: "Enter", Desc: "Confirm"}, {Key: "Ctrl+C", Desc: "Cancel"}}) + "\n\n")

	if hint, ok := m.ExtraHints[m.Current]; ok && hint != "" {
		sb.WriteString(hintStyle.Render(hint) + "\n")
	}

	for i, step := range m.Steps {
		if i < m.Current {
			val := m.Inputs[i].Value()
			if val == "" {
				val = step.Default
			}
			sb.WriteString(doneStyle.Render(fmt.Sprintf("  ✓ %s: %s", step.Label, val)) + "\n")
		} else if i == m.Current {
			sb.WriteString(questionStyle.Render(fmt.Sprintf("  → %s:", step.Label)) + "\n")
			sb.WriteString("    " + m.Inputs[i].View() + "\n")
			if step.Default != "" && step.Default != m.Inputs[i].Value() {
				sb.WriteString(hintStyle.Render(fmt.Sprintf("    (default: %s)", step.Default)) + "\n")
			}
		} else {
			sb.WriteString(hintStyle.Render(fmt.Sprintf("  · %s", step.Label)) + "\n")
		}
	}

	if m.Err != "" {
		sb.WriteString("\n" + errStyle.Render("  ✗ "+m.Err) + "\n")
	}

	return sb.String()
}

// Collect returns the final values from all steps.
func (m *WizardModel) Collect() []string {
	values := make([]string, len(m.Steps))
	for i, step := range m.Steps {
		v := strings.TrimSpace(m.Inputs[i].Value())
		if v == "" {
			v = step.Default
		}
		values[i] = v
	}
	return values
}
