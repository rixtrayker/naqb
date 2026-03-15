// Package llm wraps LLM providers with streaming and non-streaming helpers.
package llm

// ── OpenRouter models (default provider) ─────────────────────────────────────

const (
	// ModelDefault is the default model used across all stages unless overridden.
	ModelDefault = ModelMiniMax

	// MiniMax M2.5 — fast, capable, cost-efficient via OpenRouter.
	ModelMiniMax = "minimax/minimax-m2.5"

	// Aliases mapped to OpenRouter model IDs for drop-in substitution.
	ModelHaiku  = "minimax/minimax-m2.5"          // fast tasks (init interview, style check)
	ModelSonnet = "minimax/minimax-m2.5"          // chapter writing, QA
	ModelOpus   = "anthropic/claude-opus-4-5"     // deep reasoning tasks (chat, gap analysis)
)

// ── Anthropic native model IDs (used when provider type = "anthropic") ───────

const (
	ModelAnthropicHaiku  = "claude-haiku-4-5-20251001"
	ModelAnthropicSonnet = "claude-sonnet-4-6"
	ModelAnthropicOpus   = "claude-opus-4-6"
)
