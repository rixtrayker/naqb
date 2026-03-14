package wordcount

import "testing"

func TestCount(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single word", "hello", 1},
		{"two words", "hello world", 2},
		{"leading/trailing spaces", "  hello world  ", 2},
		{"multiple spaces", "a   b   c", 3},
		{"tab separated", "a\tb\tc", 3},
		{"newline separated", "a\nb\nc", 3},
		{"arabic", "مرحبا بالعالم", 2},
		{"mixed", "hello مرحبا", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Count(tc.input); got != tc.want {
				t.Errorf("Count(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestProgressPercent(t *testing.T) {
	cases := []struct {
		p    Progress
		want int
	}{
		{Progress{Words: 0, Target: 3000}, 0},
		{Progress{Words: 3000, Target: 3000}, 100},
		{Progress{Words: 1500, Target: 3000}, 50},
		{Progress{Words: 4500, Target: 3000}, 100}, // capped at 100
		{Progress{Words: 100, Target: 0}, 0},        // no target → 0
	}
	for _, tc := range cases {
		if got := tc.p.Percent(); got != tc.want {
			t.Errorf("Percent() = %d, want %d (p=%+v)", got, tc.want, tc.p)
		}
	}
}

func TestProgressStatus(t *testing.T) {
	cases := []struct {
		p    Progress
		want string
	}{
		{Progress{Words: 0}, "empty"},
		{Progress{Words: 100, Min: 500}, "short"},
		{Progress{Words: 6000, Max: 5000}, "long"},
		{Progress{Words: 2000, Min: 1000, Max: 5000}, "ok"},
	}
	for _, tc := range cases {
		if got := tc.p.Status(); got != tc.want {
			t.Errorf("Status() = %q, want %q (p=%+v)", got, tc.want, tc.p)
		}
	}
}

func TestBar(t *testing.T) {
	p := Progress{Words: 1500, Target: 3000}
	bar := Bar(p, 10)
	if len(bar) == 0 {
		t.Error("Bar should return non-empty string")
	}
	// Should contain bracket chars
	if bar[0] != '[' {
		t.Errorf("Bar should start with '[', got %q", bar)
	}
}
