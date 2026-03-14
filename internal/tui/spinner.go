package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

// SpinnerMsg is sent when the spinner task finishes.
type SpinnerMsg struct {
	Err error
}

// spinnerModel is a simple Bubble Tea model with a spinner.
type spinnerModel struct {
	spinner  spinner.Model
	label    string
	done     bool
	err      error
	taskDone <-chan SpinnerMsg
}

func newSpinnerModel(label string, taskDone <-chan SpinnerMsg) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinStyle
	return spinnerModel{
		spinner:  s,
		label:    label,
		taskDone: taskDone,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForTask(m.taskDone))
}

func waitForTask(ch <-chan SpinnerMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SpinnerMsg:
		m.done = true
		m.err = msg.Err
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("✗ %s — error: %v\n", m.label, m.err)
		}
		return fmt.Sprintf("✓ %s\n", m.label)
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.label)
}

// RunWithSpinner runs fn in a goroutine while showing a spinner.
// Returns any error from fn.
func RunWithSpinner(label string, fn func() error, out io.Writer) error {
	ch := make(chan SpinnerMsg, 1)
	go func() {
		err := fn()
		ch <- SpinnerMsg{Err: err}
	}()

	m := newSpinnerModel(label, ch)
	p := tea.NewProgram(m, tea.WithOutput(out))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(spinnerModel); ok && fm.err != nil {
		return fm.err
	}
	return nil
}
