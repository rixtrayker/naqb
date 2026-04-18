package llm

// LLM token budget constants — used across providers and agents.
// Change here; all call sites update automatically.
const (
	// DefaultMaxTokens is the fallback max_tokens for any LLM call that does not
	// specify one explicitly.
	DefaultMaxTokens = 8192

	// MinTokensMiniMax is the minimum max_tokens for MiniMax reasoning models.
	// MiniMax consumes tokens on its internal reasoning trace before emitting
	// content; values below this threshold result in null content responses.
	MinTokensMiniMax = 512

	// MaxInputQA is the character cap applied to QA and conflict-check user
	// messages before sending to the LLM. Prevents hitting context-window limits.
	MaxInputQA = 60_000

	// MaxResearchCharsPerNote is the per-note character cap in context assembly
	// and the keyword search fallback.
	MaxResearchCharsPerNote = 2000

	// MaxCharsDepthLight is the chapter-content char cap for light analysis.
	MaxCharsDepthLight = 5_000
	// MaxCharsDepthModerate is the chapter-content char cap for moderate analysis.
	MaxCharsDepthModerate = 8_000
	// MaxCharsDepthMax is the chapter-content char cap for max-depth analysis.
	MaxCharsDepthMax = 12_000

	// DefaultTargetWordsPerChapter is used when the planner creates a new BookConfig
	// and the user has not specified a target.
	DefaultTargetWordsPerChapter = 3000

	// TokensPlan is the max_tokens budget for the planner LLM call.
	TokensPlan = 4096

	// TokensQA is the max_tokens budget for QA audit LLM calls.
	TokensQA = 2048

	// TokensAnalysis is the max_tokens budget for gap/conflict analysis LLM calls.
	TokensAnalysis = 1024
)
