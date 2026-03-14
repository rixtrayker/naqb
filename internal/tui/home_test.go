package tui

import (
	"testing"
	"time"
)

func TestHumanTime(t *testing.T) {
	now := time.Now()

	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-2 * 24 * time.Hour), "2d ago"},
	}
	for _, tc := range cases {
		got := humanTime(tc.t)
		if got != tc.want {
			t.Errorf("humanTime(%v ago) = %q, want %q", now.Sub(tc.t), got, tc.want)
		}
	}
}

func TestHumanTime_OldDate(t *testing.T) {
	// Older than 7 days should return formatted date "Jan 2"
	old := time.Date(2025, time.January, 15, 12, 0, 0, 0, time.UTC)
	got := humanTime(old)
	want := "Jan 15"
	if got != want {
		t.Errorf("humanTime(old) = %q, want %q", got, want)
	}
}

func TestMaxFunc(t *testing.T) {
	cases := []struct {
		a, b, want int
	}{
		{3, 5, 5},
		{5, 3, 5},
		{4, 4, 4},
		{0, -1, 0},
	}
	for _, tc := range cases {
		if got := max(tc.a, tc.b); got != tc.want {
			t.Errorf("max(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
