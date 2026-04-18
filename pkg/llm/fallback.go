package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/amr/naqb/pkg/log"
)

// FallbackProvider tries each provider in order, returning the first success.
// Useful when a primary provider has quota or reliability issues.
// Opt-in only — configured via book.yaml fallback_provider field.
type FallbackProvider struct {
	providers []Provider
	names     []string
}

// NewFallbackProvider creates a provider that tries each candidate in order.
func NewFallbackProvider(providers []Provider, names []string) *FallbackProvider {
	return &FallbackProvider{providers: providers, names: names}
}

func (f *FallbackProvider) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	var errs []error
	for i, p := range f.providers {
		name := f.names[i]
		result, err := p.Complete(ctx, model, system, messages, maxTokens)
		if err == nil {
			if i > 0 {
				log.Info("llm: fallback provider succeeded", "provider", name, "after_failures", i)
			}
			return result, nil
		}
		log.Warn("llm: provider failed, trying next", "provider", name, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", name, err))
	}
	return "", errors.Join(errs...)
}

func (f *FallbackProvider) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	if len(f.providers) == 0 {
		return "", fmt.Errorf("fallback: no providers configured")
	}
	var errs []error
	for i, p := range f.providers {
		name := f.names[i]
		result, err := p.Stream(ctx, model, system, messages, maxTokens, onDelta)
		if err == nil {
			if i > 0 {
				log.Info("llm: fallback provider succeeded (stream)", "provider", name, "after_failures", i)
			}
			return result, nil
		}
		// Only fall through on hard errors (auth/credit/unavailable).
		// Rate-limit retries are handled inside RetryProvider — if they bubble
		// up here the quota is genuinely exhausted.
		if !IsAuthError(err) && !IsCreditError(err) && !IsProviderError(err) {
			// Transient/unknown error on first provider — don't switch mid-stream.
			return result, err
		}
		log.Warn("llm: stream provider failed, trying next", "provider", name, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", name, err))
	}
	return "", errors.Join(errs...)
}

// LastTokens delegates to the first provider that supports TokenReporter.
func (f *FallbackProvider) LastTokens() (int, int) {
	for _, p := range f.providers {
		if tr, ok := p.(TokenReporter); ok {
			return tr.LastTokens()
		}
	}
	return 0, 0
}
