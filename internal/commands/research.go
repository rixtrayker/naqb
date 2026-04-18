package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/keycheck"
	"github.com/amr/naqb/pkg/research"
)

// ResearchCmd returns the `nqb research` command.
func ResearchCmd() *cobra.Command {
	var chapterNum int
	var providerFlag string
	var all bool
	var deep bool
	var fromYouTube string

	cmd := &cobra.Command{
		Use:     "research",
		Aliases: []string{"res"},
		Short:   "Run Scout→Explorer→Scribe research pipeline for a chapter",
		GroupID: "quality",
		Long: `Run the automated research pipeline for a chapter.
  1. Scout   — LLM generates focused search queries
  2. Explorer — fetches results from the configured search API
  3. Scribe  — LLM synthesises atomic notes into .naqb/research/

Set TAVILY_API_KEY or EXA_API_KEY and configure research.search_provider
in config/rules.yaml to enable web search. Without a search key, Scout
still generates queries but Explorer is skipped.

Use --deep to activate Gemini Search grounding (requires GEMINI_API_KEY).
Deep research produces citation-backed synthesis instead of raw snippets.

Use --from-youtube to import a YouTube transcript as research notes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RunPreflight("research"); err != nil {
				return err
			}
			// --deep: soft warn if GEMINI_API_KEY is missing (fallback exists).
			if deep {
				if result := keycheck.CheckCommand("research-deep"); !result.OK {
					fmt.Fprintf(os.Stderr, "  Note: GEMINI_API_KEY not found — --deep will fall back to configured provider.\n")
				}
			}

			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}
			rules, _ := config.LoadRules(bookDir)

			// --deep: route through Gemini Search grounding when key is available
			if deep {
				if rules == nil {
					rules = &config.Rules{}
				}
				_, isDeep, err := research.NewDeepSearcher(rules.Research.SearchProvider)
				if err != nil {
					return fmt.Errorf("deep research init: %w", err)
				}
				if isDeep {
					fmt.Println("  Deep research mode: Gemini Search grounding active.")
					rules.Research.SearchProvider = "gemini"
				}
				// else: preflight already printed the fallback note above.
			}

			client, err := providerFor(providerFlag, cfg.LLM.WriteProvider)
			if err != nil {
				return err
			}
			ctx := context.Background()

			if fromYouTube != "" {
				if chapterNum <= 0 {
					return fmt.Errorf("specify --chapter N when using --from-youtube")
				}
				title := chapterTitle(cfg, chapterNum)
				fmt.Printf("Research — Chapter %d: %s (from YouTube)\n", chapterNum, title)
				result, err := research.RunYouTubeResearch(ctx, client, bookDir, cfg, chapterNum, fromYouTube, os.Stdout)
				if err != nil {
					return err
				}
				fmt.Printf("\n✓ YouTube research complete: %d notes\n", len(result.Notes))
				if len(result.Notes) > 0 {
					fmt.Printf("  Notes saved to .naqb/research/\n")
					for _, n := range result.Notes {
						if n.Filename != "" {
							fmt.Printf("  • %s\n", n.Filename)
						}
					}
				}
				return nil
			}

			if all {
				for _, ch := range cfg.Chapters {
					fmt.Printf("\nResearch — Chapter %d: %s\n", ch.Number, ch.Title)
					result, err := research.Run(ctx, client, bookDir, cfg, ch.Number, rules, os.Stdout)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  Chapter %d failed: %v\n", ch.Number, err)
						continue
					}
					fmt.Printf("  Done: %d queries, %d results, %d notes\n",
						result.Queries, result.Results, len(result.Notes))
				}
				return nil
			}

			if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N or --all")
			}

			title := chapterTitle(cfg, chapterNum)
			fmt.Printf("Research — Chapter %d: %s\n", chapterNum, title)

			result, err := research.Run(ctx, client, bookDir, cfg, chapterNum, rules, os.Stdout)
			if err != nil {
				return err
			}

			fmt.Printf("\n✓ Research complete: %d queries, %d results, %d notes\n",
				result.Queries, result.Results, len(result.Notes))
			if len(result.Notes) > 0 {
				fmt.Printf("  Notes saved to .naqb/research/\n")
				for _, n := range result.Notes {
					if n.Filename != "" {
						fmt.Printf("  • %s\n", n.Filename)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Run research for all chapters")
	cmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "Named LLM provider (overrides book.yaml)")
	cmd.Flags().BoolVar(&deep, "deep", false, "Use Gemini Search grounding for deep research (requires GEMINI_API_KEY)")
	cmd.Flags().StringVar(&fromYouTube, "from-youtube", "", "YouTube URL or video ID to import transcript from")
	return cmd
}
