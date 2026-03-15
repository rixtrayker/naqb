// Package llm wraps LLM providers with streaming and non-streaming helpers.
package llm

// ── OpenRouter models (default provider) ─────────────────────────────────────

const (
	// ModelDefault is the model used when no stage-specific override is set.
	ModelDefault = ModelSonnet

	// ModelMiniMax is the MiniMax M2.5 model ID on OpenRouter.
	ModelMiniMax = "minimax/minimax-m2.5"

	// ModelHaiku — fast/cheap tier: classification, interview, style checks, research.
	// Maps to MiniMax M2.5 today (best value at this tier via OpenRouter).
	// Update this constant when a cheaper/faster model is available; all Haiku call sites benefit.
	ModelHaiku = "minimax/minimax-m2.5"

	// ModelSonnet — balanced tier: chapter writing, fix rewrites.
	// Maps to MiniMax M2.5 today (same model as Haiku — intentional).
	// The constant split is architectural: once a stronger model ships on OpenRouter,
	// only this line changes and writing quality improves across all stages automatically.
	ModelSonnet = "minimax/minimax-m2.5"

	// ModelOpus — deep reasoning tier: interactive chat, creative problem solving.
	ModelOpus = "anthropic/claude-opus-4-6"
)

// ── Model capability registry ────────────────────────────────────────────────

// SpeedTier classifies a model's relative latency characteristic.
type SpeedTier int

const (
	SpeedFast   SpeedTier = 1 // sub-2s typical
	SpeedMedium SpeedTier = 2 // 2-8s typical
	SpeedSlow   SpeedTier = 3 // 8s+ typical
)

// ModelCapabilities holds static metadata about a known model.
// Cost fields are in USD per million tokens.
// Used by the `nqb models` command and cost tracking in Wave 4.
type ModelCapabilities struct {
	// ID is the provider-qualified model string (e.g. "minimax/minimax-m2.5").
	ID string
	// InputCostPerMTok is cost in USD per million input tokens.
	InputCostPerMTok float64
	// OutputCostPerMTok is cost in USD per million output tokens.
	OutputCostPerMTok float64
	// ContextWindow is the maximum number of input tokens the model accepts.
	ContextWindow int
	// Speed is the model's relative latency tier.
	Speed SpeedTier
	// Reasoning indicates the model uses an internal chain-of-thought trace before
	// emitting content. These models require the MinTokensMiniMax floor.
	Reasoning bool
}

// KnownModels is the registry of models nqb knows about.
// Add entries when adding new providers; used by Wave 4 features.
var KnownModels = map[string]ModelCapabilities{
	"minimax/minimax-m2.5": {
		ID:                "minimax/minimax-m2.5",
		InputCostPerMTok:  0.20,
		OutputCostPerMTok: 1.10,
		ContextWindow:     1_000_000,
		Speed:             SpeedFast,
		Reasoning:         true,
	},
	"anthropic/claude-opus-4-6": {
		ID:                "anthropic/claude-opus-4-6",
		InputCostPerMTok:  15.00,
		OutputCostPerMTok: 75.00,
		ContextWindow:     200_000,
		Speed:             SpeedSlow,
		Reasoning:         false,
	},
	"anthropic/claude-sonnet-4-6": {
		ID:                "anthropic/claude-sonnet-4-6",
		InputCostPerMTok:  3.00,
		OutputCostPerMTok: 15.00,
		ContextWindow:     200_000,
		Speed:             SpeedMedium,
		Reasoning:         false,
	},
	"minimax.minimax-m2.1": {
		ID:                "minimax.minimax-m2.1",
		InputCostPerMTok:  0.20,
		OutputCostPerMTok: 1.10,
		ContextWindow:     1_000_000,
		Speed:             SpeedFast,
		Reasoning:         true,
	},
}

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
