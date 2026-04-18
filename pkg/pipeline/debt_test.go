package pipeline

import (
	"strings"
	"testing"
)

func TestContextDebt_RecordWithinBudget(t *testing.T) {
	d := &ContextDebt{TokenBudget: 1000, Action: DebtFail}
	v := d.Record("stage1", 100, 200)
	if v != nil {
		t.Error("expected no violation within budget")
	}
	if d.TokensUsed != 300 {
		t.Errorf("TokensUsed = %d, want 300", d.TokensUsed)
	}
}

func TestContextDebt_RecordExceedsBudget(t *testing.T) {
	d := &ContextDebt{TokenBudget: 500, Action: DebtDegrade}
	v := d.Record("stage1", 300, 250)
	if v == nil {
		t.Fatal("expected violation when budget exceeded")
	}
	if v.StageID != "stage1" {
		t.Errorf("StageID = %q, want stage1", v.StageID)
	}
	if v.Action != DebtDegrade {
		t.Errorf("Action = %q, want DEGRADE", v.Action)
	}
	if !strings.Contains(v.Reason, "token budget exceeded") {
		t.Errorf("Reason = %q, expected budget exceeded", v.Reason)
	}
}

func TestContextDebt_MultipleViolations(t *testing.T) {
	d := &ContextDebt{TokenBudget: 100, Action: DebtFail}
	d.Record("a", 50, 60) // 110 > 100
	d.Record("b", 10, 10) // 130 > 100

	if len(d.Violations) != 2 {
		t.Errorf("Violations = %d, want 2", len(d.Violations))
	}
	if !d.HasViolations() {
		t.Error("expected HasViolations = true")
	}
}

func TestContextDebt_NoBudget(t *testing.T) {
	d := &ContextDebt{TokenBudget: 0, Action: DebtFail}
	v := d.Record("stage", 10000, 10000)
	if v != nil {
		t.Error("expected no violation with unlimited budget (0)")
	}
}

func TestContextDebt_Summary_WithBudget(t *testing.T) {
	d := &ContextDebt{TokenBudget: 1000, TokensUsed: 500, Violations: make([]DebtViolation, 1)}
	s := d.Summary()
	if !strings.Contains(s, "500/1000") {
		t.Errorf("Summary = %q, expected 500/1000", s)
	}
	if !strings.Contains(s, "1 violations") {
		t.Errorf("Summary = %q, expected 1 violations", s)
	}
}

func TestContextDebt_Summary_NoBudget(t *testing.T) {
	d := &ContextDebt{TokenBudget: 0, TokensUsed: 750}
	s := d.Summary()
	if !strings.Contains(s, "750 used") {
		t.Errorf("Summary = %q, expected 750 used", s)
	}
	if !strings.Contains(s, "no budget limit") {
		t.Errorf("Summary = %q, expected no budget limit", s)
	}
}
