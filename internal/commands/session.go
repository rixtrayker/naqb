package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
)

// SessionCmd returns the `nqb session` command group.
func SessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "session",
		Aliases: []string{"sess"},
		Short:   "Manage agent chat sessions",
		Long: `List, view, and delete agent chat sessions.

Sessions record all LLM interactions during write, fix, and chat operations.
Use 'nqb session list' to see recent sessions, 'nqb session show <id>' to
view messages, and 'nqb session delete <id>' to remove a session.`,
		GroupID: "management",
	}
	cmd.AddCommand(
		sessionListCmd(),
		sessionShowCmd(),
		sessionDeleteCmd(),
	)
	return cmd
}

// sessionListCmd is `nqb session list`.
func sessionListCmd() *cobra.Command {
	var chapterNum int
	var limit int

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent agent sessions for this book",
		Long: `List recent agent sessions for the current book project.

Shows session IDs, chapter numbers, and creation timestamps. Filter by
chapter with --chapter, or adjust the number of results with --limit.`,
		Example: `  nqb session list
  nqb session ls
  nqb session list --chapter 3
  nqb sess ls --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}

			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			sessions, err := db.ListSessions(sqlDB, bookDir, limit)
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("  No sessions found.")
				return nil
			}

			fmt.Printf("  %-36s  %-6s  %s\n", "Session ID", "Ch", "Created")
			fmt.Printf("  %s\n", repeat("─", 60))
			for _, s := range sessions {
				if chapterNum > 0 && s.ChapterNum != chapterNum {
					continue
				}
				chStr := "—"
				if s.ChapterNum > 0 {
					chStr = fmt.Sprintf("%d", s.ChapterNum)
				}
				fmt.Printf("  %-36s  %-6s  %s\n",
					s.ID, chStr, s.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Filter by chapter number")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sessions to show")
	return cmd
}

// sessionShowCmd is `nqb session show <id>`.
func sessionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <session-id>",
		Short: "Show messages from a session",
		Long: `Display all messages from a specific agent session.

Shows the full conversation including role labels, token counts, and
message content (truncated at 1000 chars per message).`,
		Example: `  nqb session show 550e8400-e29b-41d4-a716-446655440000
  nqb sess show 550e8400`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			sessionID := args[0]
			sess, err := db.GetSession(sqlDB, sessionID)
			if err != nil {
				return err
			}

			fmt.Printf("Session: %s\n", sess.ID)
			fmt.Printf("Book:    %s\n", sess.BookDir)
			if sess.ChapterNum > 0 {
				fmt.Printf("Chapter: %d\n", sess.ChapterNum)
			}
			fmt.Printf("Created: %s\n\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))

			msgs, err := db.GetSessionMessages(sqlDB, sessionID)
			if err != nil {
				return err
			}

			for _, m := range msgs {
				role := m.Role
				tokStr := ""
				if m.TokensIn > 0 || m.TokensOut > 0 {
					tokStr = fmt.Sprintf(" [%d/%d tok]", m.TokensIn, m.TokensOut)
				}
				fmt.Printf("── %s%s ──────────────────────\n", role, tokStr)
				content := m.Content
				if len(content) > 1000 {
					content = content[:1000] + "\n[... truncated]"
				}
				fmt.Println(content)
				fmt.Println()
			}
			return nil
		},
	}
}

// sessionDeleteCmd is `nqb session delete <id>`.
func sessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <session-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a session and all its messages",
		Long:    `Permanently delete a session and all its messages from the database.`,
		Example: `  nqb session delete 550e8400-e29b-41d4-a716-446655440000
  nqb sess rm 550e8400`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			sessionID := args[0]
			if err := db.DeleteSession(sqlDB, sessionID); err != nil {
				return err
			}
			fmt.Printf("  ✓ session %s deleted\n", sessionID[:min(8, len(sessionID))])
			return nil
		},
	}
}
