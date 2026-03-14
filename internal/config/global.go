// Package config handles global (~/.naqb/config.yaml) and per-project (book.yaml) configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig holds ~/.config/book/config.yaml
type GlobalConfig struct {
	APIKey       string            `yaml:"api_key"`
	DefaultModel string            `yaml:"default_model,omitempty"`
	Editor       string            `yaml:"editor,omitempty"`
	Extra        map[string]string `yaml:"extra,omitempty"`
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "book", "config.yaml")
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

// APIKey returns the API key from global config or ANTHROPIC_API_KEY env var.
func APIKey() (string, error) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key, nil
	}
	cfg, err := LoadGlobal()
	if err != nil {
		return "", err
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("no API key found — set ANTHROPIC_API_KEY or run: book config")
	}
	return cfg.APIKey, nil
}
