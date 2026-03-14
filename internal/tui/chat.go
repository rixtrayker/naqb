package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/llm"
)

var (
	chatTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 1)
	userMsgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).PaddingLeft(2)
	assistantStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).PaddingLeft(2)
	chatHintStyle    = lipgloss.NewStyle().Faint(true).PaddingLeft(2)
	chatBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
)

// streamDeltaMsg carries a streamed text chunk from the LLM.
type streamDeltaMsg struct {
	delta string
}

// streamDoneMsg signals that streaming is complete.
type streamDoneMsg struct {
	err error
}

// ChatModel is the Bubble Tea model for the book chat REPL.
type ChatModel struct {
	client    llm.Provider
	model     string
	system    string
	history   []llm.Message
	viewport  viewport.Model
	textarea  textarea.Model
	content   string // rendered chat history
	streaming bool
	width     int
	height    int
	err       string
	ctx       context.Context
}

// NewChatModel creates a new chat REPL model.
func NewChatModel(ctx context.Context, client llm.Provider, model, system string) *ChatModel {
	ta := textarea.New()
	ta.Placeholder = "Ask about your book... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)

	return &ChatModel{
		client:   client,
		model:    model,
		system:   system,
		viewport: vp,
		textarea: ta,
		ctx:      ctx,
	}
}

func (m *ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		m.textarea.SetWidth(msg.Width - 4)
		m.updateViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if msg.Alt {
				// Alt+Enter: newline in textarea
				break
			}
			if m.streaming {
				return m, nil
			}
			userInput := strings.TrimSpace(m.textarea.Value())
			if userInput == "" {
				return m, nil
			}
			m.textarea.Reset()
			m.addUserMessage(userInput)
			m.streaming = true
			cmds = append(cmds, m.sendMessage(userInput))
			return m, tea.Batch(cmds...)
		}

	case streamDeltaMsg:
		// Append delta to last assistant message
		if len(m.history) > 0 && m.history[len(m.history)-1].Role == "assistant" {
			m.history[len(m.history)-1].Content += msg.delta
		} else {
			m.history = append(m.history, llm.Message{Role: "assistant", Content: msg.delta})
		}
		m.rebuildContent()
		m.updateViewport()
		m.viewport.GotoBottom()

	case streamDoneMsg:
		m.streaming = false
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		m.updateViewport()
	}

	// Update sub-components
	var taCmd tea.Cmd
	m.textarea, taCmd = m.textarea.Update(msg)
	cmds = append(cmds, taCmd)

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *ChatModel) View() string {
	title := chatTitleStyle.Render("نقب  Chat — Claude Opus")
	chatArea := chatBorderStyle.Render(m.viewport.View())

	// Status line: shows thinking indicator, error, or keybinding hints
	var statusLine string
	switch {
	case m.streaming:
		statusLine = chatHintStyle.Render("⟳ Claude is thinking...")
	case m.err != "":
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: " + m.err)
	default:
		statusLine = renderHintBar(ChatBindings)
	}

	input := chatBorderStyle.Render(m.textarea.View())
	return fmt.Sprintf("%s\n%s\n%s\n%s", title, chatArea, statusLine, input)
}

func (m *ChatModel) addUserMessage(text string) {
	m.history = append(m.history, llm.Message{Role: "user", Content: text})
	m.rebuildContent()
	m.updateViewport()
	m.viewport.GotoBottom()
}

func (m *ChatModel) rebuildContent() {
	var sb strings.Builder
	for _, msg := range m.history {
		if msg.Role == "user" {
			sb.WriteString(userMsgStyle.Render("You: "+msg.Content) + "\n\n")
		} else {
			sb.WriteString(assistantStyle.Render("Claude: "+msg.Content) + "\n\n")
		}
	}
	m.content = sb.String()
}

func (m *ChatModel) updateViewport() {
	m.viewport.SetContent(m.content)
}

func (m *ChatModel) sendMessage(text string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg, 100)

		go func() {
			_, err := m.client.Stream(m.ctx, m.model, m.system, m.history, 4096, func(delta string) error {
				ch <- streamDeltaMsg{delta: delta}
				return nil
			})
			ch <- streamDoneMsg{err: err}
			close(ch)
		}()

		// Return first message
		return <-ch
	}
}

// We need a way to batch stream messages. Override Update to handle channel draining.
// The standard approach is to use a command that reads from the channel.

// RunChat starts the Bubble Tea chat REPL.
func RunChat(ctx context.Context, client llm.Provider, model, system string) error {
	m := NewChatModel(ctx, client, model, system)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
