package llm

import (
	"fmt"
	"sync"
	"time"
)

// CBState represents the circuit breaker state.
type CBState int

const (
	CBClosed   CBState = iota // normal: requests pass through
	CBOpen                    // tripped: requests blocked
	CBHalfOpen                // testing: one probe request allowed
)

const (
	cbFailureThreshold = 5
	cbResetTimeout     = 30 * time.Second
)

// CircuitBreaker is a simple per-provider state machine.
// Trips open after cbFailureThreshold consecutive failures.
// Resets to half-open after cbResetTimeout, then closes on success.
type CircuitBreaker struct {
	mu          sync.Mutex
	state       CBState
	failures    int
	lastFailure time.Time
}

// Allow returns true if the request should be attempted.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CBOpen:
		if time.Since(cb.lastFailure) >= cbResetTimeout {
			cb.state = CBHalfOpen
			return true
		}
		return false
	default:
		return true
	}
}

// RecordSuccess resets the circuit to closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CBClosed
	cb.failures = 0
}

// RecordFailure increments the failure count and opens the circuit when the threshold is crossed.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cbFailureThreshold {
		cb.state = CBOpen
	}
}

// ── global registry ───────────────────────────────────────────────────────────

var (
	cbMu       sync.Mutex
	cbRegistry = map[string]*CircuitBreaker{}
)

// CBFor returns the shared CircuitBreaker for the named provider.
// Creates one lazily on first call. State is process-scoped (resets on restart).
func CBFor(providerName string) *CircuitBreaker {
	cbMu.Lock()
	defer cbMu.Unlock()
	if cb, ok := cbRegistry[providerName]; ok {
		return cb
	}
	cb := &CircuitBreaker{}
	cbRegistry[providerName] = cb
	return cb
}

// CircuitOpenError is returned when a request is blocked by an open circuit.
type CircuitOpenError struct {
	Provider string
}

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf("llm: circuit open for provider %q — too many recent failures", e.Provider)
}
