package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChapterFilename(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "ch-01.md"},
		{9, "ch-09.md"},
		{10, "ch-10.md"},
		{99, "ch-99.md"},
	}
	for _, tc := range cases {
		if got := ChapterFilename(tc.n); got != tc.want {
			t.Errorf("ChapterFilename(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestContextFilename(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "ch-01-context.md"},
		{5, "ch-05-context.md"},
		{12, "ch-12-context.md"},
	}
	for _, tc := range cases {
		if got := ContextFilename(tc.n); got != tc.want {
			t.Errorf("ContextFilename(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestSaveAndLoadBook(t *testing.T) {
	dir := t.TempDir()

	original := &BookConfig{
		Title:       "Test Book",
		Author:      "Test Author",
		Language:    "ar",
		Domain:      "Arabic literature",
		Synopsis:    "A test book synopsis",
		TargetWords: 3000,
		CreatedAt:   time.Now().Truncate(time.Second),
		Version:     "1.0",
		Chapters: []Chapter{
			{Number: 1, Title: "Introduction", File: "ch-01.md", Summary: "Intro summary"},
			{Number: 2, Title: "Main Body", File: "ch-02.md"},
		},
	}

	if err := SaveBook(dir, original); err != nil {
		t.Fatalf("SaveBook: %v", err)
	}

	loaded, err := LoadBook(dir)
	if err != nil {
		t.Fatalf("LoadBook: %v", err)
	}

	if loaded.Title != original.Title {
		t.Errorf("Title: got %q, want %q", loaded.Title, original.Title)
	}
	if loaded.Author != original.Author {
		t.Errorf("Author: got %q, want %q", loaded.Author, original.Author)
	}
	if loaded.Language != original.Language {
		t.Errorf("Language: got %q, want %q", loaded.Language, original.Language)
	}
	if loaded.TargetWords != original.TargetWords {
		t.Errorf("TargetWords: got %d, want %d", loaded.TargetWords, original.TargetWords)
	}
	if len(loaded.Chapters) != len(original.Chapters) {
		t.Fatalf("Chapters len: got %d, want %d", len(loaded.Chapters), len(original.Chapters))
	}
	if loaded.Chapters[0].Title != original.Chapters[0].Title {
		t.Errorf("Chapter[0].Title: got %q, want %q", loaded.Chapters[0].Title, original.Chapters[0].Title)
	}
	if loaded.Chapters[0].Summary != original.Chapters[0].Summary {
		t.Errorf("Chapter[0].Summary: got %q, want %q", loaded.Chapters[0].Summary, original.Chapters[0].Summary)
	}
}

func TestInitBookDir(t *testing.T) {
	dir := t.TempDir()

	cfg := &BookConfig{
		Title:    "My Test Book",
		Author:   "Test",
		Language: "en",
		Domain:   "Testing",
		Chapters: []Chapter{
			{Number: 1, Title: "Intro", File: "ch-01.md"},
		},
	}

	if err := InitBookDir(dir, cfg); err != nil {
		t.Fatalf("InitBookDir: %v", err)
	}

	// Verify expected directories exist
	expectedDirs := []string{
		"chapters",
		"contexts",
		"research",
		filepath.Join("assets", "themes"),
		"output",
		filepath.Join("config", "prompts"),
	}
	for _, d := range expectedDirs {
		path := filepath.Join(dir, d)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected directory %q to exist: %v", d, err)
		}
	}

	// Verify book.yaml was written
	if _, err := os.Stat(filepath.Join(dir, "book.yaml")); err != nil {
		t.Error("book.yaml should exist")
	}

	// Verify rules.yaml was written
	if _, err := os.Stat(filepath.Join(dir, "config", "rules.yaml")); err != nil {
		t.Error("config/rules.yaml should exist")
	}

	// Verify prompts exist
	for _, prompt := range []string{"init.md", "write.md", "qa.md"} {
		path := filepath.Join(dir, "config", "prompts", prompt)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("prompt file %q should exist: %v", prompt, err)
		}
	}

	// Verify .gitignore was written
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Error(".gitignore should exist")
	}

	// Verify we can load the saved book
	loaded, err := LoadBook(dir)
	if err != nil {
		t.Fatalf("LoadBook after InitBookDir: %v", err)
	}
	if loaded.Title != cfg.Title {
		t.Errorf("Title after init: got %q, want %q", loaded.Title, cfg.Title)
	}
}

func TestLoadBook_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadBook(dir)
	if err == nil {
		t.Error("expected error loading book.yaml from empty dir")
	}
}

func TestSaveBook_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested", "dir")

	cfg := &BookConfig{Title: "Nested", Language: "en"}
	if err := SaveBook(dir, cfg); err != nil {
		t.Fatalf("SaveBook with nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "book.yaml")); err != nil {
		t.Error("book.yaml should exist in newly created dir")
	}
}
