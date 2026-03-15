package llm

import "fmt"

// Client is an alias for OpenRouterProvider — the default backend.
// Kept as a concrete type alias so existing llm.New() callers still compile.
type Client = OpenRouterProvider

// New creates an OpenRouterProvider with the provided API key.
// This is the primary constructor; all commands should use this.
func New(apiKey string) *OpenRouterProvider {
	return NewOpenRouter(apiKey, "")
}

// NewProvider constructs a Provider from a ProviderConfig.
// Supported types: "openrouter" (default), "anthropic", "openai-compat".
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case "", "openrouter", "openai-compat":
		return NewOpenRouter(cfg.APIKey, cfg.BaseURL), nil
	case "anthropic":
		return NewAnthropic(cfg.APIKey), nil
	default:
		return nil, fmt.Errorf("unknown provider type %q (supported: openrouter, anthropic, openai-compat)", cfg.Type)
	}
}
