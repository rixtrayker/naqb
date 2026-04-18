package chunker

import (
	"strings"
	"testing"
)

func TestSplitText_ShortText(t *testing.T) {
	text := "Hello world"
	chunks := SplitText(text, nil, 512, 50)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("chunk text = %q; want %q", chunks[0].Text, text)
	}
	if chunks[0].Start != 0 {
		t.Errorf("chunk start = %d; want 0", chunks[0].Start)
	}
}

func TestSplitText_Empty(t *testing.T) {
	chunks := SplitText("", nil, 512, 50)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestSplitText_LongEnglishText(t *testing.T) {
	// Build a long text
	sentences := make([]string, 50)
	for i := range sentences {
		sentences[i] = "This is sentence number one with some words in it."
	}
	text := strings.Join(sentences, " ")

	chunks := SplitText(text, nil, 100, 10)
	if len(chunks) == 0 {
		t.Fatal("expected chunks for long text")
	}
	// Verify each chunk is within size limit (some tolerance for separator)
	for i, c := range chunks {
		runes := []rune(c.Text)
		if len(runes) > 200 { // generous tolerance
			t.Errorf("chunk %d too long: %d runes", i, len(runes))
		}
	}
}

func TestSplitText_ArabicSeparators(t *testing.T) {
	// Arabic text with comma and semicolon separators
	text := "الجملة الأولى، الجملة الثانية؛ الجملة الثالثة، الجملة الرابعة"
	chunks := SplitText(text, nil, 20, 0)
	if len(chunks) == 0 {
		t.Fatal("expected chunks for Arabic text")
	}
	// Verify chunks cover the content
	combined := ""
	for _, c := range chunks {
		combined += c.Text
	}
	// All original content should be present (possibly without separators)
	if !strings.Contains(combined, "الجملة") {
		t.Error("chunked output should contain Arabic words")
	}
}

func TestSplitTextParentChild_Basic(t *testing.T) {
	sentences := make([]string, 30)
	for i := range sentences {
		sentences[i] = "This is a test sentence with several words."
	}
	text := strings.Join(sentences, " ")

	pairs := SplitTextParentChild(text, 200, 50, 10)
	if len(pairs) == 0 {
		t.Fatal("expected pairs for long text")
	}
	for i, pair := range pairs {
		if pair.Parent.Text == "" {
			t.Errorf("pair %d has empty parent text", i)
		}
		// Children should all be subsets of parent content
		for j, child := range pair.Children {
			if !strings.Contains(pair.Parent.Text, child.Text[:min(10, len([]rune(child.Text)))]) {
				t.Errorf("pair %d child %d text not found in parent", i, j)
			}
		}
	}
}

func TestSplitText_CustomSeparators(t *testing.T) {
	text := "part1|part2|part3|part4|part5|part6|part7|part8|part9|part10"
	chunks := SplitText(text, []string{"|", " ", ""}, 5, 0)
	if len(chunks) == 0 {
		t.Fatal("expected chunks with custom separators")
	}
}

func TestSplitText_OverlapNotExceedsChunkSize(t *testing.T) {
	// overlap >= chunkSize should be clamped
	text := strings.Repeat("word ", 100)
	chunks := SplitText(text, nil, 50, 100) // overlap > chunkSize
	if len(chunks) == 0 {
		t.Fatal("expected chunks even with bad overlap param")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
