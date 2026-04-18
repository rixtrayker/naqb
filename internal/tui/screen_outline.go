package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
)

var (
	oeTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorSecondary).
		PaddingLeft(1)

	oeActiveRow = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			PaddingLeft(1).PaddingRight(1)

	oeNormalRow = lipgloss.NewStyle().
			PaddingLeft(2)

	oeEditingRow = lipgloss.NewStyle().
			Background(lipgloss.Color("52")).
			Foreground(lipgloss.Color("230")).
			PaddingLeft(1)

	oeSaved = lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		PaddingLeft(2)
)

// outlineEditField identifies which field is being edited.
type outlineEditField int

const (
	editNone outlineEditField = iota
	editTitle
	editSummary
)

// outlineEditorModel is the Bubble Tea model for the outline editor.
type outlineEditorModel struct {
	bookDir     string
	cfg         *config.BookConfig
	chapters    []config.Chapter // working copy
	cursor      int
	editField   outlineEditField
	editInput   textinput.Model
	width       int
	height      int
	dirty       bool
	statusMsg   string
	statusTicks int
}

// NewOutlineEditor creates the outline editor model.
func NewOutlineEditor(bookDir string, cfg *config.BookConfig) *outlineEditorModel {
	chapters := make([]config.Chapter, len(cfg.Chapters))
	copy(chapters, cfg.Chapters)

	ei := textinput.New()
	ei.CharLimit = 300

	return &outlineEditorModel{
		bookDir:   bookDir,
		cfg:       cfg,
		chapters:  chapters,
		editInput: ei,
	}
}

func (m *outlineEditorModel) Init() tea.Cmd {
	return nil
}

func (m *outlineEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.editField != editNone {
			return m.updateEditing(msg)
		}
		return m.updateNavigating(msg)
	}

	if m.editField != editNone {
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *outlineEditorModel) updateNavigating(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.statusTicks > 0 {
		m.statusTicks--
		if m.statusTicks == 0 {
			m.statusMsg = ""
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.chapters)-1 {
			m.cursor++
		}

	case "t", "enter":
		if m.cursor >= len(m.chapters) {
			return m, nil
		}
		m.editField = editTitle
		m.editInput.Placeholder = "Chapter title..."
		m.editInput.SetValue(m.chapters[m.cursor].Title)
		m.editInput.Focus()
		m.editInput.CursorEnd()
		return m, textinput.Blink

	case "s":
		if m.cursor >= len(m.chapters) {
			return m, nil
		}
		m.editField = editSummary
		m.editInput.Placeholder = "Chapter summary..."
		m.editInput.SetValue(m.chapters[m.cursor].Summary)
		m.editInput.Focus()
		m.editInput.CursorEnd()
		return m, textinput.Blink

	case "U", "ctrl+up":
		if m.cursor > 0 {
			m.chapters[m.cursor], m.chapters[m.cursor-1] = m.chapters[m.cursor-1], m.chapters[m.cursor]
			m.renumber()
			m.cursor--
			m.dirty = true
		}

	case "D", "ctrl+down":
		if m.cursor < len(m.chapters)-1 {
			m.chapters[m.cursor], m.chapters[m.cursor+1] = m.chapters[m.cursor+1], m.chapters[m.cursor]
			m.renumber()
			m.cursor++
			m.dirty = true
		}

	case "ctrl+s", "S":
		if err := m.save(); err != nil {
			m.statusMsg = bvStatusErr.Render("Save failed: " + err.Error())
		} else {
			m.dirty = false
			m.statusMsg = oeSaved.Render("✓ Saved to book.yaml and outline.md")
		}
		m.statusTicks = 10
	}
	return m, nil
}

func (m *outlineEditorModel) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.editField = editNone
		m.editInput.Blur()
		return m, nil

	case tea.KeyEnter:
		val := strings.TrimSpace(m.editInput.Value())
		if val != "" {
			switch m.editField {
			case editTitle:
				m.chapters[m.cursor].Title = val
			case editSummary:
				m.chapters[m.cursor].Summary = val
			}
			m.dirty = true
		}
		m.editField = editNone
		m.editInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m *outlineEditorModel) renumber() {
	for i := range m.chapters {
		m.chapters[i].Number = i + 1
		m.chapters[i].File = config.ChapterFilename(i + 1)
	}
}

func (m *outlineEditorModel) save() error {
	m.cfg.Chapters = make([]config.Chapter, len(m.chapters))
	copy(m.cfg.Chapters, m.chapters)

	if err := config.SaveBook(m.bookDir, m.cfg); err != nil {
		return err
	}

	outline := buildOutlineMD(m.cfg)
	outlinePath := filepath.Join(m.bookDir, "outline.md")
	return os.WriteFile(outlinePath, []byte(outline), 0o644)
}

func buildOutlineMD(cfg *config.BookConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s — Outline\n\n", cfg.Title))
	sb.WriteString(fmt.Sprintf("**Author:** %s\n\n", cfg.Author))
	sb.WriteString(fmt.Sprintf("**Synopsis:** %s\n\n", cfg.Synopsis))
	sb.WriteString("---\n\n")
	for _, ch := range cfg.Chapters {
		sb.WriteString(fmt.Sprintf("## Chapter %d: %s\n\n", ch.Number, ch.Title))
		if ch.Summary != "" {
			sb.WriteString(ch.Summary + "\n\n")
		}
	}
	return sb.String()
}

func (m *outlineEditorModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(oeTitle.Render("Outline Editor — "+m.cfg.Title) + "\n\n")

	maxVisible := m.height - 10
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.chapters) {
		end = len(m.chapters)
	}

	for i := start; i < end; i++ {
		ch := m.chapters[i]
		active := i == m.cursor

		titleW := m.width - 20
		if titleW < 20 {
			titleW = 20
		}
		title := ch.Title
		if len(title) > titleW {
			title = title[:titleW-3] + "..."
		}
		summary := ch.Summary
		if len(summary) > titleW {
			summary = summary[:titleW-3] + "..."
		}

		if active && m.editField != editNone {
			label := "Title"
			if m.editField == editSummary {
				label = "Summary"
			}
			row := fmt.Sprintf("Ch%02d  [%s] %s", ch.Number, label, m.editInput.View())
			b.WriteString(oeEditingRow.Render(row) + "\n")
		} else {
			row := fmt.Sprintf("Ch%02d  %-*s  %s",
				ch.Number, titleW, title,
				lipgloss.NewStyle().Faint(true).Render(summary))
			if active {
				b.WriteString(oeActiveRow.Render(row) + "\n")
			} else {
				b.WriteString(oeNormalRow.Render(row) + "\n")
			}
		}
	}

	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(m.statusMsg + "\n")
	}

	footer := theme.RenderHintBar(keys.OutlineEditorBindings)
	if m.dirty {
		footer += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("● unsaved")
	}
	b.WriteString(footer + "\n")

	return b.String()
}

// RunOutlineEditor launches the outline editor TUI.
func RunOutlineEditor(bookDir string, cfg *config.BookConfig) error {
	m := NewOutlineEditor(bookDir, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
