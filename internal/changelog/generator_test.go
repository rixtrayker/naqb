package changelog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCategorize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"init: My Book", "Setup"},
		{"chapter(01): Chapter 1 first draft", "Writing"},
		{"draft(02): Chapter 2 draft", "Writing"},
		{"context(03): Context assembled", "Context"},
		{"qa(01): QA complete", "Quality"},
		{"research: added notes", "Research"},
		{"export(pdf): PDF generated", "Publishing"},
		{"fix: typo", "Fixes"},
		{"refactor: extract helper", "Refactoring"},
		{"random commit", "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := categorize(tt.input)
			if got != tt.want {
				t.Fatalf("categorize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHumanize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"init: My Book", "My Book"},
		{"chapter(01): Chapter 1 first draft", "Chapter 1 first draft"},
		{"qa(01): QA complete", "QA complete"},
		{"lowercase start", "Lowercase start"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := humanize(tt.input)
			if got != tt.want {
				t.Fatalf("humanize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Skip("git not available")
	}
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "Test").Run(); err != nil {
		t.Fatal(err)
	}

	// Create a file and commit
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "init: test book").Run(); err != nil {
		t.Fatal(err)
	}

	report, err := Generate(dir, 10)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(report.Entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	if report.Entries[0].Category != "Setup" {
		t.Fatalf("expected category Setup, got %q", report.Entries[0].Category)
	}
	if !strings.Contains(report.Entries[0].Message, "Test book") {
		t.Fatalf("expected message to contain 'Test book', got %q", report.Entries[0].Message)
	}
}

func TestFormatMarkdown(t *testing.T) {
	r := &SessionReport{
		Date: "2026-04-16",
		Entries: []Entry{
			{Category: "Writing", Message: "Chapter 1 drafted"},
			{Category: "Research", Message: "Added notes"},
		},
	}

	md := FormatMarkdown(r)
	if !strings.Contains(md, "# Writing Session — 2026-04-16") {
		t.Fatal("missing session header")
	}
	if !strings.Contains(md, "## Writing") {
		t.Fatal("missing Writing section")
	}
	if !strings.Contains(md, "- Chapter 1 drafted") {
		t.Fatal("missing drafted entry")
	}
}
