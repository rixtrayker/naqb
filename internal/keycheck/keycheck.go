// Package keycheck resolves API key availability and validates provider
// requirements before a command runs LLM work.
package keycheck

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Source describes where a key was found.
type Source string

const (
	SourceEnv      Source = "env"
	SourceKeychain Source = "keychain"
	SourceNone     Source = ""
)

// KeyStatus describes the resolved status of a single API key.
type KeyStatus struct {
	Name             string // canonical name (same as env var)
	EnvVar           string // environment variable name
	KeychainService  string // macOS Keychain service name
	Set              bool
	Source           Source
	Masked           string // first8...last4, or "****" if short
}

// AllKeys is the canonical list of keys known to nqb.
var AllKeys = []KeyStatus{
	{Name: "OPENROUTER_API_KEY", EnvVar: "OPENROUTER_API_KEY", KeychainService: "OPENROUTER_API_KEY"},
	{Name: "ANTHROPIC_API_KEY", EnvVar: "ANTHROPIC_API_KEY", KeychainService: "ANTHROPIC_API_KEY"},
	{Name: "GEMINI_API_KEY", EnvVar: "GEMINI_API_KEY", KeychainService: "GEMINI_API_KEY"},
	{Name: "VOYAGE_API_KEY", EnvVar: "VOYAGE_API_KEY", KeychainService: "VOYAGE_API_KEY"},
	{Name: "OPENAI_API_KEY", EnvVar: "OPENAI_API_KEY", KeychainService: "OPENAI_API_KEY"},
	{Name: "COMPOSIO_API_KEY", EnvVar: "COMPOSIO_API_KEY", KeychainService: "COMPOSIO_API_KEY"},
	{Name: "ZILLIZ_API_KEY", EnvVar: "ZILLIZ_API_KEY", KeychainService: "ZILLIZ_API_KEY"},
	{Name: "ZILLIZ_ENDPOINT", EnvVar: "ZILLIZ_ENDPOINT", KeychainService: "ZILLIZ_ENDPOINT"},
	{Name: "AWS_ACCESS_KEY_ID", EnvVar: "AWS_ACCESS_KEY_ID", KeychainService: "AWS_ACCESS_KEY_ID"},
	{Name: "AWS_SECRET_ACCESS_KEY", EnvVar: "AWS_SECRET_ACCESS_KEY", KeychainService: "AWS_SECRET_ACCESS_KEY"},
	{Name: "AWS_REGION", EnvVar: "AWS_REGION", KeychainService: "AWS_REGION"},
}

// Requirements maps a command name to the list of key names that satisfy it.
// At least one key from the list must be present for the command to proceed.
var Requirements = map[string][]string{
	"write":          {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"qa":             {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"pipeline":       {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"research":       {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"research-deep":  {"GEMINI_API_KEY"},
	"batch":          {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"chat":           {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"fix":            {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"init":           {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID"},
	"context":        {"OPENROUTER_API_KEY", "ANTHROPIC_API_KEY"},
	"index":          {"VOYAGE_API_KEY", "OPENAI_API_KEY"},
}

// PreflightResult is the outcome of CheckCommand.
type PreflightResult struct {
	Command string
	// Missing contains keys from the required set that were not found.
	Missing []string
	// Present contains keys from the required set that were found.
	Present []string
	// OK is true when at least one required key is present.
	OK bool
}

// Resolve resolves the key status for a single KeyStatus descriptor.
func Resolve(k KeyStatus) KeyStatus {
	if val := os.Getenv(k.EnvVar); val != "" {
		k.Set = true
		k.Source = SourceEnv
		k.Masked = maskKey(val)
		return k
	}
	if val := keychainGet(k.KeychainService); val != "" {
		k.Set = true
		k.Source = SourceKeychain
		k.Masked = maskKey(val)
		return k
	}
	k.Set = false
	k.Source = SourceNone
	k.Masked = ""
	return k
}

// ResolveAll resolves all known keys.
func ResolveAll() []KeyStatus {
	out := make([]KeyStatus, len(AllKeys))
	for i, k := range AllKeys {
		out[i] = Resolve(k)
	}
	return out
}

// CheckCommand returns the preflight result for the given command.
// result.OK is true when at least one of the required keys is present.
func CheckCommand(command string) PreflightResult {
	required, ok := Requirements[command]
	if !ok {
		// No requirement registered — OK by default.
		return PreflightResult{Command: command, OK: true}
	}

	// Build a fast lookup of resolved key names.
	resolved := make(map[string]bool)
	for _, k := range ResolveAll() {
		if k.Set {
			resolved[k.Name] = true
		}
	}

	var present, missing []string
	for _, name := range required {
		if resolved[name] {
			present = append(present, name)
		} else {
			missing = append(missing, name)
		}
	}

	return PreflightResult{
		Command: command,
		Missing: missing,
		Present: present,
		OK:      len(present) > 0,
	}
}

// EnvSet writes or updates KEY=value in the given .env file.
// Creates the file if absent. Updates the existing line if the key is already present.
// The file is kept in KEY=value format, one entry per line, no quoting.
func EnvSet(envFile, key, value string) error {
	// Read existing content (ok if file does not exist yet).
	existing, err := os.ReadFile(envFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("env: read %s: %w", envFile, err)
	}

	lines := strings.Split(string(existing), "\n")
	prefix := key + "="
	updated := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			updated = true
			break
		}
	}
	if !updated {
		// Strip a single trailing empty line before appending, then re-add it.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, prefix+value, "")
	}

	out := strings.Join(lines, "\n")
	return os.WriteFile(envFile, []byte(out), 0o600)
}

// KeychainSet saves value to the macOS Keychain under the given service name.
// Equivalent to: security add-generic-password -U -a $USER -s service -w value
func KeychainSet(service, value string) error {
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("keychain: USER environment variable not set")
	}
	cmd := exec.Command("security", "add-generic-password",
		"-U", "-a", user, "-s", service, "-w", value)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain set %q: %w — %s", service, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// maskKey returns a redacted representation of an API key.
// Keys longer than 12 chars show first-8…last-4; shorter keys show "****".
func maskKey(key string) string {
	if len(key) <= 12 {
		return "****"
	}
	return key[:8] + "…" + key[len(key)-4:]
}

// keychainGet retrieves a password from the macOS Keychain.
// Returns "" on any error (non-macOS, not found, etc.).
func keychainGet(service string) string {
	out, err := exec.Command("security", "find-generic-password",
		"-a", os.Getenv("USER"), "-s", service, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// KeychainReadRaw is the exported version of keychainGet for use by other packages.
func KeychainReadRaw(service string) string {
	return keychainGet(service)
}
