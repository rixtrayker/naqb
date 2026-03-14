// Package llm wraps the Anthropic SDK with streaming and non-streaming helpers.
package llm

// Model constants matching the plan's LLM assignments.
const (
	// ModelHaiku is the fast, cost-efficient model used for the init interview stage.
	ModelHaiku  = "claude-haiku-4-5-20251001"
	// ModelSonnet is the high-quality model used for chapter writing and QA stages.
	ModelSonnet = "claude-sonnet-4-6"
	// ModelOpus is the most capable model used for the interactive chat editing stage.
	ModelOpus   = "claude-opus-4-6"
)
