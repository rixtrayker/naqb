// Package watcher uses fsnotify to watch for Markdown file changes and trigger rebuilds.
package watcher

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// RebuildFunc is called when a watched file changes.
type RebuildFunc func(path string) error

// Watch watches the book directory for markdown changes and calls rebuild.
// It blocks until the context is cancelled, the watcher errors, or an unrecoverable error occurs.
func Watch(ctx context.Context, bookDir string, rebuild RebuildFunc, out io.Writer) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer w.Close()

	// Watch chapters and research directories
	for _, subDir := range []string{"chapters", "research", "."} {
		dir := filepath.Join(bookDir, subDir)
		if err := w.Add(dir); err != nil {
			fmt.Fprintf(out, "warning: cannot watch %s: %v\n", dir, err)
		}
	}

	fmt.Fprintf(out, "Watching %s for changes (Ctrl+C to stop)...\n", bookDir)

	// Debounce: group rapid events
	var debounce <-chan time.Time
	pendingPath := ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isMarkdown(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			pendingPath = event.Name
			debounce = time.After(500 * time.Millisecond)

		case <-debounce:
			if pendingPath != "" {
				fmt.Fprintf(out, "\n[watch] Change detected: %s\n", pendingPath)
				if err := rebuild(pendingPath); err != nil {
					fmt.Fprintf(out, "[watch] Rebuild error: %v\n", err)
				}
				pendingPath = ""
			}

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(out, "[watch] Watcher error: %v\n", err)
		}
	}
}

func isMarkdown(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".markdown"
}
