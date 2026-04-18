// Package agent provides the naqb agentic loop on top of charm.land/fantasy.
// It exposes a single Agent type that wraps the fantasy agent, persists
// sessions to SQLite, and provides the standard naqb tool set.
package agent

import (
	"fmt"

	"charm.land/fantasy"
	fantasyAnthropic "charm.land/fantasy/providers/anthropic"
	fantasyOpenRouter "charm.land/fantasy/providers/openrouter"

	"github.com/amr/naqb/pkg/config"
)

// NewProvider constructs a fantasy.Provider from a ProviderConfig.
// Supported types: "openrouter" (default), "anthropic", "bedrock".
//
// The returned provider is ready to call LanguageModel() on.
func NewProvider(pcfg config.ProviderConfig) (fantasy.Provider, error) {
	switch pcfg.Type {
	case "", "openrouter", "openai-compat":
		p, err := fantasyOpenRouter.New(
			fantasyOpenRouter.WithAPIKey(pcfg.APIKey),
		)
		if err != nil {
			return nil, fmt.Errorf("agent: openrouter provider: %w", err)
		}
		return p, nil

	case "anthropic":
		p, err := fantasyAnthropic.New(
			fantasyAnthropic.WithAPIKey(pcfg.APIKey),
		)
		if err != nil {
			return nil, fmt.Errorf("agent: anthropic provider: %w", err)
		}
		return p, nil

	case "bedrock":
		// Bedrock uses the native Anthropic Bedrock integration in fantasy.
		// Credentials are picked up from standard AWS env vars / ~/.aws.
		p, err := fantasyAnthropic.New(
			fantasyAnthropic.WithBedrock(),
		)
		if err != nil {
			return nil, fmt.Errorf("agent: bedrock provider: %w", err)
		}
		return p, nil

	default:
		return nil, fmt.Errorf("agent: unknown provider type %q (supported: openrouter, anthropic, bedrock)", pcfg.Type)
	}
}

// NewProviderFromGlobalConfig builds a fantasy.Provider using the global
// configuration. It selects the default provider or falls back to OpenRouter.
func NewProviderFromGlobalConfig() (fantasy.Provider, string, error) {
	pcfg, err := config.ProviderConfigFor("")
	if err != nil {
		return nil, "", fmt.Errorf("agent: load provider config: %w", err)
	}
	p, err := NewProvider(pcfg)
	if err != nil {
		return nil, "", err
	}
	// Return the model name to use — prefer what the config specifies.
	return p, "", nil
}
