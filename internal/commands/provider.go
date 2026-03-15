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

	// Convert config.ProviderConfig → llm.ProviderConfig (identical fields, different packages).
	p, err := llm.NewProvider(llm.ProviderConfig{
		Type:            pcfg.Type,
		APIKey:          pcfg.APIKey,
		BaseURL:         pcfg.BaseURL,
		SecretAccessKey: pcfg.SecretAccessKey,
		Region:          pcfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("creating provider %q: %w", name, err)
	}
	return p, nil
}
