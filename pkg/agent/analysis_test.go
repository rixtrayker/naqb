package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/pkg/config"
)

func TestAnalyze_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.BookConfig{
		Title:       "Test Book",
		Author:      "Test Author",
		Language:    "en",
		Domain:      "testing",
		TargetWords: 2000,
		Chapters: []config.Chapter{
			{Number: 1, Title: "Introduction"},
			{Number: 2, Title: "Core Concepts"},
		},
	}

	a := Analyze(dir, cfg, nil)

	if a.Title != "Test Book" {
		t.Errorf("Title = %q, want %q", a.Title, "Test Book")
	}
	if a.TotalChapters != 2 {
		t.Errorf("TotalChapters = %d, want 2", a.TotalChapters)
	}
	if a.TotalWords != 0 {
		t.Errorf("TotalWords = %d, want 0", a.TotalWords)
	}
	if a.TotalTarget != 4000 {
		t.Errorf("TotalTarget = %d, want 4000", a.TotalTarget)
	}
	if len(a.Chapters) != 2 {
		t.Fatalf("len(Chapters) = %d, want 2", len(a.Chapters))
	}
	if a.Chapters[0].Status != "pending" {
		t.Errorf("Chapter 1 Status = %q, want %q", a.Chapters[0].Status, "pending")
	}
}

func TestAnalyze_WithChapterFile(t *testing.T) {
	dir := t.TempDir()
	chapDir := filepath.Join(dir, "chapters")
	if err := os.MkdirAll(chapDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Write a chapter with some content
	content := strings.Repeat("word ", 500) // 500 words
	if err := os.WriteFile(filepath.Join(chapDir, "ch-01.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:       "Test",
		TargetWords: 1000,
		Chapters: []config.Chapter{
			{Number: 1, Title: "Chapter One"},
		},
	}

	a := Analyze(dir, cfg, nil)

	if a.TotalWords != 500 {
		t.Errorf("TotalWords = %d, want 500", a.TotalWords)
	}
	if a.Chapters[0].Words != 500 {
		t.Errorf("Chapter 1 Words = %d, want 500", a.Chapters[0].Words)
	}
	if a.Chapters[0].Percent != 50 {
		t.Errorf("Chapter 1 Percent = %d, want 50", a.Chapters[0].Percent)
	}
	if a.Chapters[0].Status != "written" {
		t.Errorf("Chapter 1 Status = %q, want %q", a.Chapters[0].Status, "written")
	}
}

func TestAnalyze_WithContextAndOutline(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"chapters", "contexts"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	// Create context file
	if err := os.WriteFile(filepath.Join(dir, "contexts", "ch-01-context.md"), []byte("context"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create outline
	if err := os.WriteFile(filepath.Join(dir, "outline.md"), []byte("# Outline"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:       "Test",
		TargetWords: 1000,
		Chapters:    []config.Chapter{{Number: 1, Title: "Ch 1"}},
	}

	a := Analyze(dir, cfg, nil)

	if !a.OutlineExists {
		t.Error("OutlineExists = false, want true")
	}
	if !a.Chapters[0].HasContext {
		t.Error("Chapter 1 HasContext = false, want true")
	}
}

func TestAnalyze_ResearchNotes(t *testing.T) {
	dir := t.TempDir()
	resDir := filepath.Join(dir, "research")
	if err := os.MkdirAll(resDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"note1.md", "note2.md", "image.png"} {
		if err := os.WriteFile(filepath.Join(resDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.BookConfig{Title: "Test", Chapters: []config.Chapter{}}

	a := Analyze(dir, cfg, nil)

	if a.ResearchNoteCount != 2 {
		t.Errorf("ResearchNoteCount = %d, want 2", a.ResearchNoteCount)
	}
}

func TestAnalyze_NilConfig(t *testing.T) {
	a := Analyze("/tmp/nonexistent", nil, nil)
	if a.Title != "" {
		t.Errorf("Title = %q, want empty", a.Title)
	}
}

func TestSystemPromptSection_Renders(t *testing.T) {
	a := &ProjectAnalysis{
		Title:         "My Book",
		Author:        "Author",
		Language:      "ar",
		Domain:        "Arabic culture",
		TotalChapters: 1,
		TotalWords:    1500,
		TotalTarget:   3000,
		OutlineExists: true,
		Chapters: []ChapterAnalysis{
			{Number: 1, Title: "Intro", Status: "written", Words: 1500, Target: 3000, Percent: 50, HasContext: true},
		},
		ResearchNoteCount: 5,
	}

	section := a.SystemPromptSection()

	if !strings.Contains(section, "My Book") {
		t.Error("missing book title")
	}
	if !strings.Contains(section, "1500 / 3000") {
		t.Error("missing total word count")
	}
	if !strings.Contains(section, "Outline: exists") {
		t.Error("missing outline indicator")
	}
	if !strings.Contains(section, "Research notes: 5") {
		t.Error("missing research note count")
	}
	if !strings.Contains(section, "| 1 | Intro |") {
		t.Error("missing chapter row")
	}
}

func TestSystemPromptSection_NilAnalysis(t *testing.T) {
	var a *ProjectAnalysis
	if s := a.SystemPromptSection(); s != "" {
		t.Errorf("expected empty string for nil analysis, got %q", s)
	}
}
