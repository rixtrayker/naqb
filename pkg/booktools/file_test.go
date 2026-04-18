package booktools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/amr/naqb/pkg/config"
)

func TestReadFileTool_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	content := "# Chapter 1\nHello world."
	path := filepath.Join(dir, "chapters", "ch-01.md")
	_ = os.MkdirAll(filepath.Dir(path), 0o750)
	_ = os.WriteFile(path, []byte(content), 0o644)

	tool := NewReadFileTool(dir)
	result, err := tool.Invoke(context.Background(), `{"path":"chapters/ch-01.md"}`)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result != content {
		t.Errorf("content mismatch: got %q, want %q", result, content)
	}
}

func TestReadFileTool_PathTraversalBlocked(t *testing.T) {
	tool := NewReadFileTool(t.TempDir())
	for _, bad := range []string{"../etc/passwd", "/etc/passwd"} {
		result, err := tool.Invoke(context.Background(), `{"path":"`+bad+`"}`)
		if err != nil {
			t.Fatalf("unexpected hard error for %q: %v", bad, err)
		}
		if result == "" {
			t.Errorf("expected path traversal %q to be blocked", bad)
		}
	}
}

func TestWriteFileTool_WritesFile(t *testing.T) {
	dir := t.TempDir()
	tool := NewWriteFileTool(dir)
	result, err := tool.Invoke(context.Background(), `{"path":"chapters/ch-02.md","content":"## Chapter 2\nContent here."}`)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	data, err := os.ReadFile(filepath.Join(dir, "chapters", "ch-02.md"))
	if err != nil {
		t.Fatalf("ReadFile after write: %v", err)
	}
	if string(data) != "## Chapter 2\nContent here." {
		t.Errorf("written content mismatch: %q", string(data))
	}
}

func TestWriteFileTool_PathTraversalBlocked(t *testing.T) {
	tool := NewWriteFileTool(t.TempDir())
	result, _ := tool.Invoke(context.Background(), `{"path":"../evil.sh","content":"rm -rf /"}`)
	if result == "" {
		t.Error("expected path traversal to be blocked")
	}
}

func TestListChaptersTool_WithDrafts(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "chapters"), 0o750)
	_ = os.WriteFile(filepath.Join(dir, "chapters", "ch-01.md"), []byte("word word word "), 0o644)

	cfg := &config.BookConfig{
		Title:       "Test Book",
		TargetWords: 3000,
		Chapters: []config.Chapter{
			{Number: 1, Title: "Introduction"},
			{Number: 2, Title: "Main Body"},
		},
	}
	tool := NewListChaptersTool(dir, cfg)
	result, err := tool.Invoke(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}
