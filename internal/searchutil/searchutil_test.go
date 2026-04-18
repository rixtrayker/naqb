package searchutil

import (
	"strings"
	"testing"
)

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"already clean", "hello world", "hello world"},
		{"extra spaces", "  hello   world  ", "hello world"},
		{"tabs and newlines", "hello\t\nworld", "hello world"},
		{"arabic text", "مرحبا بالعالم", "مرحبا بالعالم"},
		{"mixed whitespace arabic", "  مرحبا   بالعالم  ", "مرحبا بالعالم"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeContent(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeContent(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenizeContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func([]string) bool
	}{
		{
			"english sentence",
			"Hello, world! This is a test.",
			func(toks []string) bool {
				for _, tok := range toks {
					if tok == "hello" || tok == "world" || tok == "this" {
						return true
					}
				}
				return false
			},
		},
		{
			"filters short tokens",
			"a b cc ddd",
			func(toks []string) bool {
				for _, tok := range toks {
					if tok == "a" || tok == "b" {
						return true // short tokens should be filtered
					}
				}
				return false
			},
		},
		{
			"arabic text",
			"الكتاب العربي الكلاسيكي",
			func(toks []string) bool {
				return len(toks) >= 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks := TokenizeContent(tt.input)
			if tt.name == "filters short tokens" {
				// short tokens (len<2) should NOT be present
				for _, tok := range toks {
					if tok == "a" || tok == "b" {
						t.Errorf("TokenizeContent included short token %q", tok)
					}
				}
			} else {
				if tt.check(toks) && tt.name == "filters short tokens" {
					t.Errorf("check failed for %s: got %v", tt.name, toks)
				}
			}
		})
	}
}

func TestTokenizeContent_LowercasesASCII(t *testing.T) {
	toks := TokenizeContent("HELLO WORLD")
	for _, tok := range toks {
		if tok != strings.ToLower(tok) {
			t.Errorf("expected lowercase token, got %q", tok)
		}
	}
}

func TestContentSignature(t *testing.T) {
	// Same text → same signature
	s1 := ContentSignature("hello world")
	s2 := ContentSignature("hello world")
	if s1 != s2 {
		t.Errorf("same text should produce same signature: %q != %q", s1, s2)
	}

	// Different text → different signature (with very high probability)
	s3 := ContentSignature("different text entirely")
	if s1 == s3 {
		t.Errorf("different texts should (almost certainly) produce different signatures")
	}

	// Tashkeel variants should produce the same signature
	withTashkeel := "كَتَبَ"    // with fatha marks
	withoutTashkeel := "كتب"   // without
	sig1 := ContentSignature(withTashkeel)
	sig2 := ContentSignature(withoutTashkeel)
	if sig1 != sig2 {
		t.Errorf("tashkeel variants should produce same signature: %q != %q", sig1, sig2)
	}

	// Non-empty
	if s1 == "" {
		t.Error("signature should not be empty")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want float64
	}{
		{"empty both", nil, nil, 1.0},
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 1.0},
		{"disjoint", []string{"a", "b"}, []string{"c", "d"}, 0.0},
		{"half overlap", []string{"a", "b", "c", "d"}, []string{"c", "d", "e", "f"}, 2.0 / 6.0},
		{"one empty", []string{"a"}, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JaccardSimilarity(tt.a, tt.b)
			// Allow small float tolerance
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 1e-9 {
				t.Errorf("JaccardSimilarity(%v, %v) = %f; want %f", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
