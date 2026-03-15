// Package config handles global (~/.naqb/config.yaml) and per-project (book.yaml) configuration.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderConfig holds configuration for a named LLM provider entry.
type ProviderConfig struct {
	// Type is the provider kind: "anthropic", "openai-compat", "gemini".
	Type string `yaml:"type"`
	// APIKey for this provider. Falls back to environment variable if empty.
	APIKey string `yaml:"api_key,omitempty"`
	// BaseURL for OpenAI-compatible providers (Ollama, DeepSeek, z.ai, Mistral, etc.).
	BaseURL string `yaml:"base_url,omitempty"`
}

// GlobalConfig holds ~/.naqb/config.yaml
type GlobalConfig struct {
	// APIKey is the default Anthropic API key (legacy field, kept for compatibility).
	APIKey string `yaml:"api_key,omitempty"`
	// Providers is a named map of LLM provider configurations.
	// Keys are arbitrary names like "anthropic", "deepseek", "local-ollama".
	Providers map[string]ProviderConfig `yaml:"providers,omitempty"`
	// DefaultProvider is the provider name used when a stage doesn't specify one.
	DefaultProvider string `yaml:"default_provider,omitempty"`
	// DefaultModel is the fallback model when no per-stage model is set.
	DefaultModel string `yaml:"default_model,omitempty"`
	// Editor is the preferred $EDITOR override for opening files.
	Editor string `yaml:"editor,omitempty"`
	// Extra holds arbitrary user-defined key-value pairs.
	Extra map[string]string `yaml:"extra,omitempty"`
}

// NaqbDir returns the path to the ~/.naqb directory.
func NaqbDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".naqb")
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	return filepath.Join(NaqbDir(), "config.yaml")
}

// LoadGlobal reads the global config, creating defaults if absent.
func LoadGlobal() (*GlobalConfig, error) {
	path := GlobalConfigPath()
	cfg := &GlobalConfig{}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading global config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return cfg, nil
}

// SaveGlobal writes the global config to disk.
func SaveGlobal(cfg *GlobalConfig) error {
	path := GlobalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling global config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// APIKey returns the Anthropic API key, checking in order:
//  1. ANTHROPIC_API_KEY environment variable
//  2. macOS Keychain (service name: ANTHROPIC_API_KEY)
//  3. ~/.naqb/config.yaml default provider → legacy api_key field
func APIKey() (string, error) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}
	if key := keychainGet("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return "", err
	}
	if cfg.DefaultProvider != "" {
		if p, ok := cfg.Providers[cfg.DefaultProvider]; ok && p.APIKey != "" {
			return p.APIKey, nil
		}
	}
	if cfg.APIKey != "" {
		return cfg.APIKey, nil
	}
	return "", fmt.Errorf("no API key found — set ANTHROPIC_API_KEY or run: nqb config")
}

// keychainGet retrieves a password stored under the given service name from the
// macOS Keychain. Returns "" on any error (non-macOS, not found, etc.).
func keychainGet(service string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-a", os.Getenv("USER"), "-s", service, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ComposioAPIKey returns the Composio API key, checking in order:
//  1. COMPOSIO_API_KEY environment variable
//  2. macOS Keychain (service name: COMPOSIO_API_KEY)
//  3. ~/.naqb/config.yaml providers["composio"].api_key
func ComposioAPIKey() string {
	if key := os.Getenv("COMPOSIO_API_KEY"); key != "" {
		return key
	}
	if key := keychainGet("COMPOSIO_API_KEY"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["composio"]; ok && p.APIKey != "" {
		return p.APIKey
	}
	return ""
}

// GeminiAPIKey returns the Gemini API key from, in order:
//  1. GEMINI_API_KEY environment variable
//  2. macOS Keychain (service name: GEMINI_API_KEY)
//  3. ~/.naqb/config.yaml providers["gemini"].api_key
func GeminiAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	if key := keychainGet("GEMINI_API_KEY"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["gemini"]; ok && p.APIKey != "" {
		return p.APIKey
	}
	return ""
}

// ProviderConfigFor returns the ProviderConfig for a named provider, or the
// default Anthropic config if the name is empty or not found.
func ProviderConfigFor(name string) (ProviderConfig, error) {
	cfg, err := LoadGlobal()
	if err != nil {
		return ProviderConfig{}, err
	}
	if name == "" {
		name = cfg.DefaultProvider
	}
	if name != "" {
		if p, ok := cfg.Providers[name]; ok {
			return p, nil
		}
	}
	// Fall back: synthesise an Anthropic config from the legacy api_key.
	key, err := APIKey()
	if err != nil {
		return ProviderConfig{}, err
	}
	return ProviderConfig{Type: "anthropic", APIKey: key}, nil
}
