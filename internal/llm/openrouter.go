package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/amr/naqb/internal/log"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1"

// OpenRouterProvider implements Provider using OpenRouter's OpenAI-compatible API.
type OpenRouterProvider struct {
	apiKey  string
	baseURL string
	http    *http.Client
	lastIn  int
	lastOut int
}

// NewOpenRouter creates a provider for OpenRouter.
// baseURL is optional — defaults to https://openrouter.ai/api/v1.
func NewOpenRouter(apiKey, baseURL string) *OpenRouterProvider {
	if baseURL == "" {
		baseURL = openRouterBaseURL
	}
	return &OpenRouterProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

// ── OpenAI-compat wire types ──────────────────────────────────────────────────

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orRequest struct {
	Model       string      `json:"model"`
	Messages    []orMessage `json:"messages"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
	Temperature float64     `json:"temperature,omitempty"`
}

type orChoice struct {
	Message struct {
		Content *string `json:"content"` // pointer: MiniMax reasoning models return null when truncated
	} `json:"message"`
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type orUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type orResponse struct {
	Choices []orChoice `json:"choices"`
	Usage   orUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

// Complete sends a non-streaming chat completion request.
func (p *OpenRouterProvider) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	// MiniMax reasoning models consume tokens on the reasoning trace before emitting content.
	// Enforce a minimum so content is never truncated.
	if maxTokens < MinTokensMiniMax {
		maxTokens = MinTokensMiniMax
	}
	log.Debug("LLM complete", "provider", "openrouter", "model", model, "max_tokens", maxTokens)

	msgs := p.buildMessages(system, messages)
	body, err := json.Marshal(orRequest{
		Model:     model,
		Messages:  msgs,
		MaxTokens: maxTokens,
		Stream:    false,
	})
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openrouter: build request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openrouter: reading response: %w", err)
	}

	var orResp orResponse
	if err := json.Unmarshal(data, &orResp); err != nil {
		return "", fmt.Errorf("openrouter: parsing response: %w (body: %s)", err, truncate(string(data), 200))
	}
	if orResp.Error != nil {
		return "", fmt.Errorf("openrouter API error %d: %s", orResp.Error.Code, orResp.Error.Message)
	}
	if len(orResp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: empty choices in response")
	}

	content := orResp.Choices[0].Message.Content
	if content == nil {
		return "", fmt.Errorf("openrouter: model returned null content (try increasing max_tokens)")
	}
	result := *content
	p.lastIn = orResp.Usage.PromptTokens
	p.lastOut = orResp.Usage.CompletionTokens
	log.Debug("LLM complete done", "provider", "openrouter", "model", model, "chars", len(result), "input_tokens", p.lastIn, "output_tokens", p.lastOut)
	return result, nil
}

// LastTokens implements TokenReporter — returns usage from the last Complete call.
func (p *OpenRouterProvider) LastTokens() (int, int) {
	return p.lastIn, p.lastOut
}

// Stream sends a streaming chat completion request, calling onDelta for each chunk.
func (p *OpenRouterProvider) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	log.Debug("LLM stream start", "provider", "openrouter", "model", model)

	msgs := p.buildMessages(system, messages)
	body, err := json.Marshal(orRequest{
		Model:     model,
		Messages:  msgs,
		MaxTokens: maxTokens,
		Stream:    true,
	})
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openrouter: build request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter: HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk orResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if chunk.Error != nil {
			return full.String(), fmt.Errorf("openrouter stream error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		full.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return full.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("openrouter: reading stream: %w", err)
	}

	result := full.String()
	log.Debug("LLM stream done", "provider", "openrouter", "model", model, "chars", len(result))
	return result, nil
}

func (p *OpenRouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/amr/naqb")
	req.Header.Set("X-Title", "nqb")
}

func (p *OpenRouterProvider) buildMessages(system string, messages []Message) []orMessage {
	var msgs []orMessage
	if system != "" {
		msgs = append(msgs, orMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		msgs = append(msgs, orMessage{Role: m.Role, Content: m.Content})
	}
	return msgs
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
