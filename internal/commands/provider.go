package commands

import (
	"fmt"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// providerFor resolves a named provider from global config and constructs a
// Provider. namedProvider is taken from the book's LLMSettings (e.g.
// cfg.LLM.WriteProvider); providerFlag is an optional CLI --provider override
// that takes precedence.
//
// Resolution order:
//  1. providerFlag (from --provider CLI flag)
//  2. namedProvider (from book.yaml llm.*_provider)
//  3. global default_provider
//  4. legacy ANTHROPIC_API_KEY / api_key
func providerFor(providerFlag, namedProvider string) (llm.Provider, error) {
	name := providerFlag
	if name == "" {
		name = namedProvider
	}

	pcfg, err := config.ProviderConfigFor(name)
	if err != nil {
		return nil, fmt.Errorf("resolving provider %q: %w", name, err)
	}

	log.Debug("provider resolved", "name", name, "type", pcfg.Type)

	// llm.ProviderConfig is a type alias for config.ProviderConfig — no conversion needed.
	p, err := llm.NewProvider(pcfg)
	if err != nil {
		return nil, fmt.Errorf("creating provider %q: %w", name, err)
	}
	return llm.NewRetryProvider(p, name), nil
}

// providerWithFallback constructs a primary provider wrapped in retry, and if
// fallbackName is non-empty, chains it as a FallbackProvider after the primary.
func providerWithFallback(providerFlag, namedProvider, fallbackName string) (llm.Provider, error) {
	primary, err := providerFor(providerFlag, namedProvider)
	if err != nil {
		return nil, err
	}
	if fallbackName == "" {
		return primary, nil
	}
	fallback, err := providerFor("", fallbackName)
	if err != nil {
		log.Warn("provider: fallback provider unavailable, using primary only", "fallback", fallbackName, "err", err)
		return primary, nil
	}
	log.Debug("provider: fallback chain active", "fallback", fallbackName)
	return llm.NewFallbackProvider(
		[]llm.Provider{primary, fallback},
		[]string{namedProvider, fallbackName},
	), nil
}
