package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
)

func TestBuildChapterTask_WithContextFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "contexts"), 0o750)
	contextContent := "# Context for Chapter 1\nWrite about X."
	_ = os.WriteFile(
		filepath.Join(dir, "contexts", config.ContextFilename(1)),
		[]byte(contextContent),
		0o644,
	)

	cfg := &config.BookConfig{
		Title:       "My Book",
		TargetWords: 3000,
		Chapters:    []config.Chapter{{Number: 1, Title: "Introduction"}},
	}
	task := BuildChapterTask(dir, cfg, 1, nil)

	if !strings.Contains(task, "Chapter 1") {
		t.Error("expected chapter number in task")
	}
	if !strings.Contains(task, contextContent) {
		t.Error("expected context file content embedded in task")
	}
	if !strings.Contains(task, "ch-01.md") {
		t.Error("expected output file name in task")
	}
}

func TestBuildChapterTask_WithoutContextFile(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.BookConfig{
		Title:       "My Book",
		TargetWords: 2500,
		Chapters: []config.Chapter{
			{Number: 2, Title: "Deep Dive", Summary: "Covers advanced topics."},
		},
	}
	task := BuildChapterTask(dir, cfg, 2, nil)

	if !strings.Contains(task, "Chapter 2") {
		t.Error("expected chapter number")
	}
	if !strings.Contains(task, "Deep Dive") {
		t.Error("expected chapter title")
	}
	if !strings.Contains(task, "ch-02.md") {
		t.Error("expected output filename")
	}
	// Should mention target words
	if !strings.Contains(task, "2500") {
		t.Error("expected target word count in task")
	}
}

func TestBuildChapterTask_UnknownChapter(t *testing.T) {
	cfg := &config.BookConfig{
		TargetWords: 3000,
		Chapters:    []config.Chapter{{Number: 1, Title: "Only Chapter"}},
	}
	// Chapter 99 not in config — should still produce a valid (non-empty) task
	task := BuildChapterTask(t.TempDir(), cfg, 99, nil)
	if task == "" {
		t.Error("BuildChapterTask should return non-empty task even for unknown chapter")
	}
}

func TestBuildChapterTask_NilConfig(t *testing.T) {
	// nil config should not panic and should return a valid task string
	task := BuildChapterTask(t.TempDir(), nil, 1, nil)
	if task == "" {
		t.Fatal("expected non-empty task for nil config")
	}
	if !strings.Contains(task, "Chapter 1") {
		t.Error("expected 'Chapter 1' in output")
	}
}

func TestBuildSessionSummary(t *testing.T) {
	cfg := &config.BookConfig{
		Title: "My Book",
		Chapters: []config.Chapter{
			{Number: 3, Title: "Advanced Topics"},
		},
	}
	s := BuildSessionSummary("", cfg, 3)
	if !strings.Contains(s, "3") || !strings.Contains(s, "Advanced Topics") {
		t.Errorf("unexpected summary: %q", s)
	}

	// chapterNum=0 → book-level
	s0 := BuildSessionSummary("", cfg, 0)
	if !strings.Contains(s0, "My Book") {
		t.Errorf("expected book title in chapter-0 summary: %q", s0)
	}
}

func TestChapterWordCount_NoFile(t *testing.T) {
	n := ChapterWordCount(t.TempDir(), 1)
	if n != 0 {
		t.Errorf("expected 0 for missing chapter, got %d", n)
	}
}

func TestChapterWordCount_WithFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "chapters"), 0o750)
	_ = os.WriteFile(
		filepath.Join(dir, "chapters", config.ChapterFilename(1)),
		[]byte("one two three four five"),
		0o644,
	)
	n := ChapterWordCount(dir, 1)
	if n != 5 {
		t.Errorf("expected 5 words, got %d", n)
	}
}
