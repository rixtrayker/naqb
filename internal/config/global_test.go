package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveGlobal_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create the config dir and an empty config file
	configDir := filepath.Join(dir, ".naqb")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &GlobalConfig{
		DefaultProvider: "openrouter",
		SetupVersion:    "0.5.0",
	}
	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal on empty file: %v", err)
	}

	// Round-trip: load it back
	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal after save: %v", err)
	}
	if loaded.DefaultProvider != "openrouter" {
		t.Errorf("DefaultProvider = %q, want openrouter", loaded.DefaultProvider)
	}
	if loaded.SetupVersion != "0.5.0" {
		t.Errorf("SetupVersion = %q, want 0.5.0", loaded.SetupVersion)
	}
}

func TestSaveGlobal_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	configDir := filepath.Join(dir, ".naqb")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Write garbage YAML
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("{{{{not yaml at all!"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &GlobalConfig{
		DefaultProvider: "anthropic",
		SetupVersion:    "0.5.0",
	}
	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal should recover from corrupt YAML: %v", err)
	}

	// Verify the saved values are readable
	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal after corrupt save: %v", err)
	}
	if loaded.DefaultProvider != "anthropic" {
		t.Errorf("DefaultProvider = %q, want anthropic", loaded.DefaultProvider)
	}
}

func TestNeedsSetup(t *testing.T) {
	cases := []struct {
		name string
		cfg  *GlobalConfig
		want bool
	}{
		{"nil config", nil, true},
		{"empty setup version", &GlobalConfig{}, true},
		{"older version", &GlobalConfig{SetupVersion: "0.4.0"}, true},
		{"current version", &GlobalConfig{SetupVersion: NaqbVersion}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsSetup(tc.cfg); got != tc.want {
				t.Errorf("NeedsSetup = %v, want %v", got, tc.want)
			}
		})
	}
}
