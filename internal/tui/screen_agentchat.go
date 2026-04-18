package tui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/pkg/agent"
	"github.com/amr/naqb/pkg/booktools"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/internal/knowledge"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/internal/tui/components"
	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
)

var (
	agentTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Padding(0, 1)
	agentStatusStyle = lipgloss.NewStyle().Faint(true).PaddingLeft(2)
	agentTaskStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).PaddingLeft(2)
	agentBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
)

// AgentChatModel is the Bubble Tea model for the interactive agent chat.
type AgentChatModel struct {
	base            components.ChatBase
	fantasyProvider fantasy.Provider
	modelID         string
	bookDir         string
	cfg             *config.BookConfig
	sqlDB           *sql.DB
	tracker         *components.TaskTracker
	analysis        *agent.ProjectAnalysis
	sessionID       string
	entries         []components.ChatEntry
	streaming       bool
	streamCh        chan tea.Msg
	cancelStream    context.CancelFunc
	ctx             context.Context
	showTasks       bool
	llmClient       llm.Provider
}

func newAgentChatModel(
	ctx context.Context,
	bookDir string,
	cfg *config.BookConfig,
	fantasyProvider fantasy.Provider,
	modelID string,
	sqlDB *sql.DB,
) *AgentChatModel {
	m := &AgentChatModel{
		base:            components.NewChatBase(),
		fantasyProvider: fantasyProvider,
		modelID:         modelID,
		bookDir:         bookDir,
		cfg:             cfg,
		sqlDB:           sqlDB,
		tracker:         components.NewTaskTracker(),
		analysis:        agent.Analyze(bookDir, cfg, knowledge.NewEpistemicStore(sqlDB)),
		ctx:             ctx,
		entries:         []components.ChatEntry{},
	}

	var welcome strings.Builder
	welcome.WriteString(fmt.Sprintf("Welcome to **%s** — interactive agent chat.\n\n", cfg.Title))
	if m.analysis.TotalChapters > 0 {
		totalPct := 0
		if m.analysis.TotalTarget > 0 {
			totalPct = m.analysis.TotalWords * 100 / m.analysis.TotalTarget
		}
		welcome.WriteString(fmt.Sprintf("Project: %d chapters, %d/%d words (%d%%)\n",
			m.analysis.TotalChapters, m.analysis.TotalWords, m.analysis.TotalTarget, totalPct))
	}
	welcome.WriteString("\nAsk me anything about your book, or say \"write chapter N\" to start writing.")
	m.entries = append(m.entries, components.ChatEntry{Role: components.RoleSystem, Content: welcome.String()})

	m.tryResumeSession()
	return m
}

func (m *AgentChatModel) tryResumeSession() {
	if m.sqlDB == nil {
		return
	}
	sessions, err := db.NewSessionStore(m.sqlDB).ListSessions(m.ctx, m.bookDir, 1)
	if err != nil || len(sessions) == 0 {
		return
	}
	recent := sessions[0]
	if time.Since(recent.UpdatedAt) < 24*time.Hour {
		m.sessionID = recent.ID
	}
}

func (m *AgentChatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *AgentChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.base.HandleWindowSize(msg.Width, msg.Height)
		m.rebuildContent()
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.cancelStream != nil {
				m.cancelStream()
				m.cancelStream = nil
			}
			return m, tea.Quit

		case tea.KeyCtrlT:
			m.showTasks = !m.showTasks
			m.rebuildContent()
			m.refreshViewport()
			return m, nil

		case tea.KeyEnter:
			if msg.Alt {
				break
			}
			if m.streaming {
				return m, nil
			}
			userInput := strings.TrimSpace(m.base.Textarea.Value())
			if userInput == "" {
				return m, nil
			}
			m.base.Textarea.Reset()
			m.addEntry(components.RoleUser, userInput)
			m.streaming = true
			m.startAgentStream(userInput)
			cmds = append(cmds, components.WaitForStream(m.streamCh))
			return m, tea.Batch(cmds...)
		}

	case components.StreamDeltaMsg:
		wasAtBottom := m.base.Viewport.AtBottom()
		m.entries = components.ApplyDelta(m.entries, msg.Delta)
		m.rebuildContent()
		m.refreshViewport()
		if wasAtBottom {
			m.base.Viewport.GotoBottom()
		}
		cmds = append(cmds, components.WaitForStream(m.streamCh))

	case components.StreamDoneMsg:
		m.streaming = false
		m.streamCh = nil
		m.cancelStream = nil
		m.rebuildContent()
		m.refreshViewport()

	case components.TaskCompleteMsg:
		m.handleTaskComplete(msg)
		m.rebuildContent()
		m.refreshViewport()
		m.base.Viewport.GotoBottom()
	}

	var taCmd tea.Cmd
	m.base.Textarea, taCmd = m.base.Textarea.Update(msg)
	cmds = append(cmds, taCmd)

	var vpCmd tea.Cmd
	m.base.Viewport, vpCmd = m.base.Viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *AgentChatModel) View() string {
	activeCount := len(m.tracker.Active())
	titleText := fmt.Sprintf("\u2067نقب\u2069  %s", m.cfg.Title)
	if activeCount > 0 {
		titleText += fmt.Sprintf("  [%d task(s) running]", activeCount)
	}
	title := agentTitleStyle.Render(titleText)
	chatArea := agentBorderStyle.Render(m.base.Viewport.View())

	var statusLine string
	if m.streaming {
		statusLine = agentStatusStyle.Render("Thinking...")
	} else {
		statusLine = theme.RenderHintBar(keys.AgentChatBindings)
	}

	input := agentBorderStyle.Render(m.base.Textarea.View())
	return fmt.Sprintf("%s\n%s\n%s\n%s", title, chatArea, statusLine, input)
}

func (m *AgentChatModel) addEntry(role components.ChatRole, content string) {
	m.entries = append(m.entries, components.ChatEntry{Role: role, Content: content})
	m.rebuildContent()
	m.refreshViewport()
	m.base.Viewport.GotoBottom()
}

func (m *AgentChatModel) rebuildContent() {
	styles := components.ChatStyles{
		UserStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("212")).PaddingLeft(2),
		AssistStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("86")).PaddingLeft(2),
		SystemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).PaddingLeft(2).Italic(true),
	}
	content := components.BuildChatContent(m.entries, styles)

	if m.showTasks {
		content += m.renderTaskPanel()
	}

	m.base.Viewport.SetContent(content)
}

func (m *AgentChatModel) refreshViewport() {
	// viewport content is set in rebuildContent
}

func (m *AgentChatModel) renderTaskPanel() string {
	var sb strings.Builder
	tasks := m.tracker.All()
	if len(tasks) == 0 {
		return agentTaskStyle.Render("No background tasks.") + "\n"
	}

	sb.WriteString(agentTaskStyle.Render("--- Background Tasks ---") + "\n")
	for _, t := range tasks {
		status := string(t.Status)
		elapsed := time.Since(t.StartedAt).Round(time.Second)
		line := fmt.Sprintf("  [%s] %s — %s (%s)", t.ID, t.Label, status, elapsed)
		if t.Error != nil {
			line += fmt.Sprintf(" — error: %v", t.Error)
		}
		sb.WriteString(agentTaskStyle.Render(line) + "\n")
	}
	return sb.String()
}

func (m *AgentChatModel) startAgentStream(userInput string) {
	ch := make(chan tea.Msg, 100)
	m.streamCh = ch

	streamCtx, cancel := context.WithCancel(m.ctx)
	m.cancelStream = cancel

	a := m.buildAgent()

	go func() {
		defer close(ch)
		result, err := a.Run(streamCtx, userInput, m.sessionID, func(delta string) {
			ch <- components.StreamDeltaMsg{Delta: delta}
		})
		if err != nil {
			ch <- components.StreamDoneMsg{Err: err}
			return
		}
		if result != nil && result.SessionID != "" {
			m.sessionID = result.SessionID
		}
		ch <- components.StreamDoneMsg{}
	}()
}

func (m *AgentChatModel) buildAgent() *agent.Agent {
	tools := []runtime.Tool{
		booktools.NewReadFileTool(m.bookDir),
		booktools.NewWriteFileTool(m.bookDir),
		booktools.NewSearchResearchTool(m.bookDir),
		booktools.NewRunQATool(m.bookDir, m.cfg),
		booktools.NewWebFetchTool(),
		booktools.NewListChaptersTool(m.bookDir, m.cfg),
		booktools.NewKnowledgeSearchTool(m.bookDir),
		booktools.NewGrepChunksTool(m.bookDir),
	}
	if m.llmClient != nil {
		spawn := (&booktools.SpawnTools{
			Spawner:       m.tracker,
			BookDir:       m.bookDir,
			Cfg:           m.cfg,
			WriterFactory: &agentWriterFactory{provider: m.fantasyProvider, db: m.sqlDB},
			LLMClient:     m.llmClient,
		}).All()
		tools = append(tools, spawn...)
		tools = append(tools, booktools.NewRunPlannerTool(m.bookDir, m.cfg, m.llmClient, m.sqlDB))
		tools = append(tools, booktools.NewSpawnSwarmTool(m.tracker, m.bookDir, m.cfg, m.llmClient))
		tools = append(tools, booktools.NewRunReflectionTool(m.tracker, m.bookDir, m.cfg, m.llmClient))
		tools = append(tools, booktools.NewReflectionTool(m.bookDir, m.cfg, m.llmClient, m.sqlDB))

		registry := runtime.NewToolRegistry()
		for _, t := range tools {
			registry.Register(t)
		}
		tools = append(tools, booktools.NewPlanAndExecuteTool(registry, m.llmClient, m.sqlDB))
	}
	return agent.New(
		m.fantasyProvider,
		m.modelID,
		m.bookDir,
		m.cfg,
		agent.WithAnalysis(m.analysis),
		agent.WithTools(tools),
		agent.WithSessionStore(db.NewSessionStore(m.sqlDB)),
		agent.WithEpistemicStore(knowledge.NewEpistemicStore(m.sqlDB)),
	)
}

func (m *AgentChatModel) handleTaskComplete(msg components.TaskCompleteMsg) {
	task, ok := m.tracker.Get(msg.TaskID)
	if !ok {
		return
	}

	var content string
	if msg.Error != nil {
		content = fmt.Sprintf("[Background] %s — FAILED: %v", task.Label, msg.Error)
	} else {
		content = fmt.Sprintf("[Background] %s — Complete: %s", task.Label, msg.Result)
	}
	m.addEntry(components.RoleSystem, content)
}

// RunAgentChat starts the interactive agent chat TUI.
func RunAgentChat(
	bookDir string,
	cfg *config.BookConfig,
	fantasyProvider fantasy.Provider,
	modelID string,
	sqlDB *sql.DB,
	llmClient llm.Provider,
) error {
	ctx := context.Background()
	m := newAgentChatModel(ctx, bookDir, cfg, fantasyProvider, modelID, sqlDB)
	m.llmClient = llmClient

	p := tea.NewProgram(m, tea.WithAltScreen())
	m.tracker.SetProgram(p)
	_, err := p.Run()
	return err
}

// agentWriterFactory creates agentic writers for background spawn tasks.
type agentWriterFactory struct {
	provider fantasy.Provider
	db       *sql.DB
	bookDir  string
	cfg      *config.BookConfig
}

func (f *agentWriterFactory) NewWriter(bookDir string, cfg any, modelID string) runtime.Runnable {
	bookCfg := cfg.(*config.BookConfig)
	tools := []runtime.Tool{
		booktools.NewReadFileTool(bookDir),
		booktools.NewWriteFileTool(bookDir),
		booktools.NewSearchResearchTool(bookDir),
		booktools.NewRunQATool(bookDir, bookCfg),
		booktools.NewWebFetchTool(),
		booktools.NewListChaptersTool(bookDir, bookCfg),
		booktools.NewKnowledgeSearchTool(bookDir),
		booktools.NewGrepChunksTool(bookDir),
	}
	return agent.New(f.provider, modelID, bookDir, bookCfg,
		agent.WithTools(tools),
		agent.WithSessionStore(db.NewSessionStore(f.db)),
		agent.WithEpistemicStore(knowledge.NewEpistemicStore(f.db)),
	)
}
