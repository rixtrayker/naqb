package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyError_HTTP401(t *testing.T) {
	err := ClassifyError("openrouter", 401, "unauthorized")
	var e *ErrAuthFailed
	if !errors.As(err, &e) {
		t.Fatalf("expected ErrAuthFailed, got %T: %v", err, err)
	}
	if e.Provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter", e.Provider)
	}
}

func TestClassifyError_HTTP402(t *testing.T) {
	err := ClassifyError("openrouter", 402, "no credit")
	var e *ErrCreditExhausted
	if !errors.As(err, &e) {
		t.Fatalf("expected ErrCreditExhausted, got %T: %v", err, err)
	}
}

func TestClassifyError_HTTP429_RateLimit(t *testing.T) {
	err := ClassifyError("openrouter", 429, "too many requests")
	var e *ErrRateLimit
	if !errors.As(err, &e) {
		t.Fatalf("expected ErrRateLimit, got %T: %v", err, err)
	}
}

func TestClassifyError_HTTP429_Credit(t *testing.T) {
	for _, body := range []string{
		"insufficient_quota: you have no credit left",
		"billing limit reached",
		"quota exceeded",
	} {
		err := ClassifyError("openrouter", 429, body)
		var e *ErrCreditExhausted
		if !errors.As(err, &e) {
			t.Errorf("body=%q: expected ErrCreditExhausted, got %T: %v", body, err, err)
		}
	}
}

func TestClassifyError_HTTP500(t *testing.T) {
	err := ClassifyError("openrouter", 503, "service unavailable")
	var e *ErrProviderUnavailable
	if !errors.As(err, &e) {
		t.Fatalf("expected ErrProviderUnavailable, got %T: %v", err, err)
	}
	if e.StatusCode != 503 {
		t.Errorf("status = %d, want 503", e.StatusCode)
	}
}

func TestClassifyError_TextHeuristics_Auth(t *testing.T) {
	for _, body := range []string{
		"Invalid API key provided",
		"authentication failed: bad credentials",
		"unauthorized access",
	} {
		err := ClassifyError("anthropic", 0, body)
		if !IsAuthError(err) {
			t.Errorf("body=%q: expected IsAuthError, got %T: %v", body, err, err)
		}
	}
}

func TestClassifyError_TextHeuristics_Credit(t *testing.T) {
	err := ClassifyError("anthropic", 0, "insufficient funds in your account")
	if !IsCreditError(err) {
		t.Errorf("expected IsCreditError, got %T: %v", err, err)
	}
}

func TestIsProviderError(t *testing.T) {
	cases := []error{
		&ErrAuthFailed{Provider: "p"},
		&ErrCreditExhausted{Provider: "p"},
		&ErrRateLimit{Provider: "p"},
		&ErrProviderUnavailable{Provider: "p", StatusCode: 503},
	}
	for _, err := range cases {
		if !IsProviderError(err) {
			t.Errorf("IsProviderError(%T) = false, want true", err)
		}
	}
}

func TestIsRetryable_TypedErrors(t *testing.T) {
	cases := []struct {
		err      error
		wantTrue bool
	}{
		{&ErrAuthFailed{Provider: "p"}, false},
		{&ErrCreditExhausted{Provider: "p"}, false},
		{&ErrRateLimit{Provider: "p"}, true},
		{&ErrProviderUnavailable{Provider: "p", StatusCode: 503}, true},
		{&ErrProviderUnavailable{Provider: "p", StatusCode: 400}, false},
	}
	for _, tc := range cases {
		got := isRetryable(tc.err)
		if got != tc.wantTrue {
			t.Errorf("isRetryable(%T) = %v, want %v", tc.err, got, tc.wantTrue)
		}
	}
}

func TestClassifyBedrockError(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantAuth bool
		wantRate bool
		wantProv bool // provider unavailable
		wantNone bool // generic error (not typed)
	}{
		{"UnrecognizedClientException", "UnrecognizedClientException: key invalid", true, false, false, false},
		{"AccessDeniedException", "AccessDeniedException: no access", true, false, false, false},
		{"ExpiredTokenException", "ExpiredTokenException: token expired", true, false, false, false},
		{"ThrottlingException", "ThrottlingException: slow down", false, true, false, false},
		{"TooManyRequestsException", "TooManyRequestsException: rate limited", false, true, false, false},
		{"ServiceUnavailableException", "ServiceUnavailableException: try later", false, false, true, false},
		{"ModelErrorException", "ModelErrorException: model crashed", false, false, true, false},
		{"ValidationException", "ValidationException: bad input", false, false, false, true},
		{"unknown error", "something completely different", false, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyBedrockError(fmt.Errorf("%s", tc.input))
			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if tc.wantAuth && !IsAuthError(err) {
				t.Errorf("expected ErrAuthFailed, got: %v", err)
			}
			if tc.wantRate && !IsRateLimit(err) {
				t.Errorf("expected ErrRateLimit, got: %v", err)
			}
			if tc.wantProv && !isUnavailableError(err) {
				t.Errorf("expected ErrProviderUnavailable, got: %v", err)
			}
			if tc.wantNone {
				if IsAuthError(err) || IsRateLimit(err) || isUnavailableError(err) {
					t.Errorf("expected generic error, got typed: %v", err)
				}
			}
		})
	}
}

func TestClassifyBedrockError_Nil(t *testing.T) {
	if err := classifyBedrockError(nil); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestErrorMessages(t *testing.T) {
	// Ensure all four types produce non-empty messages.
	errs := []error{
		&ErrAuthFailed{Provider: "p", Detail: "bad key"},
		&ErrCreditExhausted{Provider: "p", Detail: "no money"},
		&ErrRateLimit{Provider: "p", Detail: "slow down"},
		&ErrProviderUnavailable{Provider: "p", StatusCode: 502, Detail: "down"},
	}
	for _, err := range errs {
		if err.Error() == "" {
			t.Errorf("%T.Error() is empty", err)
		}
	}
}
