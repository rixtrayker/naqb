package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amr/naqb/internal/config"
)

func TestBuildContext_Basic(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.BookConfig{
		Title:       "Test Book",
		Author:      "Test Author",
		Language:    "ar",
		Domain:      "Arabic literature",
		Synopsis:    "A test synopsis",
		TargetWords: 2000,
		Chapters: []config.Chapter{
			{Number: 1, Title: "Introduction", File: "ch-01.md", Summary: "Intro summary"},
			{Number: 2, Title: "Main Body", File: "ch-02.md", Summary: "Main content"},
		},
	}

	result, err := BuildContext(dir, cfg, 2)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	// Should contain the book title
	if !strings.Contains(result, "Test Book") {
		t.Error("context should contain book title")
	}
	// Should contain the target chapter
	if !strings.Contains(result, "Chapter 2") {
		t.Error("context should contain target chapter number")
	}
	if !strings.Contains(result, "Main Body") {
		t.Error("context should contain target chapter title")
	}
	// Should contain the language
	if !strings.Contains(result, "Modern Standard Arabic") {
		t.Error("context should contain language description for 'ar'")
	}
	// Should contain the previous chapter summary
	if !strings.Contains(result, "Intro summary") {
		t.Error("context should contain previous chapter summary")
	}
}

func TestBuildContext_ChapterNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.BookConfig{
		Chapters: []config.Chapter{
			{Number: 1, Title: "Intro"},
		},
	}
	_, err := BuildContext(dir, cfg, 99)
	if err == nil {
		t.Error("expected error for nonexistent chapter 99")
	}
}

func TestBuildContext_WithOutlineFile(t *testing.T) {
	dir := t.TempDir()

	// Create an outline.md with chapter sections
	outline := `## Chapter 1: Introduction

Background and motivation.

## Chapter 2: Deep Dive

The meat of the content with detailed analysis.

## Chapter 3: Conclusion

Wrapping up.
`
	if err := os.WriteFile(filepath.Join(dir, "outline.md"), []byte(outline), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:    "Outlined Book",
		Language: "en",
		Domain:   "Testing",
		Chapters: []config.Chapter{
			{Number: 1, Title: "Introduction"},
			{Number: 2, Title: "Deep Dive"},
		},
	}

	result, err := BuildContext(dir, cfg, 2)
	if err != nil {
		t.Fatalf("BuildContext with outline: %v", err)
	}

	if !strings.Contains(result, "The meat of the content") {
		t.Error("context should include content from outline.md section for chapter 2")
	}
	// Should NOT include chapter 3 content (stops at next ## heading)
	if strings.Contains(result, "Wrapping up") {
		t.Error("context should not include chapter 3 content")
	}
}

func TestBuildContext_WithResearchNotes(t *testing.T) {
	dir := t.TempDir()
	researchDir := filepath.Join(dir, "research")
	if err := os.MkdirAll(researchDir, 0o750); err != nil {
		t.Fatal(err)
	}

	noteContent := "This is an important research finding about the topic."
	if err := os.WriteFile(filepath.Join(researchDir, "notes.md"), []byte(noteContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.BookConfig{
		Title:    "Research Book",
		Language: "en",
		Domain:   "History",
		Chapters: []config.Chapter{
			{Number: 1, Title: "History Chapter"},
		},
	}

	result, err := BuildContext(dir, cfg, 1)
	if err != nil {
		t.Fatalf("BuildContext with research notes: %v", err)
	}

	if !strings.Contains(result, noteContent) {
		t.Error("context should include research notes")
	}
}

func TestWriteContextFile(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.BookConfig{
		Title:    "File Test Book",
		Language: "en",
		Domain:   "Science",
		Chapters: []config.Chapter{
			{Number: 3, Title: "Science Chapter"},
		},
	}

	outPath, err := WriteContextFile(dir, cfg, 3)
	if err != nil {
		t.Fatalf("WriteContextFile: %v", err)
	}

	// Check the file was created at the expected path
	expectedPath := filepath.Join(dir, "contexts", "ch-03-context.md")
	if outPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, outPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading written context file: %v", err)
	}
	if !strings.Contains(string(data), "Science Chapter") {
		t.Error("written context file should contain chapter title")
	}
}

func TestLanguageDescs_English(t *testing.T) {
	langDesc, termsDesc, hasCode := languageDescs("en", "general fiction")
	if langDesc != "English" {
		t.Errorf("unexpected langDesc: %s", langDesc)
	}
	if termsDesc != "Arabic" {
		t.Errorf("unexpected termsDesc: %s", termsDesc)
	}
	if hasCode {
		t.Error("fiction should not have code")
	}
}

func TestLanguageDescs_TechDomains(t *testing.T) {
	techDomains := []string{
		"computer science",
		"programming and software",
		"Software Engineering",
		"tech startup",
	}
	for _, domain := range techDomains {
		_, _, hasCode := languageDescs("en", domain)
		if !hasCode {
			t.Errorf("domain %q should be detected as code-heavy", domain)
		}
	}
}

func TestExtractOutlineSection_NoFile(t *testing.T) {
	// With no outline.md, should fall back to chapter title
	dir := t.TempDir()
	result := extractOutlineSection(dir, 1, "My Chapter")
	if !strings.Contains(result, "My Chapter") {
		t.Error("fallback should contain chapter title")
	}
}

func TestExtractOutlineSection_Found(t *testing.T) {
	dir := t.TempDir()
	outline := "## Chapter 2: Deep Analysis\n\nSection content here.\n\n## Chapter 3: Other\n\nOther content."
	_ = os.WriteFile(filepath.Join(dir, "outline.md"), []byte(outline), 0o644)

	result := extractOutlineSection(dir, 2, "Deep Analysis")
	if !strings.Contains(result, "Section content here") {
		t.Error("should extract chapter 2 section")
	}
	if strings.Contains(result, "Other content") {
		t.Error("should not include chapter 3 content")
	}
}
