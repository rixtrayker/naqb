package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/bubbles/textarea"

	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/internal/tui/components"
	"github.com/amr/naqb/internal/tui/keys"
	"github.com/amr/naqb/internal/tui/theme"
)

var (
	chatTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 1)
	chatHintStyle   = lipgloss.NewStyle().Faint(true).PaddingLeft(2)
	chatBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
)

// ChatModel is the Bubble Tea model for the book chat REPL.
type ChatModel struct {
	base      components.ChatBase
	client    llm.Provider
	model     string
	system    string
	history   []llm.Message
	entries   []components.ChatEntry
	streaming bool
	err       string
	ctx       context.Context
	streamCh  chan tea.Msg
	cancelFn  context.CancelFunc
}

// NewChatModel creates a new chat REPL model.
func NewChatModel(ctx context.Context, client llm.Provider, model, system string) *ChatModel {
	return &ChatModel{
		base:    components.NewChatBase(),
		client:  client,
		model:   model,
		system:  system,
		ctx:     ctx,
		entries: []components.ChatEntry{},
	}
}

func (m *ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.base.HandleWindowSize(msg.Width, msg.Height)
		m.refreshViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.cancelFn != nil {
				m.cancelFn()
				m.cancelFn = nil
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if msg.Alt {
				break
			}
			if m.streaming {
				return m, nil
			}
			userInput := m.base.Textarea.Value()
			if userInput == "" {
				return m, nil
			}
			m.base.Textarea.Reset()
			m.addUserMessage(userInput)
			m.streaming = true
			m.startStream(userInput)
			cmds = append(cmds, components.WaitForStream(m.streamCh))
			return m, tea.Batch(cmds...)
		}

	case components.StreamDeltaMsg:
		wasAtBottom := m.base.Viewport.AtBottom()
		m.entries = components.ApplyDelta(m.entries, msg.Delta)
		m.refreshViewport()
		if wasAtBottom {
			m.base.Viewport.GotoBottom()
		}
		cmds = append(cmds, components.WaitForStream(m.streamCh))

	case components.StreamDoneMsg:
		m.streaming = false
		m.streamCh = nil
		m.cancelFn = nil
		if msg.Err != nil {
			m.err = msg.Err.Error()
		}
		m.refreshViewport()
	}

	var taCmd tea.Cmd
	m.base.Textarea, taCmd = m.base.Textarea.Update(msg)
	cmds = append(cmds, taCmd)

	var vpCmd tea.Cmd
	m.base.Viewport, vpCmd = m.base.Viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *ChatModel) View() string {
	title := chatTitleStyle.Render("\u2067نقب\u2069  Chat — Claude Opus")
	chatArea := chatBorderStyle.Render(m.base.Viewport.View())

	var statusLine string
	switch {
	case m.streaming:
		statusLine = chatHintStyle.Render("⟳ Claude is thinking...")
	case m.err != "":
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: " + m.err)
	default:
		statusLine = keys.ChatBindings[0].Key
		statusLine = theme.RenderHintBar(keys.ChatBindings)
	}

	input := chatBorderStyle.Render(m.base.Textarea.View())
	return fmt.Sprintf("%s\n%s\n%s\n%s", title, chatArea, statusLine, input)
}

func (m *ChatModel) addUserMessage(text string) {
	m.history = append(m.history, llm.Message{Role: "user", Content: text})
	m.entries = append(m.entries, components.ChatEntry{Role: components.RoleUser, Content: text})
	m.refreshViewport()
	m.base.Viewport.GotoBottom()
}

func (m *ChatModel) refreshViewport() {
	styles := components.ChatStyles{
		UserStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("212")).PaddingLeft(2),
		AssistStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("86")).PaddingLeft(2),
	}
	m.base.Viewport.SetContent(components.BuildChatContent(m.entries, styles))
}

func (m *ChatModel) startStream(text string) {
	ch := make(chan tea.Msg, 100)
	m.streamCh = ch

	streamCtx, cancel := context.WithCancel(m.ctx)
	m.cancelFn = cancel

	go func() {
		defer close(ch)
		_, err := m.client.Stream(streamCtx, m.model, m.system, m.history, 4096, func(delta string) error {
			ch <- components.StreamDeltaMsg{Delta: delta}
			return nil
		})
		ch <- components.StreamDoneMsg{Err: err}
	}()
}

// RunChat starts the Bubble Tea chat REPL.
func RunChat(ctx context.Context, client llm.Provider, model, system string) error {
	m := NewChatModel(ctx, client, model, system)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
