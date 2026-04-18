// Package research implements the Scout → Explorer → Scribe automated research pipeline.
package research

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SearchResult is a single result returned by a search provider.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
	Body    string // full fetched body, if available
}

// Searcher is the interface for pluggable search backends.
type Searcher interface {
	// Search executes a query and returns up to maxResults results.
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
	// Name returns the provider name for logging.
	Name() string
}

// NewSearcher constructs a Searcher for the given provider name.
// Supported: "tavily", "exa", "gemini", "none" (returns empty results).
func NewSearcher(provider string) (Searcher, error) {
	switch strings.ToLower(provider) {
	case "tavily":
		key := os.Getenv("TAVILY_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("TAVILY_API_KEY not set")
		}
		return &TavilySearcher{apiKey: key}, nil
	case "exa":
		key := os.Getenv("EXA_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("EXA_API_KEY not set")
		}
		return &ExaSearcher{apiKey: key}, nil
	case "gemini":
		key := geminiKey()
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not set and not found in Keychain")
		}
		return &GeminiSearcher{apiKey: key}, nil
	case "", "none":
		return &NullSearcher{}, nil
	default:
		return nil, fmt.Errorf("unknown search provider %q — supported: tavily, exa, gemini, none", provider)
	}
}

// NewDeepSearcher returns a GeminiSearcher if a key is available, otherwise
// falls back to the configured provider. Used by --deep flag.
func NewDeepSearcher(fallbackProvider string) (Searcher, bool, error) {
	key := geminiKey()
	if key != "" {
		return &GeminiSearcher{apiKey: key}, true, nil
	}
	s, err := NewSearcher(fallbackProvider)
	return s, false, err
}

// geminiKey reads the Gemini API key from env or macOS Keychain.
// Avoids importing the config package to keep research self-contained.
func geminiKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	out, err := exec.Command("security", "find-generic-password",
		"-a", os.Getenv("USER"), "-s", "GEMINI_API_KEY", "-w").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ── NullSearcher ─────────────────────────────────────────────────────────────

// NullSearcher returns no results; used when search is disabled.
type NullSearcher struct{}

func (NullSearcher) Name() string { return "none" }
func (NullSearcher) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	return nil, nil
}

// ── TavilySearcher ───────────────────────────────────────────────────────────

// TavilySearcher calls the Tavily search API (general web).
type TavilySearcher struct {
	apiKey string
}

func (t *TavilySearcher) Name() string { return "tavily" }

func (t *TavilySearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	body := fmt.Sprintf(`{"api_key":%q,"query":%q,"max_results":%d,"include_answer":false}`,
		t.apiKey, query, maxResults)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search",
		strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily search: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("tavily parse: %w", err)
	}

	out := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		out = append(out, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}
	return out, nil
}

// ── ExaSearcher ──────────────────────────────────────────────────────────────

// ExaSearcher calls the Exa.ai neural search API (academic/Arabic-friendly).
type ExaSearcher struct {
	apiKey string
}

func (e *ExaSearcher) Name() string { return "exa" }

func (e *ExaSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	body := fmt.Sprintf(`{"query":%q,"numResults":%d,"useAutoprompt":true,"type":"neural","contents":{"text":{"maxCharacters":1000}}}`,
		query, maxResults)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.exa.ai/search",
		strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exa search: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Text  string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("exa parse: %w", err)
	}

	out := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		out = append(out, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Text,
		})
	}
	return out, nil
}

// ── GeminiSearcher ───────────────────────────────────────────────────────────

// GeminiSearcher uses the Gemini API with Google Search grounding to perform
// deep, citation-backed research. Each query gets a fresh Gemini call with
// search grounding enabled, returning synthesised content + source URLs.
type GeminiSearcher struct {
	apiKey string
}

func (g *GeminiSearcher) Name() string { return "gemini" }

func (g *GeminiSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// Gemini 2.0 Flash with Google Search grounding
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + g.apiKey

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": query},
				},
			},
		},
		"tools": []map[string]any{
			{"google_search": map[string]any{}},
		},
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": 2048,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini search: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini search HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			GroundingMetadata *struct {
				GroundingChunks []struct {
					Web *struct {
						URI   string `json:"uri"`
						Title string `json:"title"`
					} `json:"web"`
				} `json:"groundingChunks"`
			} `json:"groundingMetadata"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("gemini parse: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, nil
	}

	cand := result.Candidates[0]

	// Collect synthesised text
	var textParts []string
	for _, p := range cand.Content.Parts {
		if p.Text != "" {
			textParts = append(textParts, p.Text)
		}
	}
	synthesised := strings.Join(textParts, "\n")

	// Collect grounding sources
	var sources []SearchResult
	if cand.GroundingMetadata != nil {
		seen := map[string]bool{}
		for _, chunk := range cand.GroundingMetadata.GroundingChunks {
			if chunk.Web == nil || seen[chunk.Web.URI] {
				continue
			}
			seen[chunk.Web.URI] = true
			sources = append(sources, SearchResult{
				Title:   chunk.Web.Title,
				URL:     chunk.Web.URI,
				Snippet: "",
			})
			if len(sources) >= maxResults {
				break
			}
		}
	}

	// If we got sources, attach the synthesised text to the first result
	// and return individual source entries for the rest.
	if len(sources) > 0 {
		sources[0].Body = synthesised
		return sources, nil
	}

	// No grounding metadata — return the synthesis as a single result
	return []SearchResult{{
		Title:   query,
		URL:     "",
		Snippet: synthesised,
	}}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── FetchPage ────────────────────────────────────────────────────────────────

// FetchPage retrieves clean article text from a URL using the Jina Reader API
// (r.jina.ai), which strips boilerplate, ads, and navigation and returns
// plain Markdown. Falls back to raw HTML stripping if Jina is unreachable.
func FetchPage(ctx context.Context, rawURL string, maxBytes int) (string, error) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	jinaURL := "https://r.jina.ai/" + rawURL
	req, err := http.NewRequestWithContext(ctx, "GET", jinaURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nqb-research-bot/1.0")
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("X-Return-Format", "markdown")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Jina unreachable — fall back to direct fetch
		return fetchPageDirect(ctx, rawURL, maxBytes)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fetchPageDirect(ctx, rawURL, maxBytes)
	}

	limited := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// fetchPageDirect is the original raw-fetch fallback used when Jina is unavailable.
func fetchPageDirect(ctx context.Context, rawURL string, maxBytes int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nqb-research-bot/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return stripTags(string(body)), nil
}

func stripTags(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			out.WriteRune(' ')
		case !inTag:
			out.WriteRune(r)
		}
	}
	parts := strings.Fields(out.String())
	return strings.Join(parts, " ")
}
