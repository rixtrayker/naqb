package embedding

import (
	"context"
	"os"
	"testing"
)

func TestNew_UnknownProvider(t *testing.T) {
	_, err := New(Config{Provider: "invalid"})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestNew_ValidProviders(t *testing.T) {
	providers := []string{"openai", "voyage", "ollama", "bedrock"}
	for _, p := range providers {
		e, err := New(Config{Provider: p})
		if err != nil {
			t.Errorf("New(%q) returned error: %v", p, err)
		}
		if e == nil {
			t.Errorf("New(%q) returned nil embedder", p)
		}
	}
}

func TestOpenAI_Dimensions(t *testing.T) {
	e := NewOpenAI("key", "", "", 1024)
	if e.Dimensions() != 1024 {
		t.Errorf("expected 1024 dims, got %d", e.Dimensions())
	}
}

func TestOpenAI_DefaultDimensions(t *testing.T) {
	e := NewOpenAI("key", "", "", 0)
	if e.Dimensions() <= 0 {
		t.Error("expected positive default dimensions")
	}
}

func TestVoyage_Dimensions(t *testing.T) {
	e := NewVoyage("key")
	if e.Dimensions() != voyageDim {
		t.Errorf("expected %d dims for Voyage, got %d", voyageDim, e.Dimensions())
	}
}

func TestOllama_Dimensions(t *testing.T) {
	e := NewOllama("", "llama3", 512)
	if e.Dimensions() != 512 {
		t.Errorf("expected 512 dims, got %d", e.Dimensions())
	}
}

func TestBedrock_NotImplemented(t *testing.T) {
	e := NewBedrock()
	_, err := e.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("expected error from bedrock stub")
	}
}

func TestOpenAI_Embed_Integration(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	e := NewOpenAI(key, "", "", 0)
	vecs, err := e.Embed(context.Background(), []string{"hello world", "test embedding"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) == 0 {
			t.Errorf("vector %d is empty", i)
		}
	}
}
