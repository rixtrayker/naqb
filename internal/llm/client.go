package llm

import "fmt"

// Client is an alias for Provider kept for backwards compatibility.
// New code should use the Provider interface directly.
type Client = AnthropicProvider

// New creates an AnthropicProvider with the provided API key.
// Deprecated: use NewAnthropic directly or NewProvider for multi-provider support.
func New(apiKey string) *AnthropicProvider {
	return NewAnthropic(apiKey)
}

// NewProvider constructs a Provider from a named ProviderConfig.
// Supported types: "anthropic", "openai-compat" (for Ollama/DeepSeek/z.ai), "gemini".
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case "", "anthropic":
		return NewAnthropic(cfg.APIKey), nil
	case "openai-compat":
		// Phase 2: implement OpenAI-compatible provider
		return nil, fmt.Errorf("openai-compat provider not yet implemented")
	case "gemini":
		// Phase 2: implement Gemini provider
		return nil, fmt.Errorf("gemini provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider type %q", cfg.Type)
	}
}
