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
	HomeOpen HomeAction = iota
	// HomeNew indicates the user wants to create a new book.
	HomeNew
	// HomeVaults indicates the user wants to manage vaults.
	HomeVaults
	// HomeQuit indicates the user cancelled without selecting anything.
	HomeQuit
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	homeSearchLabelStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				PaddingLeft(2)

	homeItemActive = lipgloss.NewStyle().
			Background(ColorSecondary).
			Foreground(lipgloss.Color("230")).
			PaddingLeft(1).PaddingRight(1)

	homeItemNormal = lipgloss.NewStyle().
			PaddingLeft(2)

	homeMetaStyle = lipgloss.NewStyle().
			Faint(true)

	homeSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorDim).
				PaddingLeft(2).
				PaddingTop(1)

	homeDividerStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)
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
	w := m.width
	if w == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// ── Brand header ──────────────────────────────────────────────────────────
	brand := BrandStyle.Render("  نقب  nqb")
	version := VersionStyle.Render("  v0.1")
	headerLeft := brand + version
	tagline := SubtitleStyle.Render("excavate your ideas — give them depth")

	// Pad tagline to right-align in width
	headerLine := lipgloss.NewStyle().Width(w).Render(headerLeft)
	b.WriteString("\n" + headerLine + "\n")
	b.WriteString(SubtitleStyle.PaddingLeft(2).Render(tagline) + "\n")

	// Divider
	b.WriteString(homeDividerStyle.Render(strings.Repeat("─", w)) + "\n")

	// ── Search bar ────────────────────────────────────────────────────────────
	searchLabel := homeSearchLabelStyle.Render("[/] ")
	b.WriteString(searchLabel + m.search.View() + "\n")
	b.WriteString(homeDividerStyle.Render(strings.Repeat("─", w)) + "\n\n")

	// ── Project list ──────────────────────────────────────────────────────────
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
			row := m.renderProject(p, i == m.cursor, w)
			b.WriteString(row + "\n")
		}

		if len(m.filtered) > maxVisible {
			b.WriteString(homeMetaStyle.Render(fmt.Sprintf(
				"  ... %d more (scroll with j/k)", len(m.filtered)-maxVisible)) + "\n")
		}
	}

	// ── Pinned status bar ─────────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString(renderStatusBar(HomeBindings, w) + "\n")
	if m.search.Focused() {
		b.WriteString(renderStatusBar(HomeSearchHint, w) + "\n")
	}

	return b.String()
}

// renderProject renders one project row, two-column card style.
func (m *homeModel) renderProject(p vault.Project, active bool, totalW int) string {
	// Language badge
	var badge string
	if p.Language == "ar" {
		badge = BadgeAR.Render("AR")
	} else {
		badge = BadgeEN.Render("EN")
	}

	// Title column (left, truncated)
	titleW := totalW / 2
	if titleW > 40 {
		titleW = 40
	}
	title := p.Title
	if len(title) > titleW-4 {
		title = title[:titleW-7] + "..."
	}

	// Meta column (right, faint)
	age := humanTime(p.ModTime)
	domain := p.Domain
	if len(domain) > 16 {
		domain = domain[:13] + "..."
	}
	meta := homeMetaStyle.Render(fmt.Sprintf("%-16s  %s", domain, age))

	// Compose
	leftCol := fmt.Sprintf("%s %s", badge, title)
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(titleW+4).Render(leftCol),
		meta,
	)

	if active {
		return homeItemActive.Width(totalW - 2).Render(line)
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
