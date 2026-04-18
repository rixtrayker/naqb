package llm

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/amr/naqb/pkg/log"
)

// RetryConfig controls retry behaviour for transient API errors.
type RetryConfig struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	JitterFactor float64
}

// DefaultRetryConfig is three attempts with 1s base delay and 30% jitter.
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	BaseDelay:    time.Second,
	JitterFactor: 0.3,
}

// RetryProvider wraps any Provider and retries Complete on retryable errors.
// Stream is never retried — partial output cannot be cleanly resumed.
type RetryProvider struct {
	inner  Provider
	config RetryConfig
	name   string
}

// NewRetryProvider wraps p with default retry behaviour.
func NewRetryProvider(p Provider, name string) *RetryProvider {
	return &RetryProvider{inner: p, config: DefaultRetryConfig, name: name}
}

func (r *RetryProvider) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		if !CBFor(r.name).Allow() {
			return "", &CircuitOpenError{Provider: r.name}
		}

		result, err := r.inner.Complete(ctx, model, system, messages, maxTokens)
		if err == nil {
			CBFor(r.name).RecordSuccess()
			return result, nil
		}

		CBFor(r.name).RecordFailure()
		lastErr = err

		if !isRetryable(err) || attempt == r.config.MaxAttempts {
			break
		}

		delay := r.backoff(attempt)
		log.Warn("llm: retrying after transient error", "provider", r.name, "attempt", attempt, "delay", delay, "err", err)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", lastErr
}

func (r *RetryProvider) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	// Streaming is never retried — pass through directly.
	return r.inner.Stream(ctx, model, system, messages, maxTokens, onDelta)
}

// LastTokens delegates to the inner provider if it supports TokenReporter.
func (r *RetryProvider) LastTokens() (int, int) {
	if tr, ok := r.inner.(TokenReporter); ok {
		return tr.LastTokens()
	}
	return 0, 0
}

func (r *RetryProvider) backoff(attempt int) time.Duration {
	base := float64(r.config.BaseDelay) * float64(attempt)
	jitter := base * r.config.JitterFactor * (rand.Float64()*2 - 1) //nolint:gosec
	d := time.Duration(base + jitter)
	if d < 0 {
		d = r.config.BaseDelay
	}
	return d
}

// isRetryable returns true for transient errors that are safe to retry.
// Auth and credit errors are permanent — retrying them would waste quota.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Permanent failures — never retry.
	if IsAuthError(err) || IsCreditError(err) {
		return false
	}
	// Transient rate limit — retry with backoff.
	if IsRateLimit(err) {
		return true
	}
	// Provider unavailable — retry only for 5xx codes.
	var u *ErrProviderUnavailable
	if errors.As(err, &u) {
		return u.StatusCode >= 500
	}
	// Network timeouts.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
