package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/gdocs"
)

// SyncCmd returns the `nqb sync` command with subcommands.
func SyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Sync book content with external services",
		GroupID: "management",
	}
	cmd.AddCommand(syncGDocsCmd())
	return cmd
}

func syncGDocsCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "gdocs",
		Short: "Push all written chapters to a single master Google Doc",
		Long: `Pushes all written chapters into one master Google Doc.

First run: creates a new document and saves its ID to book.yaml.
Subsequent runs: updates the same document in-place.

Each chapter becomes a top-level heading section. Editors can leave
comments and suggestions in Google's native UI without touching git.

Requires COMPOSIO_API_KEY (stored in macOS Keychain by nqb setup).`,
		Example: `  nqb sync gdocs
  nqb sync gdocs --force-new`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			apiKey := config.ComposioAPIKey()
			if apiKey == "" {
				return fmt.Errorf("COMPOSIO_API_KEY not found — store it with: security add-generic-password -a $USER -s COMPOSIO_API_KEY -w <key>")
			}

			userID := cfg.Sync.ComposioUserID
			if userID == "" {
				userID = "pg-test-f3eaa561-6583-4190-9d84-06e15fd4b522"
			}

			client := gdocs.New(apiKey, userID)
			ctx := context.Background()

			// Find or create the master doc
			existingID := cfg.Sync.GDocsID
			if force {
				existingID = "" // force new doc creation
			}

			fmt.Print("  Finding/creating master Google Doc... ")
			doc, err := client.FindOrCreateBookDoc(ctx, existingID, cfg.Title)
			if err != nil {
				return fmt.Errorf("Google Docs: %w", err)
			}
			fmt.Printf("✓\n  %s\n", doc.URL)

			// Collect written chapters
			var chapters []gdocs.ChapterContent
			skipped := 0
			for _, ch := range cfg.Chapters {
				fname := config.ChapterFilename(ch.Number)
				path := filepath.Join(bookDir, "chapters", fname)
				data, err := os.ReadFile(path)
				if err != nil {
					skipped++
					continue
				}
				chapters = append(chapters, gdocs.ChapterContent{
					Number: ch.Number,
					Title:  ch.Title,
					Body:   string(data),
				})
			}

			if len(chapters) == 0 {
				return fmt.Errorf("no written chapters found — run `nqb write --chapter N` first")
			}

			fmt.Printf("  Syncing %d chapters (%d skipped/unwritten)... ", len(chapters), skipped)
			content := gdocs.BuildDocContent(chapters)
			if err := client.ReplaceContent(ctx, doc.ID, content); err != nil {
				// Non-fatal: log and continue — doc was created, just content push failed
				fmt.Printf("⚠ content push failed: %v\n", err)
				fmt.Printf("  Doc created but empty — open and paste manually: %s\n", doc.URL)
			} else {
				fmt.Printf("✓\n")
			}

			// Save doc ID back to book.yaml if new
			if cfg.Sync.GDocsID != doc.ID {
				cfg.Sync.GDocsID = doc.ID
				cfg.Sync.GDocsURL = doc.URL
				if cfg.Sync.ComposioUserID == "" {
					cfg.Sync.ComposioUserID = userID
				}
				if err := config.SaveBook(bookDir, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "  Warning: could not save doc ID to book.yaml: %v\n", err)
				} else {
					fmt.Println("  Doc ID saved to book.yaml.")
				}
			}

			fmt.Printf("\n✓ Sync complete: %s\n", doc.URL)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force-new", false, "Create a new document even if one already exists")
	return cmd
}
