package agent

import (
	"testing"

	"github.com/amr/naqb/pkg/config"
)

func TestNewProvider_OpenRouter(t *testing.T) {
	cases := []struct {
		name string
		typ  string
	}{
		{"empty type defaults to openrouter", ""},
		{"explicit openrouter", "openrouter"},
		{"openai-compat", "openai-compat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewProvider(config.ProviderConfig{Type: tc.typ, APIKey: "test-key"})
			if err != nil {
				t.Fatalf("NewProvider(%q): %v", tc.typ, err)
			}
			if p == nil {
				t.Error("expected non-nil provider")
			}
		})
	}
}

func TestNewProvider_Anthropic(t *testing.T) {
	p, err := NewProvider(config.ProviderConfig{Type: "anthropic", APIKey: "sk-ant-test"})
	if err != nil {
		t.Fatalf("NewProvider(anthropic): %v", err)
	}
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

func TestNewProvider_Bedrock(t *testing.T) {
	p, err := NewProvider(config.ProviderConfig{Type: "bedrock"})
	if err != nil {
		t.Fatalf("NewProvider(bedrock): %v", err)
	}
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

func TestNewProvider_UnknownType(t *testing.T) {
	_, err := NewProvider(config.ProviderConfig{Type: "unsupported-xyz"})
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}
