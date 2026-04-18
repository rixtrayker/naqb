package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
)

var (
	previewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99")).
				PaddingLeft(1)
)

type previewModel struct {
	vp     viewport.Model
	title  string
	width  int
	height int
	ready  bool
}

func (m *previewModel) Init() tea.Cmd { return nil }

func (m *previewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.vp = viewport.New(msg.Width-4, msg.Height-4)
			m.ready = true
		} else {
			m.vp.Width = msg.Width - 4
			m.vp.Height = msg.Height - 4
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "g":
			m.vp.GotoTop()
			return m, nil
		case "G":
			m.vp.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *previewModel) View() string {
	if !m.ready {
		return "\nLoading preview..."
	}
	pct := int(m.vp.ScrollPercent() * 100)
	scrollInfo := lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf(" %d%%", pct))
	title := previewTitleStyle.Render("Preview — "+m.title) + scrollInfo
	footer := theme.RenderHintBar(keys.PreviewBindings)
	return fmt.Sprintf("%s\n%s\n%s", title, m.vp.View(), footer)
}

// renderPreview renders a chapter using glamour and returns the rendered string.
func renderPreview(bookDir string, cfg *config.BookConfig, chNum int) (string, error) {
	chapPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chNum))
	data, err := os.ReadFile(chapPath)
	if err != nil {
		return "", fmt.Errorf("chapter %d not written yet (file not found)", chNum)
	}

	style := "dark"
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return string(data), nil
	}
	rendered, err := renderer.Render(string(data))
	if err != nil {
		return string(data), nil
	}
	return rendered, nil
}

// RunPreview opens a full-screen scrollable preview of a chapter.
func RunPreview(bookDir string, cfg *config.BookConfig, chNum int) error {
	chapPath := filepath.Join(bookDir, "chapters", config.ChapterFilename(chNum))
	data, err := os.ReadFile(chapPath)
	if err != nil {
		return fmt.Errorf("chapter %d not written yet", chNum)
	}

	chTitle := fmt.Sprintf("Chapter %d", chNum)
	for _, ch := range cfg.Chapters {
		if ch.Number == chNum {
			chTitle = fmt.Sprintf("Ch%02d: %s", ch.Number, ch.Title)
			break
		}
	}

	isRTL := cfg.Language == "ar"

	style := "dark"
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(80),
	)
	var rendered string
	if err == nil {
		rendered, err = renderer.Render(string(data))
	}
	if err != nil {
		rendered = string(data)
	}

	if isRTL {
		rendered = lipgloss.NewStyle().Faint(true).Render(
			"[Note: RTL rendering in terminal is limited]\n\n",
		) + "\u2068" + rendered + "\u2069"
	}

	m := &previewModel{title: chTitle}
	p := tea.NewProgram(
		&previewRunner{inner: m, content: rendered},
		tea.WithAltScreen(),
	)
	_, runErr := p.Run()
	return runErr
}

// previewRunner wraps previewModel to set content after init.
type previewRunner struct {
	inner   *previewModel
	content string
	set     bool
}

func (r *previewRunner) Init() tea.Cmd { return r.inner.Init() }

func (r *previewRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := r.inner.Update(msg)
	if pm, ok := m.(*previewModel); ok {
		r.inner = pm
	}
	if r.inner.ready && !r.set {
		r.inner.vp.SetContent(r.content)
		r.set = true
	}
	return r, cmd
}

func (r *previewRunner) View() string {
	return r.inner.View()
}

// renderChapterList returns a summary of all chapters' word counts (for status).
func renderChapterList(bookDir string, cfg *config.BookConfig) string {
	var sb strings.Builder
	for _, ch := range cfg.Chapters {
		path := filepath.Join(bookDir, "chapters", ch.File)
		data, err := os.ReadFile(path)
		words := "—"
		status := "○ pending"
		if err == nil {
			wc := len(strings.Fields(string(data)))
			words = fmt.Sprintf("%d words", wc)
			status = "● written"
		}
		title := ch.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		sb.WriteString(fmt.Sprintf("  Ch%02d  %-32s  %s  (%s)\n",
			ch.Number, title, status, words))
	}
	return sb.String()
}
