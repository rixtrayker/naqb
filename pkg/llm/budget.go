package llm

import (
	"fmt"
	"sync/atomic"
)

// BudgetTracker tracks cumulative token spend during a CLI session and
// triggers a degraded mode when a spending limit is crossed.
//
// All operations are lock-free using atomic int64 (micro-USD = USD * 1e6).
// State is process-scoped — it resets on program restart, which is correct
// for a CLI tool where sessions are naturally bounded.
type BudgetTracker struct {
	limitMicro int64 // 0 = unlimited
	spentMicro atomic.Int64
	degraded   atomic.Bool
}

// NewBudgetTracker creates a tracker with the given USD limit (0 = unlimited).
func NewBudgetTracker(limitUSD float64) *BudgetTracker {
	return &BudgetTracker{
		limitMicro: int64(limitUSD * 1_000_000),
	}
}

// SessionBudget is the process-wide budget tracker.
// Set its limit via SetLimit before running any pipeline.
var SessionBudget = NewBudgetTracker(0)

// SetLimit updates the spending limit in USD. Pass 0 for unlimited.
func (b *BudgetTracker) SetLimit(limitUSD float64) {
	b.limitMicro = int64(limitUSD * 1_000_000)
	b.degraded.Store(false)
	b.spentMicro.Store(0)
}

// Record adds the cost of a completed LLM call.
// modelID must match a key in KnownModels; unknown models contribute $0.
func (b *BudgetTracker) Record(inputTok, outputTok int, modelID string) {
	caps, ok := KnownModels[modelID]
	if !ok || (inputTok == 0 && outputTok == 0) {
		return
	}
	costUSD := float64(inputTok)/1_000_000*caps.InputCostPerMTok() +
		float64(outputTok)/1_000_000*caps.OutputCostPerMTok()
	micro := int64(costUSD * 1_000_000)
	spent := b.spentMicro.Add(micro)

	if b.limitMicro > 0 && spent >= b.limitMicro && !b.degraded.Load() {
		b.degraded.Store(true)
	}
}

// SpentUSD returns the cumulative spend in USD.
func (b *BudgetTracker) SpentUSD() float64 {
	return float64(b.spentMicro.Load()) / 1_000_000
}

// Degraded returns true once the spending limit has been crossed.
// When degraded, callers should route to a cheaper model tier.
func (b *BudgetTracker) Degraded() bool {
	if b.limitMicro == 0 {
		return false // unlimited
	}
	return b.degraded.Load()
}

// String returns a short spend summary for display.
func (b *BudgetTracker) String() string {
	if b.limitMicro == 0 {
		return fmt.Sprintf("$%.4f (no limit)", b.SpentUSD())
	}
	return fmt.Sprintf("$%.4f / $%.4f", b.SpentUSD(), float64(b.limitMicro)/1_000_000)
}
