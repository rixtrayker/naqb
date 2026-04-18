package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/keycheck"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// RunPreflight checks that at least one required API key is present for the
// given command. Returns a descriptive error if no key is found.
func RunPreflight(commandName string) error {
	result := keycheck.CheckCommand(commandName)
	if result.OK {
		return nil
	}
	fmt.Fprintf(os.Stderr, "\nnqb: missing API key for `%s`\n", commandName)
	fmt.Fprintf(os.Stderr, "  Need one of: %s\n", strings.Join(result.Missing, ", "))
	fmt.Fprintf(os.Stderr, "  Run `nqb keys --set <NAME>` to save a key.\n\n")
	return fmt.Errorf("preflight: no API key for %q", commandName)
}

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
//
// If global config has default_fallback_provider set, the returned provider is
// automatically wrapped in a FallbackProvider chain so errors transparently
// retry on the fallback without any per-book configuration.
func providerFor(providerFlag, namedProvider string) (llm.Provider, error) {
	return providerWithFallback(providerFlag, namedProvider, "")
}

// providerWithFallback constructs a primary provider wrapped in retry, then
// chains a fallback provider if one is specified.
//
// fallbackName resolution order:
//  1. explicit fallbackName argument (from book.yaml llm.fallback_provider)
//  2. global config default_fallback_provider
//  3. none — single-provider mode
func providerWithFallback(providerFlag, namedProvider, fallbackName string) (llm.Provider, error) {
	primary, err := buildProvider(providerFlag, namedProvider)
	if err != nil {
		return nil, err
	}

	// Resolve effective fallback: explicit arg wins, then global default.
	effective := fallbackName
	if effective == "" {
		if gcfg, err := config.LoadGlobal(); err == nil {
			effective = gcfg.DefaultFallbackProvider
		}
	}
	if effective == "" {
		return primary, nil
	}

	// Determine the resolved primary name for logging.
	primaryName := providerFlag
	if primaryName == "" {
		primaryName = namedProvider
	}
	if primaryName == "" {
		if gcfg, err := config.LoadGlobal(); err == nil && gcfg.DefaultProvider != "" {
			primaryName = gcfg.DefaultProvider
		}
	}

	fallback, err := buildProvider("", effective)
	if err != nil {
		log.Warn("provider: fallback unavailable, using primary only", "fallback", effective, "err", err)
		return primary, nil
	}
	log.Debug("provider: fallback chain active", "primary", primaryName, "fallback", effective)
	return llm.NewFallbackProvider(
		[]llm.Provider{primary, fallback},
		[]string{primaryName, effective},
	), nil
}

// buildProvider constructs a single provider (no fallback chain).
func buildProvider(providerFlag, namedProvider string) (llm.Provider, error) {
	name := providerFlag
	if name == "" {
		name = namedProvider
	}

	pcfg, err := config.ProviderConfigFor(name)
	if err != nil {
		return nil, fmt.Errorf("resolving provider %q: %w", name, err)
	}

	log.Debug("provider resolved", "name", name, "type", pcfg.Type)

	p, err := llm.NewProvider(pcfg)
	if err != nil {
		return nil, fmt.Errorf("creating provider %q: %w", name, err)
	}
	return llm.NewRetryProvider(p, name), nil
}
