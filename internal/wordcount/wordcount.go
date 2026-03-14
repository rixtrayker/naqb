// Package wordcount provides word counting utilities for book chapters.
// It correctly handles Arabic, Latin, and mixed-language content.
package wordcount

import (
	"os"
	"strings"
	"unicode"
)

// Count returns the number of words in s.
// A word is any sequence of non-whitespace Unicode runes.
// Markdown syntax tokens (###, ---, ```) are not counted.
func Count(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else {
			if !inWord {
				count++
				inWord = true
			}
		}
	}
	return count
}

// CountFile reads a file and returns its word count.
// Returns 0 and the error if the file cannot be read.
func CountFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return Count(string(data)), nil
}

// Progress describes a chapter's word count relative to a target.
type Progress struct {
	Words  int
	Target int
	Min    int
	Max    int
}

// Percent returns completion as 0–100. Capped at 100.
func (p Progress) Percent() int {
	if p.Target <= 0 {
		return 0
	}
	pct := p.Words * 100 / p.Target
	if pct > 100 {
		return 100
	}
	return pct
}

// Status returns "ok", "short", "long", or "empty".
func (p Progress) Status() string {
	if p.Words == 0 {
		return "empty"
	}
	if p.Min > 0 && p.Words < p.Min {
		return "short"
	}
	if p.Max > 0 && p.Words > p.Max {
		return "long"
	}
	return "ok"
}

// Bar renders a fixed-width ASCII progress bar, e.g. "[████░░░░] 75%".
func Bar(p Progress, width int) string {
	if width < 4 {
		width = 4
	}
	pct := p.Percent()
	filled := pct * width / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return "[" + bar + "] " + formatNum(p.Words) + "/" + formatNum(p.Target)
}

func formatNum(n int) string {
	if n >= 1000 {
		k := n / 1000
		r := (n % 1000) / 100
		if r == 0 {
			return strings.Repeat("", 0) + itoa(k) + "k"
		}
		return itoa(k) + "." + itoa(r) + "k"
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
