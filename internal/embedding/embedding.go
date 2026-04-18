// Package embedding provides a unified Embedder interface and implementations
// for OpenAI-compatible APIs (covering Voyage AI, Jina, Ollama), and stubs for
// AWS Bedrock. Vendored from WeKnora (Tencent, MIT) with additions.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Embedder is the interface implemented by all embedding backends.
type Embedder interface {
	// Embed encodes a batch of texts and returns one float32 vector per text.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions returns the output vector size for this model.
	Dimensions() int
}

// Config holds the configuration for creating an Embedder.
type Config struct {
	// Provider selects the backend: "openai", "voyage", "ollama", "bedrock".
	Provider string
	// APIKey for the embedding service.
	APIKey string
	// BaseURL overrides the default endpoint (useful for Ollama, proxies, Voyage AI).
	BaseURL string
	// ModelName is the model identifier.
	ModelName string
	// Dimensions is the output vector dimension (required for some backends).
	Dimensions int
}

// New creates an Embedder from a Config.
func New(cfg Config) (Embedder, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.ModelName, cfg.Dimensions), nil
	case "voyage":
		return NewVoyage(cfg.APIKey), nil
	case "ollama":
		base := cfg.BaseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		return NewOllama(base, cfg.ModelName, cfg.Dimensions), nil
	case "bedrock":
		return NewBedrock(), nil
	default:
		return nil, fmt.Errorf("embedding: unknown provider %q (valid: openai, voyage, ollama, bedrock)", cfg.Provider)
	}
}

// ── OpenAI-compatible embedder ────────────────────────────────────────────────

const (
	defaultOpenAIBase  = "https://api.openai.com/v1"
	defaultOpenAIModel = "text-embedding-3-small"
	defaultOpenAIDim   = 1536
)

// openAIEmbedder calls any OpenAI-compatible /embeddings endpoint.
type openAIEmbedder struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	client     *http.Client
}

// NewOpenAI creates an OpenAI-compatible embedder.
// Pass a custom baseURL to target Voyage AI, Jina, or similar services.
func NewOpenAI(apiKey, baseURL, model string, dimensions int) Embedder {
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	if model == "" {
		model = defaultOpenAIModel
	}
	if dimensions <= 0 {
		dimensions = defaultOpenAIDim
	}
	return &openAIEmbedder{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *openAIEmbedder) Dimensions() int { return e.dimensions }

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body := map[string]any{
		"model": e.model,
		"input": texts,
	}
	// Some APIs (Voyage) don't support 'dimensions' param.
	// Only include it when using the default OpenAI model.
	if e.dimensions > 0 && e.model == defaultOpenAIModel {
		body["dimensions"] = e.dimensions
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/embeddings", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("embedding: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding: API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embedding: decode response: %w", err)
	}

	vectors := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		vectors[i] = d.Embedding
	}
	return vectors, nil
}

// ── Voyage AI embedder ────────────────────────────────────────────────────────

const (
	voyageBaseURL = "https://api.voyageai.com/v1"
	voyageModel   = "voyage-3-large"
	voyageDim     = 1024
)

// NewVoyage creates a Voyage AI embedder using the voyage-3-large model.
// Voyage AI uses an OpenAI-compatible API with a custom endpoint.
func NewVoyage(apiKey string) Embedder {
	return NewOpenAI(apiKey, voyageBaseURL, voyageModel, voyageDim)
}

// ── Ollama embedder ───────────────────────────────────────────────────────────

// ollamaEmbedder calls a local Ollama /api/embeddings endpoint.
type ollamaEmbedder struct {
	baseURL    string
	model      string
	dimensions int
	client     *http.Client
}

// NewOllama creates an Ollama embedder for local models.
func NewOllama(baseURL, model string, dimensions int) Embedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if dimensions <= 0 {
		dimensions = 768 // sensible default for most Ollama models
	}
	return &ollamaEmbedder{
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

func (e *ollamaEmbedder) Dimensions() int { return e.dimensions }

func (e *ollamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := e.embedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vec)
	}
	return vectors, nil
}

func (e *ollamaEmbedder) embedOne(ctx context.Context, text string) ([]float32, error) {
	body := map[string]any{
		"model":  e.model,
		"prompt": text,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/api/embeddings", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("ollama embed: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: HTTP: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed: decode: %w", err)
	}
	return result.Embedding, nil
}

// ── AWS Bedrock embedder stub ─────────────────────────────────────────────────

// bedrockEmbedder is a stub — not yet implemented.
type bedrockEmbedder struct{}

// NewBedrock returns a stub Bedrock embedder. All calls return ErrNotImplemented.
func NewBedrock() Embedder { return &bedrockEmbedder{} }

func (e *bedrockEmbedder) Dimensions() int { return 1536 }

func (e *bedrockEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("bedrock embedder: not yet implemented — use openai or voyage")
}
