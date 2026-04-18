// Package components provides reusable Bubble Tea widgets for the nqb TUI.
package components

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/amr/naqb/internal/tui/theme"
)

var spinStyle = lipgloss.NewStyle().Foreground(theme.ColorPrimary)
var spinTimerStyle = lipgloss.NewStyle().Faint(true)

// SpinnerMsg is sent when the spinner task finishes.
type SpinnerMsg struct {
	Err error
}

// tickMsg is sent every second to update the elapsed timer.
type tickMsg time.Time

// SpinnerModel is a simple Bubble Tea model with a spinner and elapsed timer.
type SpinnerModel struct {
	spinner  spinner.Model
	label    string
	done     bool
	err      error
	taskDone <-chan SpinnerMsg
	startAt  time.Time
	elapsed  time.Duration
}

// NewSpinner creates a new spinner component.
func NewSpinner(label string, taskDone <-chan SpinnerMsg) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinStyle
	return SpinnerModel{
		spinner:  s,
		label:    label,
		taskDone: taskDone,
		startAt:  time.Now(),
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForTask(m.taskDone), tickEvery())
}

func waitForTask(ch <-chan SpinnerMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SpinnerMsg:
		m.done = true
		m.err = msg.Err
		m.elapsed = time.Since(m.startAt)
		return m, tea.Quit
	case tickMsg:
		m.elapsed = time.Since(m.startAt)
		return m, tickEvery()
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

func (m SpinnerModel) View() string {
	if m.done {
		dur := formatDuration(m.elapsed)
		if m.err != nil {
			return fmt.Sprintf("✗ %s — error: %v %s\n", m.label, m.err, spinTimerStyle.Render("("+dur+")"))
		}
		return fmt.Sprintf("✓ %s %s\n", m.label, spinTimerStyle.Render("— "+dur))
	}
	timer := ""
	if m.elapsed >= time.Second {
		timer = spinTimerStyle.Render(" (" + formatDuration(m.elapsed) + ")")
	}
	return fmt.Sprintf("%s %s%s\n", m.spinner.View(), m.label, timer)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}

// RunWithSpinner runs fn in a goroutine while showing a spinner with elapsed time.
// Returns any error from fn.
func RunWithSpinner(label string, fn func() error, out io.Writer) error {
	ch := make(chan SpinnerMsg, 1)
	go func() {
		err := fn()
		ch <- SpinnerMsg{Err: err}
	}()

	m := NewSpinner(label, ch)
	p := tea.NewProgram(m, tea.WithOutput(out))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(SpinnerModel); ok && fm.err != nil {
		return fm.err
	}
	return nil
}
