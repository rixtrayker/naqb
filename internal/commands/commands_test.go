package commands

import (
	"os"
	"testing"
)

// ── VersionCmd ───────────────────────────────────────────────────────────────

func TestVersionCmd_Runs(t *testing.T) {
	cmd := VersionCmd()
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "v" {
		t.Error("expected alias 'v'")
	}
}

func TestVersionCmd_GroupID(t *testing.T) {
	cmd := VersionCmd()
	if cmd.GroupID != "utility" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "utility")
	}
}

// ── ListCmd ──────────────────────────────────────────────────────────────────

func TestListCmd_Structure(t *testing.T) {
	cmd := ListCmd()
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "ls" {
		t.Error("expected alias 'ls'")
	}
	if cmd.GroupID != "management" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "management")
	}

	// Should have "chapters" subcommand
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "chapters" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'chapters' subcommand")
	}
}

func TestListCmd_ChaptersAlias(t *testing.T) {
	cmd := ListCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "chapters" {
			if len(sub.Aliases) == 0 || sub.Aliases[0] != "ch" {
				t.Error("expected 'ch' alias for chapters subcommand")
			}
			return
		}
	}
	t.Error("chapters subcommand not found")
}

// ── DoctorCmd ────────────────────────────────────────────────────────────────

func TestDoctorCmd_Structure(t *testing.T) {
	cmd := DoctorCmd()
	if cmd.Use != "doctor" {
		t.Errorf("Use = %q, want %q", cmd.Use, "doctor")
	}
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "doc" {
		t.Error("expected alias 'doc'")
	}
	if cmd.GroupID != "utility" {
		t.Errorf("GroupID = %q, want %q", cmd.GroupID, "utility")
	}
}

// ── OutputFlags ──────────────────────────────────────────────────────────────

func TestOutputFlags_Defaults(t *testing.T) {
	f := OutputFlags{}
	if f.IsVerbose() {
		t.Error("default Verbose should be false")
	}
	if f.IsQuiet() {
		t.Error("default Quiet should be false")
	}
}

func TestOutputFlags_NoColor_Flag(t *testing.T) {
	f := OutputFlags{NoColor: true}
	if !f.ColorDisabled() {
		t.Error("ColorDisabled should be true when NoColor flag is set")
	}
}

func TestOutputFlags_NoColor_Env(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	f := OutputFlags{}
	if !f.ColorDisabled() {
		t.Error("ColorDisabled should be true when NO_COLOR env is set")
	}
}

func TestOutputFlags_ColorEnabled_Default(t *testing.T) {
	// Ensure NO_COLOR is unset for this test
	os.Unsetenv("NO_COLOR")
	f := OutputFlags{}
	if f.ColorDisabled() {
		t.Error("ColorDisabled should be false by default")
	}
}

// ── helper functions ─────────────────────────────────────────────────────────

func TestFirstLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello\nworld", "hello"},
		{"single line", "single line"},
		{"", ""},
		{"first\nsecond\nthird", "first"},
	}
	for _, tt := range tests {
		got := firstLine(tt.input)
		if got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"─", 3, "───"},
		{"ab", 2, "abab"},
		{"x", 0, ""},
	}
	for _, tt := range tests {
		got := repeat(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("repeat(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestPassWarnFailLabels(t *testing.T) {
	if passLabel() != "PASS" {
		t.Errorf("passLabel() = %q", passLabel())
	}
	if warnLabel() != "WARN" {
		t.Errorf("warnLabel() = %q", warnLabel())
	}
	if failLabel() != "FAIL" {
		t.Errorf("failLabel() = %q", failLabel())
	}
}
