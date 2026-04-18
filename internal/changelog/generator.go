// Package changelog generates human-friendly session changelogs from git history.
package changelog

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Entry is one line in the generated changelog.
type Entry struct {
	Category string
	Message  string
}

// SessionReport contains the parsed changelog for a writing session.
type SessionReport struct {
	Date    string
	Entries []Entry
	Raw     string
}

// Generate runs git log in bookDir and returns a session report.
func Generate(bookDir string, limit int) (*SessionReport, error) {
	if limit <= 0 {
		limit = 20
	}

	cmd := exec.Command("git", "-C", bookDir, "log", "--pretty=format:%s", "-n", fmt.Sprintf("%d", limit))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	raw := string(out)
	if strings.TrimSpace(raw) == "" {
		return &SessionReport{
			Date: time.Now().Format("2006-01-02"),
			Raw:  "No commits found.",
		}, nil
	}

	lines := strings.Split(raw, "\n")
	var entries []Entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cat := categorize(line)
		entries = append(entries, Entry{Category: cat, Message: humanize(line)})
	}

	return &SessionReport{
		Date:    time.Now().Format("2006-01-02"),
		Entries: entries,
		Raw:     raw,
	}, nil
}

// categorize maps a commit message prefix to a category.
func categorize(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.HasPrefix(lower, "init:"):
		return "Setup"
	case strings.HasPrefix(lower, "chapter("),
		strings.HasPrefix(lower, "draft("),
		strings.Contains(lower, "first draft"),
		strings.Contains(lower, "draft written"):
		return "Writing"
	case strings.HasPrefix(lower, "context("),
		strings.Contains(lower, "context assembled"):
		return "Context"
	case strings.HasPrefix(lower, "qa("),
		strings.Contains(lower, "qa complete"),
		strings.Contains(lower, "qa passed"):
		return "Quality"
	case strings.HasPrefix(lower, "research"),
		strings.Contains(lower, "research note"):
		return "Research"
	case strings.HasPrefix(lower, "export("),
		strings.Contains(lower, "pdf generated"),
		strings.Contains(lower, "epub generated"):
		return "Publishing"
	case strings.HasPrefix(lower, "fix"),
		strings.Contains(lower, "fix "):
		return "Fixes"
	case strings.HasPrefix(lower, "refactor"):
		return "Refactoring"
	default:
		return "Other"
	}
}

// humanize turns a git commit message into a sentence-style summary.
func humanize(msg string) string {
	// Remove common prefixes
	msg = strings.TrimPrefix(msg, "init: ")
	msg = strings.TrimPrefix(msg, "chapter(")
	msg = strings.TrimPrefix(msg, "draft(")
	msg = strings.TrimPrefix(msg, "context(")
	msg = strings.TrimPrefix(msg, "qa(")
	msg = strings.TrimPrefix(msg, "export(")
	msg = strings.TrimPrefix(msg, "research: ")

	// Remove trailing ): from prefix markers
	msg = regexp.MustCompile(`^\d+\):\s*`).ReplaceAllString(msg, "")
	msg = regexp.MustCompile(`^[^)]+\):\s*`).ReplaceAllString(msg, "")

	// Capitalize first letter
	msg = strings.TrimSpace(msg)
	if len(msg) > 0 {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return msg
}

// FormatMarkdown renders the session report as a markdown changelog.
func FormatMarkdown(r *SessionReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Writing Session — %s\n\n", r.Date))

	if len(r.Entries) == 0 {
		sb.WriteString("No commits found in this session.\n")
		return sb.String()
	}

	// Group by category
	groups := make(map[string][]string)
	for _, e := range r.Entries {
		groups[e.Category] = append(groups[e.Category], e.Message)
	}

	order := []string{"Writing", "Context", "Research", "Quality", "Publishing", "Fixes", "Refactoring", "Setup", "Other"}
	for _, cat := range order {
		items, ok := groups[cat]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", cat))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
