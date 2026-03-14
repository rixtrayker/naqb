package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/wordcount"
)

// sidebarTab identifies which sidebar panel is active.
type sidebarTab int

const (
	tabActions sidebarTab = iota
	tabNotes
	tabTodos
	tabStats
	tabQA
	tabGit
	tabCount // sentinel — keep last
)

var tabNames = [tabCount]string{
	"Actions", "Notes", "Todos", "Stats", "QA", "Git",
}

// tabBarStyle renders the tab bar at the top of the sidebar.
var (
	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("245"))
	sectionHeader    = lipgloss.NewStyle().Bold(true).Faint(true).PaddingLeft(1)
	noteItemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).PaddingLeft(1)
	statOKStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statBadStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// renderTabBar returns the tab strip for the sidebar.
func renderTabBar(active sidebarTab, width int) string {
	var parts []string
	for i := sidebarTab(0); i < tabCount; i++ {
		name := tabNames[i]
		// Abbreviate to fit sidebar width
		if width < 24 && len(name) > 4 {
			name = name[:3]
		}
		if i == active {
			parts = append(parts, tabActiveStyle.Render(name))
		} else {
			parts = append(parts, tabInactiveStyle.Render(name))
		}
	}
	bar := strings.Join(parts, " ")
	return lipgloss.NewStyle().PaddingLeft(1).Render(bar)
}

// renderSidebarContent returns the body content for the active tab.
func renderSidebarContent(tab sidebarTab, bookDir string, cfg *config.BookConfig, actions []sidebarAction) string {
	switch tab {
	case tabActions:
		return renderActionsTab(actions)
	case tabNotes:
		return renderNotesTab(bookDir)
	case tabTodos:
		return renderTodosTab(bookDir)
	case tabStats:
		return renderStatsTab(bookDir, cfg)
	case tabQA:
		return renderQATab(bookDir)
	case tabGit:
		return renderGitTab(bookDir)
	default:
		return ""
	}
}

func renderActionsTab(actions []sidebarAction) string {
	var sb strings.Builder
	sb.WriteString(sectionHeader.Render("ACTIONS") + "\n\n")
	for _, a := range actions {
		line := fmt.Sprintf("%s  %s",
			bvSidebarKeyStyle.Render(a.key),
			bvSidebarLabelStyle.Render(a.label),
		)
		sb.WriteString(" " + line + "\n")
	}
	return sb.String()
}

func renderNotesTab(bookDir string) string {
	notesDir := filepath.Join(bookDir, ".naqb", "notes")
	return renderFileList(notesDir, "NOTES", "No notes yet.\nCreate files in .naqb/notes/")
}

func renderTodosTab(bookDir string) string {
	todosDir := filepath.Join(bookDir, ".naqb", "todos")
	return renderFileList(todosDir, "TODOS", "No todos yet.\nCreate files in .naqb/todos/")
}

func renderFileList(dir, heading, emptyMsg string) string {
	var sb strings.Builder
	sb.WriteString(sectionHeader.Render(heading) + "\n\n")

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		sb.WriteString(noteItemStyle.Render(emptyMsg) + "\n")
		return sb.String()
	}

	shown := 0
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		// Strip extension for display
		if idx := strings.LastIndex(name, "."); idx > 0 {
			name = name[:idx]
		}
		if len(name) > 18 {
			name = name[:15] + "..."
		}
		sb.WriteString(noteItemStyle.Render("• "+name) + "\n")
		shown++
		if shown >= 10 {
			remaining := 0
			for _, e2 := range entries {
				if !e2.IsDir() && !strings.HasPrefix(e2.Name(), ".") {
					remaining++
				}
			}
			remaining -= shown
			if remaining > 0 {
				sb.WriteString(noteItemStyle.Faint(true).Render(fmt.Sprintf("  +%d more…", remaining)) + "\n")
			}
			break
		}
	}
	return sb.String()
}

func renderStatsTab(bookDir string, cfg *config.BookConfig) string {
	rules, _ := config.LoadRules(bookDir)

	var sb strings.Builder
	sb.WriteString(sectionHeader.Render("STATS") + "\n\n")

	var total int
	for _, ch := range cfg.Chapters {
		path := filepath.Join(bookDir, "chapters", ch.File)
		wc, err := wordcount.CountFile(path)
		if err != nil {
			continue
		}
		total += wc

		p := wordcount.Progress{
			Words:  wc,
			Target: rules.WordCount.Target,
			Min:    rules.WordCount.Min,
			Max:    rules.WordCount.Max,
		}

		label := fmt.Sprintf("Ch%02d", ch.Number)
		countStr := fmt.Sprintf("%dw", wc)
		style := statOKStyle
		switch p.Status() {
		case "short":
			style = statWarnStyle
		case "long":
			style = statBadStyle
		case "empty":
			style = tabInactiveStyle
			countStr = "—"
		}

		line := fmt.Sprintf(" %-5s %s", label, style.Render(countStr))
		sb.WriteString(line + "\n")
	}

	if len(cfg.Chapters) > 0 {
		sb.WriteString("\n")
		sb.WriteString(noteItemStyle.Render(fmt.Sprintf(" Total: %dw", total)) + "\n")
		if rules.WordCount.Target > 0 {
			goal := rules.WordCount.Target * len(cfg.Chapters)
			sb.WriteString(noteItemStyle.Faint(true).Render(fmt.Sprintf(" Goal:  %dw", goal)) + "\n")
		}
	}
	return sb.String()
}

func renderQATab(bookDir string) string {
	var sb strings.Builder
	sb.WriteString(sectionHeader.Render("QA") + "\n\n")

	reportPath := filepath.Join(bookDir, "pipeline-report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		sb.WriteString(noteItemStyle.Render("No QA report yet.\nRun: nqb qa -c N") + "\n")
		return sb.String()
	}

	// Show last ~15 non-empty lines of the report
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var tail []string
	for i := len(lines) - 1; i >= 0 && len(tail) < 15; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			tail = append([]string{lines[i]}, tail...)
		}
	}

	for _, line := range tail {
		// Trim markdown decorations for compact display
		line = strings.TrimPrefix(line, "## ")
		line = strings.TrimPrefix(line, "**")
		line = strings.TrimSuffix(line, "**")
		line = strings.TrimPrefix(line, "- ")
		if len(line) > 20 {
			line = line[:17] + "…"
		}
		if strings.Contains(line, "PASSED") {
			sb.WriteString(" " + statOKStyle.Render(line) + "\n")
		} else if strings.Contains(line, "ISSUES") || strings.Contains(line, "issue") {
			sb.WriteString(" " + statWarnStyle.Render(line) + "\n")
		} else {
			sb.WriteString(noteItemStyle.Render(" "+line) + "\n")
		}
	}
	return sb.String()
}

func renderGitTab(bookDir string) string {
	var sb strings.Builder
	sb.WriteString(sectionHeader.Render("GIT") + "\n\n")

	gitDir := filepath.Join(bookDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		sb.WriteString(noteItemStyle.Render("Not a git repo.\nnqb init creates git.") + "\n")
		return sb.String()
	}

	out, err := exec.Command("git", "-C", bookDir, "log", "--oneline", "-8").Output()
	if err != nil || len(out) == 0 {
		sb.WriteString(noteItemStyle.Render("No commits yet.") + "\n")
		return sb.String()
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) > 20 {
			line = line[:17] + "…"
		}
		sb.WriteString(noteItemStyle.Render(" "+line) + "\n")
	}
	return sb.String()
}
