package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/amr/naqb/internal/tui/theme"
)

// ChatRole identifies the speaker in a chat entry.
type ChatRole string

const (
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
	RoleSystem    ChatRole = "system"
)

// ChatEntry is one message in the chat history.
type ChatEntry struct {
	Role    ChatRole
	Content string
}

// StreamDeltaMsg carries a streamed text chunk.
type StreamDeltaMsg struct {
	Delta string
}

// StreamDoneMsg signals that streaming is complete.
type StreamDoneMsg struct {
	Err error
}

// WaitForStream returns a cmd that reads the next message from the stream channel.
func WaitForStream(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// ChatBase provides reusable viewport + textarea layout for chat screens.
type ChatBase struct {
	Viewport viewport.Model
	Textarea textarea.Model
	Width    int
	Height   int
}

// NewChatBase creates a new ChatBase with default sizing.
func NewChatBase() ChatBase {
	ta := textarea.New()
	ta.Placeholder = "Ask about your book... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)

	return ChatBase{
		Viewport: vp,
		Textarea: ta,
	}
}

// HandleWindowSize updates dimensions when the terminal is resized.
func (c *ChatBase) HandleWindowSize(w, h int) {
	c.Width = w
	c.Height = h
	c.Viewport.Width = w - 4
	c.Viewport.Height = h - 8
	c.Textarea.SetWidth(w - 4)
}

// ChatStyles groups common lipgloss styles for chat rendering.
type ChatStyles struct {
	TitleStyle   lipgloss.Style
	UserStyle    lipgloss.Style
	AssistStyle  lipgloss.Style
	SystemStyle  lipgloss.Style
	BorderStyle  lipgloss.Style
	StatusStyle  lipgloss.Style
}

// DefaultChatStyles returns a sensible default style set.
func DefaultChatStyles() ChatStyles {
	return ChatStyles{
		TitleStyle:  lipgloss.NewStyle().Bold(true).Foreground(theme.ColorSecondary).Padding(0, 1),
		UserStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("212")).PaddingLeft(2),
		AssistStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("86")).PaddingLeft(2),
		SystemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).PaddingLeft(2).Italic(true),
		BorderStyle: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")),
		StatusStyle: lipgloss.NewStyle().Faint(true).PaddingLeft(2),
	}
}

// BuildChatContent renders entries into a display string.
func BuildChatContent(entries []ChatEntry, styles ChatStyles) string {
	var sb strings.Builder
	for _, e := range entries {
		switch e.Role {
		case RoleUser:
			sb.WriteString(styles.UserStyle.Render("You: "+e.Content) + "\n\n")
		case RoleAssistant:
			sb.WriteString(styles.AssistStyle.Render("نقب: "+e.Content) + "\n\n")
		case RoleSystem:
			sb.WriteString(styles.SystemStyle.Render(e.Content) + "\n\n")
		}
	}
	return sb.String()
}

// ApplyDelta appends a delta to the last assistant entry, or creates a new one.
func ApplyDelta(entries []ChatEntry, delta string) []ChatEntry {
	if n := len(entries); n > 0 && entries[n-1].Role == RoleAssistant {
		entries[n-1].Content += delta
	} else {
		entries = append(entries, ChatEntry{Role: RoleAssistant, Content: delta})
	}
	return entries
}
