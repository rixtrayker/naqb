package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"

	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
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
				Foreground(theme.ColorSecondary).
				PaddingLeft(2)

	homeItemActive = lipgloss.NewStyle().
				Background(theme.ColorSecondary).
				Foreground(lipgloss.Color("230")).
				PaddingLeft(1).PaddingRight(1)

	homeItemNormal = lipgloss.NewStyle().
				PaddingLeft(2)

	homeMetaStyle = lipgloss.NewStyle().
				Faint(true)

	homeSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.ColorDim).
				PaddingLeft(2).
				PaddingTop(1)

	homeDividerStyle = lipgloss.NewStyle().
				Foreground(theme.ColorBorder)
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
	loadErr  string
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
		if msg.err != nil {
			m.loadErr = fmt.Sprintf("Failed to load projects: %v", msg.err)
		} else {
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

// projectSource implements fuzzy.Source over a slice of vault.Project,
// searching across the concatenated title + name + domain + language fields.
type projectSource struct {
	projects []vault.Project
	haystack []string
}

func newProjectSource(projects []vault.Project) projectSource {
	ps := projectSource{projects: projects}
	for _, p := range projects {
		ps.haystack = append(ps.haystack, strings.ToLower(p.Title+" "+p.Name+" "+p.Domain+" "+p.Language))
	}
	return ps
}

func (ps projectSource) String(i int) string { return ps.haystack[i] }
func (ps projectSource) Len() int            { return len(ps.projects) }

func (m *homeModel) applyFilter() {
	q := strings.TrimSpace(m.search.Value())
	if q == "" {
		m.filtered = m.projects
		return
	}
	src := newProjectSource(m.projects)
	matches := fuzzy.FindFrom(strings.ToLower(q), src)
	out := make([]vault.Project, 0, len(matches))
	for _, match := range matches {
		out = append(out, m.projects[match.Index])
	}
	m.filtered = out
}

func (m *homeModel) View() string {
	w := m.width
	h := m.height
	if w == 0 || h == 0 {
		return "Loading..."
	}

	// ── Top section: header + search ─────────────────────────────────────────
	var top strings.Builder

	brand := theme.BrandStyle.Render("  نقب  nqb")
	version := theme.VersionStyle.Render("  v0.5")
	headerLeft := brand + version
	tagline := theme.SubtitleStyle.Render("excavate your ideas — give them depth")

	headerLine := lipgloss.NewStyle().Width(w).Render(headerLeft)
	top.WriteString("\n" + headerLine + "\n")
	top.WriteString(theme.SubtitleStyle.PaddingLeft(2).Render(tagline) + "\n")
	top.WriteString(homeDividerStyle.Render(strings.Repeat("─", w)) + "\n")

	searchLabel := homeSearchLabelStyle.Render("[/] ")
	top.WriteString(searchLabel + m.search.View() + "\n")
	top.WriteString(homeDividerStyle.Render(strings.Repeat("─", w)) + "\n")

	topStr := top.String()
	topLines := strings.Count(topStr, "\n")

	// ── Bottom section: status bars ──────────────────────────────────────────
	var bot strings.Builder
	vaultPath := vault.DefaultVaultPath()
	bot.WriteString(homeMetaStyle.PaddingLeft(2).Render("vault: "+vaultPath) + "\n")
	bot.WriteString(theme.RenderStatusBar(keys.HomeBindings, w) + "\n")
	if m.search.Focused() {
		bot.WriteString(theme.RenderStatusBar(keys.HomeSearchHint, w) + "\n")
	}
	botStr := bot.String()
	botLines := strings.Count(botStr, "\n")

	// ── Middle section: project list (fills remaining height) ────────────────
	middleH := h - topLines - botLines
	if middleH < 3 {
		middleH = 3
	}

	var mid strings.Builder
	contentLines := 0

	if m.loading {
		mid.WriteString(homeItemNormal.Render("  Loading projects...") + "\n")
		contentLines++
	} else if m.loadErr != "" {
		mid.WriteString("\n")
		contentLines++
		mid.WriteString(homeItemNormal.Render(m.loadErr) + "\n")
		contentLines++
	} else if len(m.filtered) == 0 {
		mid.WriteString("\n")
		contentLines++
		if len(m.projects) == 0 {
			mid.WriteString(homeItemNormal.Render("No books found in your vault.") + "\n")
			contentLines++
			mid.WriteString(homeMetaStyle.PaddingLeft(2).Render("") + "\n")
			contentLines++
			mid.WriteString(homeItemNormal.Render("  [N]  Create a new book project") + "\n")
			contentLines++
			mid.WriteString(homeItemNormal.Render("  Run  nqb init           from CLI") + "\n")
			contentLines++
			mid.WriteString(homeItemNormal.Render("  Run  nqb open <path>    to open existing") + "\n")
			contentLines++
		} else {
			mid.WriteString(homeItemNormal.Render("No projects match your search.") + "\n")
			contentLines++
		}
	} else {
		// Show recently opened books at the top, then the rest
		mid.WriteString(homeSectionStyle.Render("Projects") + "\n")
		contentLines++

		// Use all available middle height for the project list
		maxVisible := middleH - 2 // reserve 1 for header, 1 for "... more"
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
			mid.WriteString(row + "\n")
			contentLines++
		}

		if len(m.filtered) > maxVisible {
			mid.WriteString(homeMetaStyle.Render(fmt.Sprintf(
				"  ... %d more (scroll with j/k)", len(m.filtered)-maxVisible)) + "\n")
			contentLines++
		}
	}

	// Pad middle section to fill remaining height — pushes status bar to bottom
	for contentLines < middleH {
		mid.WriteString("\n")
		contentLines++
	}

	return topStr + mid.String() + botStr
}

// renderProject renders one project row, two-column card style.
func (m *homeModel) renderProject(p vault.Project, active bool, totalW int) string {
	// Language badge
	var badge string
	if p.Language == "ar" {
		badge = theme.BadgeAR.Render("AR")
	} else {
		badge = theme.BadgeEN.Render("EN")
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
	return humanize.Time(t)
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
