package pipeline

import "fmt"

// DebtAction defines how a pipeline handles context debt (resource limits).
type DebtAction string

const (
	DebtFail       DebtAction = "FAIL"       // abort the pipeline
	DebtDegrade    DebtAction = "DEGRADE"    // continue with reduced capability
	DebtSubstitute DebtAction = "SUBSTITUTE" // swap to cheaper model
	DebtHumanGate  DebtAction = "HUMAN_GATE" // pause and wait for human
)

// ContextDebt tracks resource consumption and policy violations during a DAG run.
type ContextDebt struct {
	// TokensUsed is the total token count so far in this run.
	TokensUsed int
	// TokenBudget is the maximum allowed (0 = unlimited).
	TokenBudget int
	// Violations is a list of policy breaches recorded during the run.
	Violations []DebtViolation
	// Action is the policy applied when budget is exceeded.
	Action DebtAction
}

// DebtViolation records a single budget/policy breach.
type DebtViolation struct {
	StageID string
	Reason  string
	Action  DebtAction
}

// Record adds token usage and checks if the budget is exceeded.
// Returns a DebtViolation if the budget is exceeded, nil otherwise.
func (d *ContextDebt) Record(stageID string, tokensIn, tokensOut int) *DebtViolation {
	d.TokensUsed += tokensIn + tokensOut
	if d.TokenBudget > 0 && d.TokensUsed > d.TokenBudget {
		v := DebtViolation{
			StageID: stageID,
			Reason:  fmt.Sprintf("token budget exceeded: %d > %d", d.TokensUsed, d.TokenBudget),
			Action:  d.Action,
		}
		d.Violations = append(d.Violations, v)
		return &v
	}
	return nil
}

// HasViolations reports whether any policy breaches occurred.
func (d *ContextDebt) HasViolations() bool {
	return len(d.Violations) > 0
}

// Summary returns a human-readable summary of the debt state.
func (d *ContextDebt) Summary() string {
	if d.TokenBudget > 0 {
		return fmt.Sprintf("tokens: %d/%d used (%d violations)", d.TokensUsed, d.TokenBudget, len(d.Violations))
	}
	return fmt.Sprintf("tokens: %d used (no budget limit)", d.TokensUsed)
}
