package watcher

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatch_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	// Create the directories that Watch expects
	for _, sub := range []string{"chapters", "research"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, dir, func(path string) error { return nil }, &buf)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch did not return promptly after context cancellation")
	}
}

func TestIsMarkdown(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"chapter.md", true},
		{"notes.markdown", true},
		{"data.json", false},
		{"image.png", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isMarkdown(tc.path); got != tc.want {
			t.Errorf("isMarkdown(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
