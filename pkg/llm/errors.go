package llm

import (
	"errors"
	"fmt"
	"strings"
)

// ErrAuthFailed is returned when the provider rejects the API key (HTTP 401).
type ErrAuthFailed struct {
	Provider string
	Detail   string
}

func (e *ErrAuthFailed) Error() string {
	return fmt.Sprintf("%s: authentication failed: %s", e.Provider, e.Detail)
}

// ErrCreditExhausted is returned when the account has no credit remaining
// (HTTP 402, or HTTP 429 with a billing/quota body).
type ErrCreditExhausted struct {
	Provider string
	Detail   string
}

func (e *ErrCreditExhausted) Error() string {
	return fmt.Sprintf("%s: credit exhausted: %s", e.Provider, e.Detail)
}

// ErrRateLimit is returned for transient rate-limit responses (HTTP 429).
type ErrRateLimit struct {
	Provider string
	Detail   string
}

func (e *ErrRateLimit) Error() string {
	return fmt.Sprintf("%s: rate limit exceeded: %s", e.Provider, e.Detail)
}

// ErrProviderUnavailable is returned for 5xx server errors.
type ErrProviderUnavailable struct {
	Provider   string
	StatusCode int
	Detail     string
}

func (e *ErrProviderUnavailable) Error() string {
	return fmt.Sprintf("%s: provider unavailable (HTTP %d): %s", e.Provider, e.StatusCode, e.Detail)
}

// ClassifyError maps an HTTP status code and message body to a typed sentinel.
// It is the single place where raw HTTP errors are converted to domain errors.
func ClassifyError(provider string, statusCode int, msg string) error {
	lower := strings.ToLower(msg)

	switch {
	case statusCode == 401:
		return &ErrAuthFailed{Provider: provider, Detail: msg}
	case statusCode == 402:
		return &ErrCreditExhausted{Provider: provider, Detail: msg}
	case statusCode == 429:
		if isCreditBody(lower) {
			return &ErrCreditExhausted{Provider: provider, Detail: msg}
		}
		return &ErrRateLimit{Provider: provider, Detail: msg}
	case statusCode >= 500:
		return &ErrProviderUnavailable{Provider: provider, StatusCode: statusCode, Detail: msg}
	}

	// Text heuristics for providers that embed status in the body.
	switch {
	case containsAny(lower, "invalid api key", "invalid_api_key", "unauthorized", "authentication", "api key"):
		return &ErrAuthFailed{Provider: provider, Detail: msg}
	case isCreditBody(lower):
		return &ErrCreditExhausted{Provider: provider, Detail: msg}
	}

	return fmt.Errorf("%s: API error: %s", provider, msg)
}

// isCreditBody returns true when the message body signals a billing/quota issue.
func isCreditBody(lower string) bool {
	return containsAny(lower, "credit", "quota", "billing", "insufficient_quota", "insufficient funds")
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// IsAuthError reports whether err is (or wraps) an ErrAuthFailed.
func IsAuthError(err error) bool {
	var t *ErrAuthFailed
	return errors.As(err, &t)
}

// IsCreditError reports whether err is (or wraps) an ErrCreditExhausted.
func IsCreditError(err error) bool {
	var t *ErrCreditExhausted
	return errors.As(err, &t)
}

// IsRateLimit reports whether err is (or wraps) an ErrRateLimit.
func IsRateLimit(err error) bool {
	var t *ErrRateLimit
	return errors.As(err, &t)
}

// IsProviderError reports whether err is any of the four provider sentinel types.
func IsProviderError(err error) bool {
	return IsAuthError(err) || IsCreditError(err) || IsRateLimit(err) || isUnavailableError(err)
}

func isUnavailableError(err error) bool {
	var t *ErrProviderUnavailable
	return errors.As(err, &t)
}
