package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	bvTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Padding(0, 1)

	bvSidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("238")).
			PaddingRight(1).
			Width(22)

	bvSidebarKeyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	bvSidebarLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	bvMainStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	bvChapterActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
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
				BorderForeground(lipgloss.Color("62")).
				Padding(0, 1).
				Width(60)

	bvOutputStyle = lipgloss.NewStyle().
			Faint(true).
			PaddingLeft(2)

	bvFooterStyle = lipgloss.NewStyle().
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
	client       *llm.Client
	cursor       int // selected chapter index
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
func NewBookView(bookDir string, cfg *config.BookConfig, client *llm.Client) *BookViewModel {
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

// dispatchCommand parses a slash command and executes the appropriate action.
func dispatchCommand(input, bookDir string, cfg *config.BookConfig, client *llm.Client, defaultChapter int) (string, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	chNum := defaultChapter
	format := "pdf"

	// Parse flags
	for i := 1; i < len(parts); i++ {
		if parts[i] == "--chapter" || parts[i] == "-c" {
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%d", &chNum)
				i++
			}
		}
		if parts[i] == "--format" || parts[i] == "-f" {
			if i+1 < len(parts) {
				format = parts[i+1]
				i++
			}
		}
	}

	ctx := context.Background()

	switch cmd {
	case "write", "w":
		path, err := agents.WriteContextFile(bookDir, cfg, chNum)
		if err != nil {
			return "", fmt.Errorf("context: %w", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Context → %s\n", path))
		chPath, err := agents.WriteChapter(ctx, client, bookDir, cfg, chNum, func(delta string) error {
			sb.WriteString(delta)
			return nil
		})
		if err != nil {
			return sb.String(), fmt.Errorf("write: %w", err)
		}
		sb.WriteString(fmt.Sprintf("\n\nChapter → %s", chPath))
		return sb.String(), nil

	case "qa", "q":
		result, err := agents.RunQA(ctx, client, bookDir, cfg, chNum)
		if err != nil {
			return "", err
		}
		_ = agents.WriteQAReport(bookDir, result)
		if result.Passed {
			return "QA passed: " + result.DeterministicMsg, nil
		}
		return "QA issues:\n" + strings.Join(result.Issues, "\n"), nil

	case "export", "e":
		return runExport(bookDir, cfg, format)

	case "status", "s":
		return buildStatusText(bookDir, cfg), nil

	case "context":
		path, err := agents.WriteContextFile(bookDir, cfg, chNum)
		if err != nil {
			return "", err
		}
		return "Context → " + path, nil

	case "preview", "p":
		return renderPreview(bookDir, cfg, chNum)

	case "help", "?":
		return buildHelp(), nil

	case "watch", "W":
		return "(Watch mode — use 'nqb watch' from CLI for daemon mode)", nil

	case "chat", "~":
		// Chat can't run inline — signal caller
		return "(Opening chat — use 'nqb chat' from CLI)", nil

	case "outline", "o":
		return "(Opening outline editor — use 'nqb outline' from CLI)", nil

	default:
		return "", fmt.Errorf("unknown command: /%s  (try /help)", cmd)
	}
}

func runExport(bookDir string, cfg *config.BookConfig, format string) (string, error) {
	// Import exporter inline to avoid circular dep via thin wrappers
	_ = bookDir
	_ = cfg
	return fmt.Sprintf("Export %s — run 'nqb export --format %s' from CLI for full export", strings.ToUpper(format), format), nil
}

func buildStatusText(bookDir string, cfg *config.BookConfig) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📚 %s\n", cfg.Title))
	for _, ch := range cfg.Chapters {
		icon := "○"
		chapPath := filepath.Join(bookDir, "chapters", ch.File)
		if _, err := os.Stat(chapPath); err == nil {
			icon = "●"
		}
		sb.WriteString(fmt.Sprintf("  %s Ch%02d: %s\n", icon, ch.Number, ch.Title))
	}
	return sb.String()
}

func buildHelp() string {
	// Returns a plain-text summary used when /help is run from the palette.
	var sb strings.Builder
	sb.WriteString("Palette commands:\n")
	for _, sec := range BookViewHelpSections {
		if sec.Title == "Palette commands" {
			for _, b := range sec.Bindings {
				sb.WriteString(fmt.Sprintf("  %-30s  %s\n", b.Key, b.Desc))
			}
		}
	}
	sb.WriteString("\nPress [?] for full keybinding reference.\n")
	return sb.String()
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

	sidebar := m.renderSidebar()
	main := m.renderMain()

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	title := bvTitleStyle.Render(fmt.Sprintf("نقب  %s", m.cfg.Title))

	var full strings.Builder
	full.WriteString(title + "\n")
	full.WriteString(body + "\n")

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

	// Footer hint bar
	full.WriteString("\n" + renderHintBar(BookViewBindings) + "\n")
	return full.String()
}

func (m *BookViewModel) renderSidebar() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Faint(true).PaddingLeft(1).Render("ACTIONS") + "\n\n")
	for _, a := range sidebarActions {
		line := fmt.Sprintf("%s  %s",
			bvSidebarKeyStyle.Render(a.key),
			bvSidebarLabelStyle.Render(a.label),
		)
		sb.WriteString(" " + line + "\n")
	}
	return bvSidebarStyle.Render(sb.String())
}

func (m *BookViewModel) renderMain() string {
	mainW := m.width - 26
	if mainW < 20 {
		mainW = 20
	}

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Faint(true).PaddingLeft(1).Render("CHAPTERS") + "\n\n")

	for i, ch := range m.cfg.Chapters {
		icon := "○"
		chStyle := bvChapterPending
		chapPath := filepath.Join(m.bookDir, "chapters", ch.File)
		if _, err := os.Stat(chapPath); err == nil {
			icon = "●"
			chStyle = bvChapterDone
		}
		if i == m.cursor {
			icon = "▶"
			chStyle = bvChapterActive
		}

		title := ch.Title
		if len(title) > mainW-10 {
			title = title[:mainW-13] + "..."
		}
		line := fmt.Sprintf(" %s  Ch%02d  %s", icon, ch.Number, title)
		sb.WriteString(chStyle.Render(line) + "\n")
	}

	// Show selected chapter detail
	if m.cursor < len(m.cfg.Chapters) {
		ch := m.cfg.Chapters[m.cursor]
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Faint(true).PaddingLeft(2).
			Render(fmt.Sprintf("Selected: Chapter %d — %s", ch.Number, ch.Title)) + "\n")
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
func RunBookView(bookDir string, cfg *config.BookConfig, client *llm.Client) error {
	m := NewBookView(bookDir, cfg, client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
