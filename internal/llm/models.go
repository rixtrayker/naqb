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

// ── AWS Bedrock model IDs (used when provider type = "bedrock") ───────────────
// Format: {provider}.{model-slug}  — dot-notation used by the Converse API and
// the bedrock-mantle OpenAI-compatible endpoint.
// Enable model access at: https://console.aws.amazon.com/bedrock/home#/modelaccess

const (
	// BedrockModelMiniMaxM2 is MiniMax M2 on AWS Bedrock (generally available).
	BedrockModelMiniMaxM2 = "minimax.minimax-m2"

	// BedrockModelMiniMaxM21 is MiniMax M2.1 on AWS Bedrock (added Feb 2026).
	// Improved reasoning, coding, and instruction following over M2.
	BedrockModelMiniMaxM21 = "minimax.minimax-m2.1"

	// BedrockModelMiniMaxM25 is MiniMax M2.5 on AWS Bedrock.
	// Verify availability in the Bedrock console before use — model access must be enabled.
	// If not yet listed, use BedrockModelMiniMaxM21 or route via OpenRouter instead.
	BedrockModelMiniMaxM25 = "minimax.minimax-m2.5"

	// ModelBedrockMiniMax is the recommended Bedrock MiniMax model.
	// Using M2.1 (confirmed GA Feb 2026) until M2.5 is verified available in the console.
	// Switch to BedrockModelMiniMaxM25 once confirmed.
	ModelBedrockMiniMax = BedrockModelMiniMaxM21
)
