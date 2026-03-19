package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

// humanTime now delegates to go-humanize (dustin/go-humanize).
// Tests verify approximate output shapes rather than exact strings,
// since go-humanize uses prose like "5 minutes ago" not "5m ago".

func TestHumanTime_Recent(t *testing.T) {
	now := time.Now()

	cases := []struct {
		t        time.Time
		contains string
		desc     string
	}{
		{now.Add(-30 * time.Second), "seconds ago", "30s ago"},
		{now.Add(-5 * time.Minute), "minutes ago", "5 minutes ago"},
		{now.Add(-3 * time.Hour), "hours ago", "3 hours ago"},
		{now.Add(-48 * time.Hour), "days ago", "2 days ago"},
	}
	for _, tc := range cases {
		got := humanTime(tc.t)
		if !strings.Contains(got, tc.contains) {
			t.Errorf("humanTime(%s): got %q, want substring %q", tc.desc, got, tc.contains)
		}
	}
}

func TestHumanTime_Old(t *testing.T) {
	// go-humanize returns "X years ago" for old dates.
	old := time.Date(2020, time.January, 15, 12, 0, 0, 0, time.UTC)
	got := humanTime(old)
	if !strings.Contains(got, "ago") {
		t.Errorf("humanTime(old date) = %q, expected 'ago' in output", got)
	}
}

func TestHumanTime_Future(t *testing.T) {
	// go-humanize handles future times too
	future := time.Now().Add(5 * time.Minute)
	got := humanTime(future)
	// Should be non-empty
	if got == "" {
		t.Error("humanTime(future) returned empty string")
	}
}

// applyFilter tests: verify fuzzy search matches and non-matches.

func TestHomeView_FillsHeight(t *testing.T) {
	m := &homeModel{
		width:  80,
		height: 40,
	}
	m.search = textinput.New()
	// 0 projects, loading=false → "No projects" view
	m.filtered = nil
	m.projects = nil

	output := m.View()
	lines := strings.Split(output, "\n")
	// The view should fill approximately the full height (±4 for rendering variance)
	if len(lines) < m.height-4 {
		t.Errorf("View() produced %d lines, expected ~%d (fills height)", len(lines), m.height)
	}
}

func TestApplyFilter_Empty(t *testing.T) {
	m := &homeModel{}
	m.search.SetValue("")
	m.applyFilter()
	// empty query → all projects returned
	if m.filtered != nil {
		t.Error("empty filter should leave filtered nil (same as projects)")
	}
}

func TestApplyFilter_NoProjects(t *testing.T) {
	m := &homeModel{}
	m.search.SetValue("something")
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Errorf("expected 0 results with no projects, got %d", len(m.filtered))
	}
}
