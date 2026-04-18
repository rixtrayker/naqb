package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/vault"
	"github.com/amr/naqb/pkg/wordcount"
)

// ListCmd returns the `nqb list` command group.
func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List books, chapters, jobs, or sessions",
		Long: `Unified listing command. Without a subcommand, lists all book
projects in the vault. Use subcommands to list chapters, jobs, or sessions.`,
		Example: `  nqb list
  nqb ls
  nqb list chapters
  nqb ls ch`,
		GroupID: "management",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listBooks()
		},
	}

	cmd.AddCommand(
		listChaptersCmd(),
	)
	return cmd
}

// listBooks lists all books in the vault.
func listBooks() error {
	projects, err := vault.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No books found.")
		fmt.Println("  Run 'nqb init' to create a new book project.")
		fmt.Println("  Run 'nqb open <path>' to open an existing project.")
		return nil
	}

	fmt.Printf("  %-30s  %-10s  %-14s  %s\n", "TITLE", "LANGUAGE", "DOMAIN", "PATH")
	fmt.Printf("  %-30s  %-10s  %-14s  %s\n", "─────", "────────", "──────", "────")
	for _, p := range projects {
		title := p.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}
		domain := p.Domain
		if len(domain) > 12 {
			domain = domain[:9] + "..."
		}
		lang := p.Language
		if lang == "" {
			lang = "—"
		}
		fmt.Printf("  %-30s  %-10s  %-14s  %s\n", title, lang, domain, p.Path)
	}
	fmt.Printf("\n  %d book(s) found.\n", len(projects))
	return nil
}

// listChaptersCmd lists chapters in the current book with status and word count.
func listChaptersCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "chapters",
		Aliases: []string{"ch"},
		Short:   "List chapters in the current book with status and word count",
		Example: `  nqb list chapters
  nqb ls ch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}
			rules, _ := config.LoadRules(bookDir)

			fmt.Printf("\n  %s — Chapters\n\n", cfg.Title)
			fmt.Printf("  %-4s  %-32s  %-10s  %s\n", "NUM", "TITLE", "STATUS", "WORDS")
			fmt.Printf("  %-4s  %-32s  %-10s  %s\n", "───", "─────", "──────", "─────")

			var totalWords int
			for _, ch := range cfg.Chapters {
				status := ch.Status
				if status == "" {
					status = "pending"
				}

				var words int
				chapPath := filepath.Join(bookDir, "chapters", ch.File)
				if data, err := os.ReadFile(chapPath); err == nil {
					words = wordcount.Count(string(data))
					totalWords += words
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
				if len(title) > 30 {
					title = title[:27] + "..."
				}

				wordStr := "—"
				if words > 0 {
					target := rules.WordCount.Target
					if target > 0 {
						pct := float64(words) / float64(target) * 100
						wordStr = fmt.Sprintf("%d (%.0f%%)", words, pct)
					} else {
						wordStr = fmt.Sprintf("%d", words)
					}
				}

				fmt.Printf("  %-4d  %-32s  %s %-8s  %s\n", ch.Number, title, icon, status, wordStr)
			}

			fmt.Printf("\n  Total: %d words across %d chapters\n", totalWords, len(cfg.Chapters))
			return nil
		},
	}
}
