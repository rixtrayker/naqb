package agents

import (
	"testing"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/wordcount"
)

// ── checkHeadingHierarchy ────────────────────────────────────────────────────

func TestCheckHeadingHierarchy_OK(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"flat h2s", "## Intro\n## Body\n## Conclusion"},
		{"h1 then h2 then h3", "# Book\n## Part\n### Section"},
		{"mixed with prose", "Some prose\n## Heading\nMore prose\n### Sub"},
		{"no headings", "Just plain text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if msg := checkHeadingHierarchy(splitLines(tc.input)); msg != "" {
				t.Errorf("expected no issue, got: %s", msg)
			}
		})
	}
}

func TestCheckHeadingHierarchy_Skip(t *testing.T) {
	// h1 → h3 is a skip
	input := "# Title\n### Too deep"
	if msg := checkHeadingHierarchy(splitLines(input)); msg == "" {
		t.Error("expected hierarchy skip to be detected")
	}
}

func TestCheckHeadingHierarchy_H2ToH4(t *testing.T) {
	input := "## Section\n#### Skip two levels"
	if msg := checkHeadingHierarchy(splitLines(input)); msg == "" {
		t.Error("expected h2→h4 skip to be detected")
	}
}

// ── checkCodeBlockLanguages ──────────────────────────────────────────────────

func TestCheckCodeBlockLanguages_OK(t *testing.T) {
	cases := []string{
		"```go\nfmt.Println()\n```",
		"```python\nprint()\n```",
		"no code blocks here",
		"```bash\necho hello\n```\n```rust\nfn main(){}\n```",
	}
	for _, c := range cases {
		if msg := checkCodeBlockLanguages(c); msg != "" {
			t.Errorf("expected no issue for %q, got: %s", c, msg)
		}
	}
}

func TestCheckCodeBlockLanguages_Missing(t *testing.T) {
	if msg := checkCodeBlockLanguages("```\nno language\n```"); msg == "" {
		t.Error("expected missing language to be detected")
	}
}

func TestCheckCodeBlockLanguages_MultipleUnlabeled(t *testing.T) {
	content := "```\nfoo\n```\n```\nbar\n```"
	msg := checkCodeBlockLanguages(content)
	if msg == "" {
		t.Error("expected 2 unlabeled code blocks to be detected")
	}
}

// ── checkCalloutSyntax ───────────────────────────────────────────────────────

func TestCheckCalloutSyntax_OK(t *testing.T) {
	cases := []string{
		"[!] This is a note",
		"[?] Deep dive here",
		"[X] Warning",
		"Regular text with no callouts",
	}
	for _, c := range cases {
		if msg := checkCalloutSyntax(c); msg != "" {
			t.Errorf("expected no issue for %q, got: %s", c, msg)
		}
	}
}

func TestCheckCalloutSyntax_Bad(t *testing.T) {
	bad := []string{"[Note]", "[Warning]", "[Info]", "[TODO]", "[FIXME]"}
	for _, b := range bad {
		if msg := checkCalloutSyntax("Some text " + b + " more"); msg == "" {
			t.Errorf("expected %q to be flagged as non-standard callout", b)
		}
	}
}

// ── wordcount.Count ───────────────────────────────────────────────────────────

func TestCountWords(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"hello world", 2},
		{"  spaces   everywhere  ", 2},
		{"", 0},
		{"one", 1},
		{"مرحبا بالعالم", 2}, // Arabic words
	}
	for _, tc := range cases {
		if got := wordcount.Count(tc.input); got != tc.want {
			t.Errorf("wordcount.Count(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// ── runDeterministicChecks ───────────────────────────────────────────────────

func TestRunDeterministicChecks_Pass(t *testing.T) {
	// Build a chapter that passes all checks
	chapter := `## Introduction

This is a well-formed chapter with enough words to pass the word count check.
` + makeWords(600) + `

## Main Body

Here is some content.

` + "```go\nfmt.Println(\"hello\")\n```" + `

## Conclusion

[!] Important note here.

` + makeWords(100)

	issues := runDeterministicChecks(chapter, &config.BookConfig{TargetWords: 1000}, nil)
	if len(issues) > 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestRunDeterministicChecks_TooShort(t *testing.T) {
	issues := runDeterministicChecks("## Chapter\n\nToo short.", &config.BookConfig{TargetWords: 1000}, nil)
	found := false
	for _, i := range issues {
		if contains(i, "Word count too low") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected word count too low issue, got: %v", issues)
	}
}

func TestRunDeterministicChecks_NilConfig(t *testing.T) {
	// nil cfg and nil rules should use defaults (500 min); a tiny chapter still fails
	issues := runDeterministicChecks("## Hi\n\nShort.", nil, nil)
	found := false
	for _, i := range issues {
		if contains(i, "Word count too low") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected word count too low with nil config, got: %v", issues)
	}
}

// ── buildFinishedSummaries ───────────────────────────────────────────────────

func TestBuildFinishedSummaries(t *testing.T) {
	cfg := &config.BookConfig{
		Chapters: []config.Chapter{
			{Number: 1, Title: "Intro", Summary: "Overview"},
			{Number: 2, Title: "Middle", Summary: "Details"},
			{Number: 3, Title: "End", Summary: "Conclusion"},
		},
	}

	// Current chapter = 3, should see ch1 and ch2 summaries
	result := buildFinishedSummaries(cfg, 3)
	if !contains(result, "Chapter 1") {
		t.Error("expected Chapter 1 in summaries")
	}
	if !contains(result, "Chapter 2") {
		t.Error("expected Chapter 2 in summaries")
	}
	if contains(result, "Chapter 3") {
		t.Error("Chapter 3 should not appear (it's the current chapter)")
	}
}

func TestBuildFinishedSummaries_EmptySummary(t *testing.T) {
	cfg := &config.BookConfig{
		Chapters: []config.Chapter{
			{Number: 1, Title: "Intro", Summary: ""},
			{Number: 2, Title: "Middle", Summary: "Has summary"},
		},
	}
	result := buildFinishedSummaries(cfg, 3)
	// Chapter 1 has no summary, should not appear
	if contains(result, "Chapter 1") {
		t.Error("Chapter 1 with empty summary should not appear")
	}
	if !contains(result, "Chapter 2") {
		t.Error("Chapter 2 with summary should appear")
	}
}

// ── languageDescs ────────────────────────────────────────────────────────────

func TestLanguageDescs(t *testing.T) {
	langDesc, termsDesc, hasCode := languageDescs("ar", "Arabic history")
	if langDesc != "Modern Standard Arabic (MSA)" {
		t.Errorf("unexpected langDesc: %s", langDesc)
	}
	if termsDesc != "English" {
		t.Errorf("unexpected termsDesc: %s", termsDesc)
	}
	if hasCode {
		t.Error("Arabic history should not have code")
	}

	_, _, hasCode = languageDescs("en", "computer science and programming")
	if !hasCode {
		t.Error("CS domain should have code")
	}

	_, _, hasCode = languageDescs("ar", "software engineering")
	if !hasCode {
		t.Error("software domain should have code regardless of language")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func makeWords(n int) string {
	word := "word "
	result := make([]byte, 0, n*5)
	for i := 0; i < n; i++ {
		result = append(result, []byte(word)...)
	}
	return string(result)
}
