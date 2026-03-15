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

// PricingTier selects the pricing mode for a model call.
// Different tiers have different latency and cost characteristics.
type PricingTier int

const (
	// PricingStandard is the default: up to 200K context, standard rates.
	PricingStandard PricingTier = iota
	// PricingBatch gives 50% off for asynchronous (non-real-time) jobs via the Batches API.
	PricingBatch
	// PricingFast is 6× standard rates for latency-sensitive workloads.
	PricingFast
	// PricingLongContext applies when input > 200K tokens (premium rate for entire request).
	PricingLongContext
)

// ActivePricingTier is the process-wide default. Override per-call if needed.
// Defaults to PricingStandard; set to PricingBatch for bulk offline jobs.
var ActivePricingTier = PricingStandard

// ModelCosts holds all pricing tiers for one model.
// All values are USD per million tokens (MTok).
type ModelCosts struct {
	// Standard tier (default, up to 200K context)
	StandardIn  float64
	StandardOut float64
	// Batch tier (50% off, async via Batches API)
	BatchIn  float64
	BatchOut float64
	// Fast tier (6× standard, latency-sensitive)
	FastIn  float64
	FastOut float64
	// LongContext tier (input > 200K tokens; premium rate for entire request)
	LongContextIn  float64
	LongContextOut float64
}

// CostForTier returns the input and output costs (USD/MTok) for the given tier.
// Falls back to Standard if the tier has no specific pricing defined (zeros).
func (c ModelCosts) CostForTier(tier PricingTier) (in, out float64) {
	switch tier {
	case PricingBatch:
		if c.BatchIn > 0 {
			return c.BatchIn, c.BatchOut
		}
	case PricingFast:
		if c.FastIn > 0 {
			return c.FastIn, c.FastOut
		}
	case PricingLongContext:
		if c.LongContextIn > 0 {
			return c.LongContextIn, c.LongContextOut
		}
	}
	return c.StandardIn, c.StandardOut
}

// ModelCapabilities holds static metadata about a known model.
// Used by the `nqb models` command and cost tracking.
type ModelCapabilities struct {
	// ID is the provider-qualified model string (e.g. "minimax/minimax-m2.5").
	ID string
	// Costs holds all pricing tiers for this model.
	Costs ModelCosts
	// ContextWindow is the maximum number of input tokens the model accepts.
	ContextWindow int
	// Speed is the model's relative latency tier.
	Speed SpeedTier
	// Reasoning indicates the model uses an internal chain-of-thought trace before
	// emitting content. These models require the MinTokensMiniMax floor.
	Reasoning bool
}

// InputCostPerMTok returns the input cost for the active pricing tier.
// Kept for backward compatibility with budget.go and pipeline.go.
func (m ModelCapabilities) InputCostPerMTok() float64 {
	in, _ := m.Costs.CostForTier(ActivePricingTier)
	return in
}

// OutputCostPerMTok returns the output cost for the active pricing tier.
func (m ModelCapabilities) OutputCostPerMTok() float64 {
	_, out := m.Costs.CostForTier(ActivePricingTier)
	return out
}

// KnownModels is the registry of models nqb knows about.
// Pricing source: https://www.anthropic.com/pricing (Anthropic), OpenRouter model pages (others).
// Last verified: 2026-03-15.
var KnownModels = map[string]ModelCapabilities{
	"minimax/minimax-m2.5": {
		ID: "minimax/minimax-m2.5",
		Costs: ModelCosts{
			StandardIn:  0.20,
			StandardOut: 1.10,
		},
		ContextWindow: 1_000_000,
		Speed:         SpeedFast,
		Reasoning:     true,
	},
	// Claude Opus 4.6 — Anthropic direct API pricing:
	//   Standard:    $5.00 in / $25.00 out  (up to 200K context)
	//   LongContext: $10.00 in / $37.50 out (input > 200K, beta)
	//   Batch:       $2.50 in / $12.50 out  (50% off, async Batches API)
	//   Fast:        $30.00 in / $150.00 out (6× standard, latency-sensitive)
	"anthropic/claude-opus-4-6": {
		ID: "anthropic/claude-opus-4-6",
		Costs: ModelCosts{
			StandardIn:     5.00,
			StandardOut:    25.00,
			LongContextIn:  10.00,
			LongContextOut: 37.50,
			BatchIn:        2.50,
			BatchOut:       12.50,
			FastIn:         30.00,
			FastOut:        150.00,
		},
		ContextWindow: 200_000,
		Speed:         SpeedSlow,
	},
	// Claude Sonnet 4.6 — Anthropic direct API pricing:
	//   Standard:    $3.00 in / $15.00 out
	//   LongContext: $6.00 in / $22.50 out  (input > 200K, beta)
	//   Batch:       $1.50 in / $7.50 out
	//   Fast:        $18.00 in / $90.00 out
	"anthropic/claude-sonnet-4-6": {
		ID: "anthropic/claude-sonnet-4-6",
		Costs: ModelCosts{
			StandardIn:     3.00,
			StandardOut:    15.00,
			LongContextIn:  6.00,
			LongContextOut: 22.50,
			BatchIn:        1.50,
			BatchOut:       7.50,
			FastIn:         18.00,
			FastOut:        90.00,
		},
		ContextWindow: 200_000,
		Speed:         SpeedMedium,
	},
	"minimax.minimax-m2.1": {
		ID: "minimax.minimax-m2.1",
		Costs: ModelCosts{
			StandardIn:  0.20,
			StandardOut: 1.10,
		},
		ContextWindow: 1_000_000,
		Speed:         SpeedFast,
		Reasoning:     true,
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
