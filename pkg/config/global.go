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

// NaqbVersion is the current application version used to detect config upgrades.
// Bump this whenever new required config fields are added.
const NaqbVersion = "0.5.0"

// ProviderConfig holds configuration for a named LLM provider entry.
type ProviderConfig struct {
	// Type is the provider kind: "openrouter" (default), "anthropic", "openai-compat", "bedrock".
	Type string `yaml:"type"`
	// APIKey for this provider. Falls back to environment variable if empty.
	// For "bedrock", this is the AWS_ACCESS_KEY_ID.
	APIKey string `yaml:"api_key,omitempty"`
	// BaseURL for OpenAI-compatible providers (Ollama, DeepSeek, custom OpenRouter endpoints).
	BaseURL string `yaml:"base_url,omitempty"`
	// SecretAccessKey is the AWS secret access key (bedrock provider only).
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	// Region is the AWS region (bedrock provider only). Defaults to eu-central-1.
	Region string `yaml:"region,omitempty"`
}

// GlobalConfig holds ~/.naqb/config.yaml
type GlobalConfig struct {
	// APIKey is the legacy Anthropic API key field, kept for backwards compatibility.
	APIKey string `yaml:"api_key,omitempty"`
	// Providers is a named map of LLM provider configurations.
	Providers map[string]ProviderConfig `yaml:"providers,omitempty"`
	// DefaultProvider is the provider name used when a stage doesn't specify one.
	// Defaults to "openrouter".
	DefaultProvider string `yaml:"default_provider,omitempty"`
	// DefaultFallbackProvider is the global automatic fallback tried when the
	// primary provider fails (auth/credit/outage). Applied to all commands unless
	// the book's LLM.FallbackProvider already specifies one.
	// Example: set to "openrouter" when primary is "bedrock".
	DefaultFallbackProvider string `yaml:"default_fallback_provider,omitempty"`
	// DefaultModel is the fallback model when no per-stage model is set.
	DefaultModel string `yaml:"default_model,omitempty"`
	// Editor is the preferred $EDITOR override for opening files.
	Editor string `yaml:"editor,omitempty"`
	// SetupVersion records the NaqbVersion at which the user last ran setup.
	// Empty = never set up. On version bump nqb will prompt for new config fields.
	SetupVersion string `yaml:"setup_version,omitempty"`
	// DefaultVaultPath overrides where new books are stored.
	// Empty = use vault.DefaultVaultPath() heuristic.
	DefaultVaultPath string `yaml:"default_vault_path,omitempty"`
	// Extra holds arbitrary user-defined key-value pairs.
	Extra map[string]string `yaml:"extra,omitempty"`
}

// NeedsSetup returns true when the user should be walked through the setup wizard:
// - SetupVersion is empty (first run)
// - SetupVersion is older than currentVersion
func NeedsSetup(cfg *GlobalConfig) bool {
	if cfg == nil || cfg.SetupVersion == "" {
		return true
	}
	return versionLess(cfg.SetupVersion, NaqbVersion)
}

// versionLess does a simple lexicographic semver comparison sufficient for
// our monotonically increasing version strings (e.g. "0.4.0" < "0.5.0").
func versionLess(a, b string) bool {
	return a < b
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

// SaveGlobal writes the global config to disk using a merge strategy:
// it reads the existing YAML node tree, updates only the fields present in cfg,
// and writes back — preserving comments, key ordering, and any user-added fields.
// If the file does not exist yet it creates it from scratch.
func SaveGlobal(cfg *GlobalConfig) error {
	path := GlobalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	// Load existing raw node tree (or start with empty mapping).
	var root yaml.Node
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &root); err != nil {
			// Corrupted file — fall back to full overwrite.
			root = yaml.Node{}
		}
	}

	// Ensure we have a document → mapping structure.
	if root.Kind == 0 || len(root.Content) == 0 {
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}}
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		mapping = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content[0] = mapping
	}

	// Marshal the new config to a plain node tree for field extraction.
	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling global config: %w", err)
	}
	var newNode yaml.Node
	if err := yaml.Unmarshal(newData, &newNode); err != nil {
		return fmt.Errorf("parsing marshalled config: %w", err)
	}
	if newNode.Kind == yaml.DocumentNode && len(newNode.Content) > 0 {
		mergeYAMLMapping(mapping, newNode.Content[0])
	}

	out, err := yaml.Marshal(&root)
	if err != nil {
		return fmt.Errorf("serialising merged config: %w", err)
	}
	return os.WriteFile(path, out, 0o600)
}

// mergeYAMLMapping overlays src key-value pairs onto dst, adding new keys
// and updating existing ones but never removing keys that exist only in dst.
func mergeYAMLMapping(dst, src *yaml.Node) {
	if src.Kind != yaml.MappingNode || dst.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		key := src.Content[i].Value
		val := src.Content[i+1]

		// Find the key in dst.
		found := false
		for j := 0; j+1 < len(dst.Content); j += 2 {
			if dst.Content[j].Value == key {
				// Key exists — update value in-place (preserves comments on the key node).
				dst.Content[j+1] = val
				found = true
				break
			}
		}
		if !found {
			// New key — append.
			dst.Content = append(dst.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				val,
			)
		}
	}
}

// APIKey returns the active LLM API key, checking in order:
//  1. OPENROUTER_API_KEY environment variable
//  2. macOS Keychain (service: OPENROUTER_API_KEY)
//  3. ANTHROPIC_API_KEY environment variable (legacy fallback)
//  4. macOS Keychain (service: ANTHROPIC_API_KEY)
//  5. ~/.naqb/config.yaml default_provider → providers map → legacy api_key
func APIKey() (string, error) {
	// OpenRouter (preferred)
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key, nil
	}
	if key := keychainGet("OPENROUTER_API_KEY"); key != "" {
		return key, nil
	}
	// Anthropic (legacy fallback)
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}
	if key := keychainGet("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}
	// Config file
	cfg, err := LoadGlobal()
	if err != nil {
		return "", err
	}
	if cfg.DefaultProvider != "" {
		if p, ok := cfg.Providers[cfg.DefaultProvider]; ok && p.APIKey != "" {
			return p.APIKey, nil
		}
	}
	// Try "openrouter" provider directly
	if p, ok := cfg.Providers["openrouter"]; ok && p.APIKey != "" {
		return p.APIKey, nil
	}
	if cfg.APIKey != "" {
		return cfg.APIKey, nil
	}
	return "", fmt.Errorf("no API key found — set OPENROUTER_API_KEY or run: nqb config")
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

// VoyageAPIKey returns the Voyage AI API key from, in order:
//  1. VOYAGE_API_KEY environment variable
//  2. macOS Keychain (service name: VOYAGE_API_KEY)
//  3. ~/.naqb/config.yaml providers["voyage"].api_key
func VoyageAPIKey() string {
	if key := os.Getenv("VOYAGE_API_KEY"); key != "" {
		return key
	}
	if key := keychainGet("VOYAGE_API_KEY"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["voyage"]; ok && p.APIKey != "" {
		return p.APIKey
	}
	return ""
}

// ZillizAPIKey returns the Zilliz Cloud API key from, in order:
//  1. ZILLIZ_API_KEY environment variable
//  2. macOS Keychain (service name: ZILLIZ_API_KEY)
//  3. ~/.naqb/config.yaml providers["zilliz"].api_key
func ZillizAPIKey() string {
	if key := os.Getenv("ZILLIZ_API_KEY"); key != "" {
		return key
	}
	if key := keychainGet("ZILLIZ_API_KEY"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["zilliz"]; ok && p.APIKey != "" {
		return p.APIKey
	}
	return ""
}

// ZillizEndpoint returns the Zilliz Cloud cluster endpoint from, in order:
//  1. ZILLIZ_ENDPOINT environment variable
//  2. macOS Keychain (service name: ZILLIZ_ENDPOINT)
//  3. ~/.naqb/config.yaml providers["zilliz"].base_url
func ZillizEndpoint() string {
	if ep := os.Getenv("ZILLIZ_ENDPOINT"); ep != "" {
		return ep
	}
	if ep := keychainGet("ZILLIZ_ENDPOINT"); ep != "" {
		return ep
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["zilliz"]; ok && p.BaseURL != "" {
		return p.BaseURL
	}
	return ""
}

// AWSAccessKeyID returns the AWS access key ID from, in order:
//  1. AWS_ACCESS_KEY_ID environment variable
//  2. macOS Keychain (service name: AWS_ACCESS_KEY_ID)
//  3. ~/.naqb/config.yaml providers["bedrock"].api_key
func AWSAccessKeyID() string {
	if key := os.Getenv("AWS_ACCESS_KEY_ID"); key != "" {
		return key
	}
	if key := keychainGet("AWS_ACCESS_KEY_ID"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["bedrock"]; ok && p.APIKey != "" {
		return p.APIKey
	}
	return ""
}

// AWSSecretAccessKey returns the AWS secret access key from, in order:
//  1. AWS_SECRET_ACCESS_KEY environment variable
//  2. macOS Keychain (service name: AWS_SECRET_ACCESS_KEY)
//  3. ~/.naqb/config.yaml providers["bedrock"].secret_access_key
func AWSSecretAccessKey() string {
	if key := os.Getenv("AWS_SECRET_ACCESS_KEY"); key != "" {
		return key
	}
	if key := keychainGet("AWS_SECRET_ACCESS_KEY"); key != "" {
		return key
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return ""
	}
	if p, ok := cfg.Providers["bedrock"]; ok && p.SecretAccessKey != "" {
		return p.SecretAccessKey
	}
	return ""
}

// AWSRegion returns the AWS region from, in order:
//  1. AWS_REGION environment variable
//  2. macOS Keychain (service name: AWS_REGION)
//  3. ~/.naqb/config.yaml providers["bedrock"].region
//  4. Hardcoded default: eu-central-1
func AWSRegion() string {
	if r := os.Getenv("AWS_REGION"); r != "" {
		return r
	}
	if r := keychainGet("AWS_REGION"); r != "" {
		return r
	}
	cfg, err := LoadGlobal()
	if err == nil {
		if p, ok := cfg.Providers["bedrock"]; ok && p.Region != "" {
			return p.Region
		}
	}
	return "eu-central-1"
}

// ProviderConfigFor returns the ProviderConfig for a named provider, or the
// default OpenRouter config if the name is empty or not found.
func ProviderConfigFor(name string) (ProviderConfig, error) {
	cfg, err := LoadGlobal()
	if err != nil {
		return ProviderConfig{}, err
	}
	if name == "" {
		name = cfg.DefaultProvider
	}
	if name == "" {
		name = "openrouter" // system default
	}
	if name != "" {
		if p, ok := cfg.Providers[name]; ok {
			// Overlay Keychain values so plaintext keys in config.yaml are optional.
			if p.APIKey == "" {
				switch name {
				case "bedrock":
					p.APIKey = AWSAccessKeyID()
				case "openrouter":
					p.APIKey = keychainGet("OPENROUTER_API_KEY")
					if p.APIKey == "" {
						p.APIKey = os.Getenv("OPENROUTER_API_KEY")
					}
				}
			}
			if name == "bedrock" {
				if p.SecretAccessKey == "" {
					p.SecretAccessKey = AWSSecretAccessKey()
				}
				if p.Region == "" {
					p.Region = AWSRegion()
				}
			}
			return p, nil
		}
	}
	// Fall back: synthesise an openrouter config from the key lookup.
	key, err := APIKey()
	if err != nil {
		return ProviderConfig{}, err
	}
	return ProviderConfig{Type: "openrouter", APIKey: key}, nil
}
