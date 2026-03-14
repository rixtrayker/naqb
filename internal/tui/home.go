package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/vault"
)

// HomeResult is what the home screen returns when a project is selected.
type HomeResult struct {
	Action  HomeAction
	Project *vault.Project
	NewBook bool // user pressed N
}

// HomeAction describes what the user picked.
type HomeAction int

const (
	// HomeOpen indicates the user selected an existing project to open.
	HomeOpen    HomeAction = iota
	// HomeNew indicates the user wants to create a new book.
	HomeNew
	// HomeVaults indicates the user wants to manage vaults.
	HomeVaults
	// HomeQuit indicates the user cancelled without selecting anything.
	HomeQuit
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	homeBrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			PaddingLeft(2)

	homeSubtitleStyle = lipgloss.NewStyle().
				Faint(true).
				PaddingLeft(2)

	homeSearchLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			PaddingLeft(2)

	homeItemActive = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			PaddingLeft(1).PaddingRight(1)

	homeItemNormal = lipgloss.NewStyle().
			PaddingLeft(2)

	homeMetaStyle = lipgloss.NewStyle().
			Faint(true)

	homeSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("241")).
				PaddingLeft(2).
				PaddingTop(1)

	homeFooterStyle = lipgloss.NewStyle().
			Faint(true).
			PaddingLeft(2)

	homeLangAr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	homeLangEn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))
)

// ── Model ────────────────────────────────────────────────────────────────────

type homeModel struct {
	projects []vault.Project
	filtered []vault.Project
	cursor   int
	search   textinput.Model
	width    int
	height   int
	loading  bool
	result   *HomeResult
}

type projectsLoadedMsg struct {
	projects []vault.Project
	err      error
}

func loadProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := vault.ListProjects()
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

// NewHomeModel creates the home screen model.
func NewHomeModel() *homeModel {
	search := textinput.New()
	search.Placeholder = "Search projects..."
	search.CharLimit = 100
	return &homeModel{
		loading: true,
		search:  search,
	}
}

func (m *homeModel) Init() tea.Cmd {
	return tea.Batch(loadProjectsCmd(), textinput.Blink)
}

func (m *homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case projectsLoadedMsg:
		m.loading = false
		if msg.err == nil {
			m.projects = msg.projects
			m.filtered = msg.projects
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.result = &HomeResult{Action: HomeQuit}
			return m, tea.Quit

		case "n", "N":
			m.result = &HomeResult{Action: HomeNew}
			return m, tea.Quit

		case "v", "V":
			m.result = &HomeResult{Action: HomeVaults}
			return m, tea.Quit

		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				p := m.filtered[m.cursor]
				m.result = &HomeResult{Action: HomeOpen, Project: &p}
				return m, tea.Quit
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		default:
			// Forward to search input
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.applyFilter()
			if m.cursor >= len(m.filtered) {
				m.cursor = max(0, len(m.filtered)-1)
			}
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m *homeModel) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	if q == "" {
		m.filtered = m.projects
		return
	}
	var out []vault.Project
	for _, p := range m.projects {
		if strings.Contains(strings.ToLower(p.Title), q) ||
			strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Language), q) ||
			strings.Contains(strings.ToLower(p.Domain), q) {
			out = append(out, p)
		}
	}
	m.filtered = out
}

func (m *homeModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	b.WriteString("\n")
	b.WriteString(homeBrandStyle.Render("نقب  nqb") + "\n")
	b.WriteString(homeSubtitleStyle.Render("probe your ideas. give them depth.") + "\n\n")

	// Search bar
	b.WriteString(homeSearchLabel.Render("Search: ") + m.search.View() + "\n\n")

	if m.loading {
		b.WriteString(homeItemNormal.Render("  Loading projects...") + "\n")
	} else if len(m.filtered) == 0 {
		if len(m.projects) == 0 {
			b.WriteString(homeItemNormal.Render("No projects yet. Press [N] to create your first book.") + "\n")
		} else {
			b.WriteString(homeItemNormal.Render("No projects match your search.") + "\n")
		}
	} else {
		b.WriteString(homeSectionStyle.Render("Projects") + "\n")
		// Show up to height-12 items
		maxVisible := m.height - 14
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			p := m.filtered[i]
			row := m.renderProject(p, i == m.cursor)
			b.WriteString(row + "\n")
		}

		if len(m.filtered) > maxVisible {
			b.WriteString(homeMetaStyle.Render(fmt.Sprintf(
				"  ... %d more (scroll with j/k)", len(m.filtered)-maxVisible)) + "\n")
		}
	}

	// Footer — keybinding hint bar
	b.WriteString("\n")
	b.WriteString(renderHintBar(HomeBindings) + "\n")
	if m.search.Focused() {
		b.WriteString(renderHintBar(HomeSearchHint) + "\n")
	}

	return b.String()
}

func (m *homeModel) renderProject(p vault.Project, active bool) string {
	lang := p.Language
	langStyle := homeLangEn
	if lang == "ar" {
		langStyle = homeLangAr
	}

	progress := ""
	if p.Chapters > 0 {
		progress = fmt.Sprintf("%d/%d ch", p.Written, p.Chapters)
	}

	age := humanTime(p.ModTime)

	title := p.Title
	if len(title) > 35 {
		title = title[:32] + "..."
	}

	meta := homeMetaStyle.Render(
		fmt.Sprintf("%-8s  %-10s  %s", langStyle.Render(lang), progress, age),
	)

	line := fmt.Sprintf("%-36s  %s", title, meta)

	if active {
		return homeItemActive.Render(line)
	}
	return homeItemNormal.Render(line)
}

func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunHome launches the TUI home screen and returns the user's choice.
func RunHome() (*HomeResult, error) {
	m := NewHomeModel()
	// Focus search on start
	m.search.Focus()
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := finalModel.(*homeModel)
	if fm.result == nil {
		return &HomeResult{Action: HomeQuit}, nil
	}
	return fm.result, nil
}
