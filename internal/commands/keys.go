package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/keycheck"
)

// KeysCmd returns the `nqb keys` command.
func KeysCmd() *cobra.Command {
	var setName string
	var toEnv bool
	var envFile string
	var testKeys bool

	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"k"},
		Short:   "Show API key status, test connectivity, or save a key",
		Long: `Display the status of all API keys used by nqb.

Shows which keys are configured, their source (env var, Keychain, or config
file), and whether they're needed for each command. Use --test to verify
live connectivity, or --set to save a new key to the macOS Keychain.`,
		Example: `  nqb keys
  nqb keys --test
  nqb keys --set ANTHROPIC_API_KEY
  nqb keys --set ZILLIZ_API_KEY --env
  nqb k`,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setName != "" {
				return keysSet(setName, toEnv, envFile)
			}
			if testKeys {
				return keysTest()
			}
			return keysList()
		},
	}

	cmd.Flags().StringVar(&setName, "set", "", "Key name to save (e.g. --set ZILLIZ_API_KEY)")
	cmd.Flags().BoolVar(&toEnv, "env", false, "Also write the key to a .env file")
	cmd.Flags().StringVar(&envFile, "env-file", ".env", "Path to the .env file (default: .env in current dir)")
	cmd.Flags().BoolVar(&testKeys, "test", false, "Test live connectivity for all set keys")
	return cmd
}

// keysList prints the key status table and per-command summary.
func keysList() error {
	statuses := keycheck.ResolveAll()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tSTATUS\tSOURCE")
	_, _ = fmt.Fprintln(w, strings.Repeat("─", 26)+"\t"+strings.Repeat("─", 22)+"\t"+strings.Repeat("─", 10))
	for _, k := range statuses {
		status := "MISSING"
		source := "—"
		if k.Set {
			status = fmt.Sprintf("SET (%s)", k.Masked)
			source = string(k.Source)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", k.Name, status, source)
	}
	_ = w.Flush()

	// Per-command summary if we can find a book root.
	bookDir, err := config.FindBookRoot()
	if err == nil && bookDir != "" {
		fmt.Println()
		printCommandRequirements()
	}
	return nil
}

// printCommandRequirements prints a one-line status for each LLM command.
func printCommandRequirements() {
	commands := []struct {
		name  string
		label string
	}{
		{"write", "write"},
		{"qa", "qa"},
		{"pipeline", "pipeline"},
		{"research", "research"},
		{"research-deep", "research --deep"},
		{"batch", "batch"},
	}

	fmt.Println("Required for current project:")
	for _, c := range commands {
		result := keycheck.CheckCommand(c.name)
		if result.OK {
			fmt.Printf("  %-18s ✓\n", c.label)
		} else {
			fmt.Printf("  %-18s ✗  (needs one of: %s)\n", c.label, strings.Join(result.Missing, ", "))
		}
	}
}

// keysTest runs a live connectivity probe for each set key.
func keysTest() error {
	statuses := keycheck.ResolveAll()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tRESULT")
	_, _ = fmt.Fprintln(w, strings.Repeat("─", 26)+"\t"+strings.Repeat("─", 40))

	for _, k := range statuses {
		if !k.Set {
			fmt.Fprintf(w, "%s\t%s\n", k.Name, "— not set, skipped")
			continue
		}
		result := probeKey(ctx, k.Name)
		fmt.Fprintf(w, "%s\t%s\n", k.Name, result)
	}
	_ = w.Flush()
	return nil
}

// probeKey runs a minimal live API call to verify a key works.
func probeKey(ctx context.Context, name string) string {
	switch name {
	case "OPENROUTER_API_KEY":
		return probeOpenRouter(ctx)
	case "ANTHROPIC_API_KEY":
		return probeAnthropic(ctx)
	case "GEMINI_API_KEY":
		return probeGemini(ctx)
	case "ZILLIZ_API_KEY":
		return probeZilliz(ctx)
	case "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY":
		return "— tested via Bedrock (run nqb write --chapter 1 --stream)"
	default:
		return "— no test available"
	}
}

func probeOpenRouter(ctx context.Context) string {
	key := keycheck.Resolve(keycheck.AllKeys[0]).Masked
	_ = key
	// GET /api/v1/models — lightweight, no tokens consumed.
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	resolvedKey := resolveRaw("OPENROUTER_API_KEY")
	req.Header.Set("Authorization", "Bearer "+resolvedKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "✗ " + err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 200 {
		return "✓ OK"
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return fmt.Sprintf("✗ HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func probeAnthropic(ctx context.Context) string {
	// Minimal message — 1 token, counts against quota but is negligible.
	payload := map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("x-api-key", resolveRaw("ANTHROPIC_API_KEY"))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "✗ " + err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == 200 {
		return "✓ OK"
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return fmt.Sprintf("✗ HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func probeGemini(ctx context.Context) string {
	key := resolveRaw("GEMINI_API_KEY")
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + key
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "✗ " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "✓ OK"
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return fmt.Sprintf("✗ HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func probeZilliz(ctx context.Context) string {
	endpoint := resolveRaw("ZILLIZ_ENDPOINT")
	key := resolveRaw("ZILLIZ_API_KEY")
	if endpoint == "" {
		return "✗ ZILLIZ_ENDPOINT not set"
	}
	url := strings.TrimRight(endpoint, "/") + "/v2/vectordb/collections/list"
	payload := []byte(`{}`)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "✗ " + err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if resp.StatusCode == 200 {
		// Parse code field from Zilliz response.
		var out struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(b, &out) == nil && out.Code == 0 {
			return "✓ OK"
		}
		return fmt.Sprintf("✓ HTTP 200 — %s", strings.TrimSpace(string(b)))
	}
	return fmt.Sprintf("✗ HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

// resolveRaw returns the raw (unmasked) value of a key by name.
// Checks env first, then Keychain.
func resolveRaw(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	for _, k := range keycheck.AllKeys {
		if k.Name == name {
			return keycheck.KeychainReadRaw(k.KeychainService)
		}
	}
	return ""
}

// keysSet prompts for a value and saves it to Keychain and optionally .env.
func keysSet(name string, toEnv bool, envFile string) error {
	var found bool
	for _, k := range keycheck.AllKeys {
		if k.Name == name {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(keycheck.AllKeys))
		for i, k := range keycheck.AllKeys {
			names[i] = k.Name
		}
		return fmt.Errorf("unknown key %q\nValid keys:\n  %s", name, strings.Join(names, "\n  "))
	}

	fmt.Printf("Enter value for %s: ", name)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("no input provided")
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	if err := keycheck.KeychainSet(name, value); err != nil {
		return fmt.Errorf("saving to Keychain: %w", err)
	}
	fmt.Printf("  ✓ %s saved to Keychain.\n", name)

	if toEnv {
		absEnvFile := envFile
		if !filepath.IsAbs(absEnvFile) {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting cwd: %w", err)
			}
			absEnvFile = filepath.Join(cwd, envFile)
		}
		if err := keycheck.EnvSet(absEnvFile, name, value); err != nil {
			return fmt.Errorf("saving to %s: %w", absEnvFile, err)
		}
		fmt.Printf("  ✓ %s written to %s\n", name, absEnvFile)
		ensureGitignore(absEnvFile)
	}
	return nil
}

// ensureGitignore prints a reminder if the .env file is not gitignored.
func ensureGitignore(envFile string) {
	dir := filepath.Dir(envFile)
	base := filepath.Base(envFile)
	giPath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(giPath)
	if err != nil {
		fmt.Printf("  ⚠  No .gitignore found — make sure %s is not committed.\n", base)
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == base || strings.TrimSpace(line) == "/"+base {
			return
		}
	}
	fmt.Printf("  ⚠  %s is not in .gitignore — add it to avoid committing secrets.\n", base)
}
