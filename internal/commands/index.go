package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/search"
)

// IndexCmd returns the `nqb index` command.
func IndexCmd() *cobra.Command {
	var reindex bool

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index chapters and research notes into the local vector store",
		Long: `Indexes all chapter files and research notes into the local vector store
(.naqb/vectors/). Enables semantic search when building chapter contexts.

Requires OPENAI_API_KEY or MISTRAL_API_KEY for semantic (embedding-based)
indexing. Without a key, documents are stored for keyword-only retrieval.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			store, err := search.Open(bookDir)
			if err != nil {
				return fmt.Errorf("opening vector store: %w", err)
			}
			defer store.Close()

			if store.HasEmbedder() {
				fmt.Println("Semantic indexing enabled (embeddings active).")
			} else {
				fmt.Println("No embedding key found — storing docs for keyword retrieval only.")
				fmt.Println("Set OPENAI_API_KEY or MISTRAL_API_KEY for semantic search.")
			}

			ctx := context.Background()
			chapIndexed, chapSkipped := 0, 0
			noteIndexed := 0

			// Index chapter files
			fmt.Println("\nIndexing chapters…")
			for _, ch := range cfg.Chapters {
				fname := config.ChapterFilename(ch.Number)
				path := filepath.Join(bookDir, "chapters", fname)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					chapSkipped++
					continue
				}
				if err := store.IndexChapter(ctx, bookDir, ch.Number, path); err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ ch-%02d: %v\n", ch.Number, err)
					continue
				}
				fmt.Printf("  ✓ %s\n", fname)
				chapIndexed++
			}

			// Index research notes from both dirs
			fmt.Println("\nIndexing research notes…")
			for _, dir := range []string{
				filepath.Join(bookDir, "research"),
				filepath.Join(bookDir, ".naqb", "research"),
			} {
				n, err := indexDir(ctx, store, dir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", dir, err)
					continue
				}
				noteIndexed += n
			}

			fmt.Printf("\n✓ Done: %d chapters indexed (%d skipped), %d notes indexed.\n",
				chapIndexed, chapSkipped, noteIndexed)
			_ = reindex
			return nil
		},
	}

	cmd.Flags().BoolVar(&reindex, "reindex", false, "Force re-indexing of already indexed documents")
	return cmd
}

// indexDir indexes all .md/.txt files in dir, returning the count indexed.
func indexDir(ctx context.Context, store *search.Store, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var count int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		path := filepath.Join(dir, name)
		if err := store.IndexFile(ctx, "research", name, path); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s: %v\n", name, err)
			continue
		}
		fmt.Printf("  ✓ %s\n", name)
		count++
	}
	return count, nil
}
