package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/wordcount"
)

// StatusCmd returns the `book status` command.
func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st", "info"},
		Short:   "Show chapter progress, git status, and last QA summary",
		Long: `Display a summary of the current book project.

Shows chapter completion status with word count progress bars, recent
git commits, and any uncommitted changes. Run from a book directory
or any subdirectory containing book.yaml.`,
		Example: `  nqb status
  nqb st`,
		GroupID: "publishing",
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			printBookStatus(bookDir, cfg)
			printGitStatus(bookDir)
			return nil
		},
	}
}

func printBookStatus(bookDir string, cfg *config.BookConfig) {
	rules, _ := config.LoadRules(bookDir)

	fmt.Printf("\n📚 %s\n", cfg.Title)
	fmt.Printf("   by %s | Language: %s | Domain: %s\n", cfg.Author, cfg.Language, cfg.Domain)
	fmt.Printf("\nChapters:\n")
	fmt.Printf("  %-4s  %-30s  %-8s  %-28s\n", "Num", "Title", "Status", "Words")
	fmt.Printf("  %-4s  %-30s  %-8s  %-28s\n", "---", "-----", "------", "-----")

	var totalWords, totalTarget int

	for _, ch := range cfg.Chapters {
		status := ch.Status
		if status == "" {
			status = "pending"
		}

		var p wordcount.Progress
		p.Target = rules.WordCount.Target
		p.Min = rules.WordCount.Min
		p.Max = rules.WordCount.Max
		totalTarget += p.Target

		chapPath := filepath.Join(bookDir, "chapters", ch.File)
		if data, err := os.ReadFile(chapPath); err == nil {
			p.Words = wordcount.Count(string(data))
			totalWords += p.Words
			if status == "pending" {
				status = "written"
			}
		}

		icon := "○"
		switch status {
		case "written":
			icon = "◑"
		case "reviewed", "done":
			icon = "●"
		}

		title := ch.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}

		bar := "—"
		if p.Words > 0 {
			bar = wordcount.Bar(p, 12)
		}

		fmt.Printf("  %-4d  %-30s  %s %-6s  %s\n", ch.Number, title, icon, status, bar)
	}

	if len(cfg.Chapters) > 0 {
		total := wordcount.Progress{Words: totalWords, Target: totalTarget}
		fmt.Printf("\n  Total: %s\n", wordcount.Bar(total, 16))
	}
}

func printGitStatus(bookDir string) {
	gitDir := filepath.Join(bookDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Printf("\nGit: not initialized\n")
		return
	}

	fmt.Printf("\nGit:\n")
	logCmd := exec.Command("git", "-C", bookDir, "log", "--oneline", "-5")
	logOut, err := logCmd.Output()
	if err == nil && len(logOut) > 0 {
		fmt.Printf("  Recent commits:\n")
		for _, line := range splitLines(string(logOut)) {
			fmt.Printf("    %s\n", line)
		}
	}

	statusCmd := exec.Command("git", "-C", bookDir, "status", "--short")
	statusOut, err := statusCmd.Output()
	if err == nil && len(statusOut) > 0 {
		fmt.Printf("  Uncommitted changes:\n")
		for _, line := range splitLines(string(statusOut)) {
			fmt.Printf("    %s\n", line)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			if current != "" {
				lines = append(lines, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
