package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	bvHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.ColorPrimary).
				Background(theme.ColorBg).
				Padding(0, 2)

	bvHeaderMetaStyle = lipgloss.NewStyle().
					Faint(true).
					Foreground(theme.ColorDim).
					Background(theme.ColorBg).
					Padding(0, 1)

	bvSidebarStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(theme.ColorBorder).
				PaddingRight(1).
				Width(24)

	bvMainStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	bvChapterActive = lipgloss.NewStyle().
				Foreground(theme.ColorAccent).
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
					BorderForeground(theme.ColorSecondary).
					Padding(0, 1).
					Width(60)

	bvOutputStyle = lipgloss.NewStyle().
				Faint(true).
				PaddingLeft(2)
)

// ── Sidebar actions ──────────────────────────────────────────────────────────

var defaultSidebarActions = []SidebarAction{
	{Key: "w", Label: "Write chapter", Cmd: "/write"},
	{Key: "q", Label: "QA chapter", Cmd: "/qa"},
	{Key: "e", Label: "Export", Cmd: "/export"},
	{Key: "p", Label: "Preview chapter", Cmd: "/preview"},
	{Key: "o", Label: "Outline editor", Cmd: "/outline"},
	{Key: "~", Label: "Chat (Opus)", Cmd: "/chat"},
	{Key: "s", Label: "Status", Cmd: "/status"},
	{Key: "W", Label: "Watch", Cmd: "/watch"},
	{Key: "?", Label: "Help", Cmd: "/help"},
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
	palette      PaletteHandler
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
		palette:      nil, // set by RunBookView
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
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" || msg.String() == "ctrl+c" {
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
		if m.cfg != nil && m.cursor < len(m.cfg.Chapters)-1 {
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
	m.statusMsg = bvStatusWip.Render("⟳ Running: " + input + "  (10m timeout)")
	m.outputLines = nil

	// Determine target chapter
	chNum := 0
	if m.cursor < len(m.cfg.Chapters) {
		chNum = m.cfg.Chapters[m.cursor].Number
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if m.palette == nil {
			return bookTaskMsg{err: fmt.Errorf("palette not configured")}
		}
		out, err := m.palette.Dispatch(ctx, input, m.bookDir, m.cfg, m.client, chNum)
		return bookTaskMsg{output: out, err: err}
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *BookViewModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Help overlay takes over the full screen.
	if m.showHelp {
		return "\n" + keys.RenderHelpOverlay(keys.BookViewHelpSections) + "\n"
	}

	// ── Top: header bar ─────────────────────────────────────────────────────
	if m.cfg == nil {
		return "Error: book config not loaded"
	}
	var top strings.Builder
	chCount := len(m.cfg.Chapters)
	written := 0
	for _, ch := range m.cfg.Chapters {
		if _, err := os.Stat(filepath.Join(m.bookDir, "chapters", ch.File)); err == nil {
			written++
		}
	}
	headerTitle := bvHeaderStyle.Render(fmt.Sprintf("\u2067نقب\u2069  %s", m.cfg.Title))
	headerMeta := bvHeaderMetaStyle.Render(fmt.Sprintf(
		"by %s  ·  %d/%d chapters  ·  %s",
		m.cfg.Author, written, chCount, m.cfg.Language,
	))
	headerBar := lipgloss.JoinHorizontal(lipgloss.Top, headerTitle, headerMeta)
	top.WriteString(lipgloss.NewStyle().
		Background(theme.ColorBg).
		Width(m.width).
		Render(headerBar) + "\n")
	top.WriteString(lipgloss.NewStyle().
		Foreground(theme.ColorBorder).
		Render(strings.Repeat("─", m.width)) + "\n")
	topStr := top.String()
	topLines := strings.Count(topStr, "\n")

	// ── Bottom: status bar + overlay/output ──────────────────────────────────
	var bot strings.Builder
	// Command palette or status message
	if m.showPalette {
		paletteW := min(60, m.width-4)
		palette := bvCmdPaletteStyle.Width(paletteW).Render(
			"Command palette\n" +
				m.paletteInput.View() + "\n" +
				theme.RenderHintBar(keys.BookViewPaletteBindings),
		)
		bot.WriteString(palette + "\n")
	} else if m.statusMsg != "" {
		bot.WriteString(m.statusMsg + "\n")
	}

	if len(m.outputLines) > 0 {
		maxOut := min(len(m.outputLines), 5)
		for _, line := range m.outputLines[len(m.outputLines)-maxOut:] {
			bot.WriteString(bvOutputStyle.Render(line) + "\n")
		}
	}

	var chInfo string
	if m.cursor < len(m.cfg.Chapters) {
		ch := m.cfg.Chapters[m.cursor]
		chInfo = fmt.Sprintf("Ch%02d: %s", ch.Number, ch.Title)
		if len(chInfo) > 30 {
			chInfo = chInfo[:27] + "..."
		}
	}
	bot.WriteString(theme.RenderStatusBar(append([]theme.Binding{{Key: "sel", Desc: chInfo}}, keys.BookViewBindings...), m.width) + "\n")
	botStr := bot.String()
	botLines := strings.Count(botStr, "\n")

	// ── Middle: sidebar + main (fills remaining height) ─────────────────────
	bodyH := m.height - topLines - botLines
	if bodyH < 5 {
		bodyH = 5
	}

	sidebarContent := m.renderSidebar()
	mainContent := m.renderMain()

	sidebarSized := lipgloss.NewStyle().Height(bodyH).Render(sidebarContent)
	mainSized := lipgloss.NewStyle().Height(bodyH).Render(mainContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebarSized, mainSized)

	return topStr + body + "\n" + botStr
}

func (m *BookViewModel) renderSidebar() string {
	sidebarW := min(24, m.width/3)
	if sidebarW < 12 {
		sidebarW = 12
	}
	tabBar := renderTabBar(m.activeTab, sidebarW)
	content := renderSidebarContent(m.activeTab, m.bookDir, m.cfg, defaultSidebarActions)

	var sb strings.Builder
	sb.WriteString(tabBar + "\n\n")
	sb.WriteString(content)
	return bvSidebarStyle.Width(sidebarW).Render(sb.String())
}

func (m *BookViewModel) renderMain() string {
	sidebarW := min(24, m.width/3)
	if sidebarW < 12 {
		sidebarW = 12
	}
	mainW := m.width - sidebarW - 4
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
			pill = theme.PillWritten.Render("✓")
			chStyle = bvChapterActive
		case i == m.cursor:
			pill = theme.PillPending.Render("·")
			chStyle = bvChapterActive
		case ch.Status == "imported":
			pill = theme.PillImported.Render("↓")
			chStyle = bvChapterDone
		case exists:
			pill = theme.PillWritten.Render("✓")
			chStyle = bvChapterDone
		default:
			pill = theme.PillPending.Render("·")
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
	m.EnsurePalette(DefaultCommandRegistry)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
