package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	bvHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBg).
			Padding(0, 2)

	bvHeaderMetaStyle = lipgloss.NewStyle().
				Faint(true).
				Foreground(ColorDim).
				Background(ColorBg).
				Padding(0, 1)

	bvSidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(ColorBorder).
			PaddingRight(1).
			Width(24)

	bvSidebarKeyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorAccent)

	bvSidebarLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	bvMainStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	bvChapterActive = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	bvChapterDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	bvChapterPending = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	bvStatusOK  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	bvStatusErr = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	bvStatusWip = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	bvCmdPaletteStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSecondary).
				Padding(0, 1).
				Width(60)

	bvOutputStyle = lipgloss.NewStyle().
			Faint(true).
			PaddingLeft(2)
)

// ── Sidebar actions ──────────────────────────────────────────────────────────

type sidebarAction struct {
	key   string
	label string
	cmd   string // slash command template
}

var sidebarActions = []sidebarAction{
	{key: "w", label: "Write chapter", cmd: "/write"},
	{key: "q", label: "QA chapter", cmd: "/qa"},
	{key: "e", label: "Export", cmd: "/export"},
	{key: "p", label: "Preview chapter", cmd: "/preview"},
	{key: "o", label: "Outline editor", cmd: "/outline"},
	{key: "~", label: "Chat (Opus)", cmd: "/chat"},
	{key: "s", label: "Status", cmd: "/status"},
	{key: "W", label: "Watch", cmd: "/watch"},
	{key: "?", label: "Help", cmd: "/help"},
}

// ── Async task messages ──────────────────────────────────────────────────────

type bookTaskMsg struct {
	output string
	err    error
}

// ── Model ────────────────────────────────────────────────────────────────────

// BookViewModel is the Bubble Tea model for the main book editing TUI.
type BookViewModel struct {
	bookDir      string
	cfg          *config.BookConfig
	client       llm.Provider
	cursor       int        // selected chapter index
	activeTab    sidebarTab // which sidebar tab is shown
	width        int
	height       int
	showPalette  bool
	showHelp     bool // help overlay toggle
	paletteInput textinput.Model
	outputLines  []string
	running      bool
	statusMsg    string
}

// NewBookView creates the book TUI model.
func NewBookView(bookDir string, cfg *config.BookConfig, client llm.Provider) *BookViewModel {
	pi := textinput.New()
	pi.Placeholder = "type command... e.g. /write --chapter 1"
	pi.CharLimit = 200

	return &BookViewModel{
		bookDir:      bookDir,
		cfg:          cfg,
		client:       client,
		paletteInput: pi,
	}
}

func (m *BookViewModel) Init() tea.Cmd {
	return nil
}

func (m *BookViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case bookTaskMsg:
		m.running = false
		if msg.err != nil {
			m.statusMsg = bvStatusErr.Render("✗ " + msg.err.Error())
		} else {
			m.statusMsg = bvStatusOK.Render("✓ Done")
		}
		if msg.output != "" {
			for _, line := range strings.Split(strings.TrimSpace(msg.output), "\n") {
				m.outputLines = append(m.outputLines, line)
			}
			// Keep last 20 lines
			if len(m.outputLines) > 20 {
				m.outputLines = m.outputLines[len(m.outputLines)-20:]
			}
		}

	case tea.KeyMsg:
		// Help overlay intercepts all keys — only Esc/? closes it.
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "ctrl+c" {
				m.showHelp = false
			}
			return m, nil
		}
		if m.showPalette {
			return m.updatePalette(msg)
		}
		return m.updateMain(msg)
	}

	if m.showPalette {
		var cmd tea.Cmd
		m.paletteInput, cmd = m.paletteInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *BookViewModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "/":
		m.showPalette = true
		m.paletteInput.SetValue("/")
		m.paletteInput.Focus()
		m.paletteInput.SetCursor(1)
		return m, textinput.Blink

	case "tab":
		m.activeTab = (m.activeTab + 1) % tabCount
	case "shift+tab":
		m.activeTab = (m.activeTab - 1 + tabCount) % tabCount

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.cfg.Chapters)-1 {
			m.cursor++
		}

	case "w":
		return m, m.runCommand("/write")
	case "q":
		return m, m.runCommand("/qa")
	case "r":
		return m, m.runCommand("/research")
	case "e":
		return m, m.runCommand("/export --format pdf")
	case "p":
		return m, m.runCommand("/preview")
	case "o":
		return m, m.runCommand("/outline")
	case "~":
		return m, m.runCommand("/chat")
	case "s":
		return m, m.runCommand("/status")
	case "W":
		return m, m.runCommand("/watch")
	}
	return m, nil
}

func (m *BookViewModel) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.showPalette = false
		m.paletteInput.Blur()
		return m, nil
	case tea.KeyEnter:
		input := strings.TrimSpace(m.paletteInput.Value())
		m.showPalette = false
		m.paletteInput.Blur()
		m.paletteInput.SetValue("")
		if input != "" {
			return m, m.runCommand(input)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	return m, cmd
}

func (m *BookViewModel) runCommand(input string) tea.Cmd {
	if m.running {
		return nil
	}
	m.running = true
	m.statusMsg = bvStatusWip.Render("⟳ Running: " + input)
	m.outputLines = nil

	// Determine target chapter
	chNum := 0
	if m.cursor < len(m.cfg.Chapters) {
		chNum = m.cfg.Chapters[m.cursor].Number
	}

	return func() tea.Msg {
		out, err := dispatchCommand(input, m.bookDir, m.cfg, m.client, chNum)
		return bookTaskMsg{output: out, err: err}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *BookViewModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Help overlay takes over the full screen.
	if m.showHelp {
		return "\n" + RenderHelpOverlay(BookViewHelpSections) + "\n"
	}

	var full strings.Builder

	// ── Header bar (full width) ──────────────────────────────────────────────
	chCount := len(m.cfg.Chapters)
	written := 0
	for _, ch := range m.cfg.Chapters {
		if _, err := os.Stat(filepath.Join(m.bookDir, "chapters", ch.File)); err == nil {
			written++
		}
	}
	headerTitle := bvHeaderStyle.Render(fmt.Sprintf("نقب  %s", m.cfg.Title))
	headerMeta := bvHeaderMetaStyle.Render(fmt.Sprintf(
		"by %s  ·  %d/%d chapters  ·  %s",
		m.cfg.Author, written, chCount, m.cfg.Language,
	))
	headerBar := lipgloss.JoinHorizontal(lipgloss.Top, headerTitle, headerMeta)
	full.WriteString(lipgloss.NewStyle().
		Background(ColorBg).
		Width(m.width).
		Render(headerBar) + "\n")
	full.WriteString(lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", m.width)) + "\n")

	// ── Body: sidebar + main ──────────────────────────────────────────────────
	sidebarContent := m.renderSidebar()
	mainContent := m.renderMain()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, mainContent)
	full.WriteString(body + "\n")

	// ── Command palette overlay ───────────────────────────────────────────────
	if m.showPalette {
		palette := bvCmdPaletteStyle.Render(
			"Command palette\n" +
				m.paletteInput.View() + "\n" +
				renderHintBar(BookViewPaletteBindings),
		)
		full.WriteString("\n" + palette + "\n")
	} else if m.statusMsg != "" {
		full.WriteString("\n" + m.statusMsg + "\n")
	}

	if len(m.outputLines) > 0 {
		for _, line := range m.outputLines {
			full.WriteString(bvOutputStyle.Render(line) + "\n")
		}
	}

	// ── Bottom status bar ────────────────────────────────────────────────────
	var chInfo string
	if m.cursor < len(m.cfg.Chapters) {
		ch := m.cfg.Chapters[m.cursor]
		chInfo = fmt.Sprintf("Ch%02d: %s", ch.Number, ch.Title)
		if len(chInfo) > 30 {
			chInfo = chInfo[:27] + "..."
		}
	}
	statusLeft := StatusKeyStyle.Render(chInfo)
	bottomBar := renderStatusBar(append([]Binding{{Key: "sel", Desc: chInfo}}, BookViewBindings...), m.width)
	_ = statusLeft
	full.WriteString("\n" + bottomBar + "\n")

	return full.String()
}

func (m *BookViewModel) renderSidebar() string {
	sidebarW := 24
	tabBar := renderTabBar(m.activeTab, sidebarW)
	content := renderSidebarContent(m.activeTab, m.bookDir, m.cfg, sidebarActions)

	var sb strings.Builder
	sb.WriteString(tabBar + "\n\n")
	sb.WriteString(content)
	return bvSidebarStyle.Width(sidebarW).Render(sb.String())
}

func (m *BookViewModel) renderMain() string {
	mainW := m.width - 28
	if mainW < 20 {
		mainW = 20
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Faint(true).PaddingLeft(1).Render("CHAPTERS") + "\n\n")

	for i, ch := range m.cfg.Chapters {
		chapPath := filepath.Join(m.bookDir, "chapters", ch.File)
		exists := false
		if _, err := os.Stat(chapPath); err == nil {
			exists = true
		}

		var pill string
		var chStyle lipgloss.Style
		switch {
		case i == m.cursor && exists:
			pill = PillWritten.Render("✓")
			chStyle = bvChapterActive
		case i == m.cursor:
			pill = PillPending.Render("·")
			chStyle = bvChapterActive
		case ch.Status == "imported":
			pill = PillImported.Render("↓")
			chStyle = bvChapterDone
		case exists:
			pill = PillWritten.Render("✓")
			chStyle = bvChapterDone
		default:
			pill = PillPending.Render("·")
			chStyle = bvChapterPending
		}

		title := ch.Title
		maxTitleW := mainW - 14
		if maxTitleW < 10 {
			maxTitleW = 10
		}
		if len(title) > maxTitleW {
			title = title[:maxTitleW-3] + "..."
		}

		var cursor string
		if i == m.cursor {
			cursor = bvChapterActive.Render("▶")
		} else {
			cursor = "  "
		}

		line := fmt.Sprintf("%s %s  Ch%02d  %s", cursor, pill, ch.Number, title)
		sb.WriteString(chStyle.Render(line) + "\n")
	}

	// Selected chapter detail
	if m.cursor < len(m.cfg.Chapters) {
		ch := m.cfg.Chapters[m.cursor]
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Faint(true).PaddingLeft(2).
			Render(fmt.Sprintf("Chapter %d — %s", ch.Number, ch.Title)) + "\n")
		if ch.Summary != "" {
			summary := ch.Summary
			if len(summary) > mainW-4 {
				summary = summary[:mainW-7] + "..."
			}
			sb.WriteString(lipgloss.NewStyle().Faint(true).PaddingLeft(2).Render(summary) + "\n")
		}
	}

	return bvMainStyle.Width(mainW).Render(sb.String())
}

// RunBookView launches the book TUI for a given project.
func RunBookView(bookDir string, cfg *config.BookConfig, client llm.Provider) error {
	m := NewBookView(bookDir, cfg, client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
