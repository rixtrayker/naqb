package agents

import (
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
)

// Stage identifies a pipeline stage for model/provider selection.
type Stage string

const (
	StageInit     Stage = "init"
	StageWrite    Stage = "write"
	StageQA       Stage = "qa"
	StageGap      Stage = "gap"
	StageConflict Stage = "conflict"
	StageFix      Stage = "fix"
	StageChat     Stage = "chat"
	StageResearch Stage = "research"
)

// StageDefaults maps each stage to its capability-based default model.
//
// Tier rationale:
//   - Haiku: cheap/fast — classification tasks, interview questions, style checks,
//     research queries. Output quality matters less than throughput and cost.
//   - Sonnet: balanced — chapter writing and fix rewrites where quality matters.
//   - Opus: deep reasoning — interactive chat where multi-turn nuance is critical.
var StageDefaults = map[Stage]string{
	StageInit:     llm.ModelHaiku,
	StageWrite:    llm.ModelSonnet,
	StageQA:       llm.ModelHaiku,
	StageGap:      llm.ModelHaiku,
	StageConflict: llm.ModelHaiku,
	StageFix:      llm.ModelSonnet,
	StageChat:     llm.ModelOpus,
	StageResearch: llm.ModelHaiku,
}

// ModelFor returns the model string to use for a stage.
// Resolution order:
//  1. Budget degradation (when session limit crossed, Write/Fix route to Haiku)
//  2. book.yaml LLMSettings override
//  3. StageDefaults
//
// cfg may be nil (used during init when no book config exists yet).
func ModelFor(stage Stage, cfg *config.BookConfig) string {
	// Budget auto-degrade: expensive write/fix stages fall back to fast/cheap model.
	if llm.SessionBudget.Degraded() {
		switch stage {
		case StageWrite, StageFix:
			return llm.ModelHaiku
		}
	}

	if cfg != nil {
		switch stage {
		case StageWrite:
			if cfg.LLM.WriteModel != "" {
				return cfg.LLM.WriteModel
			}
		case StageQA, StageGap, StageConflict:
			if cfg.LLM.QAModel != "" {
				return cfg.LLM.QAModel
			}
		case StageFix:
			if cfg.LLM.FixModel != "" {
				return cfg.LLM.FixModel
			}
			if cfg.LLM.WriteModel != "" {
				return cfg.LLM.WriteModel
			}
		case StageChat:
			if cfg.LLM.ChatModel != "" {
				return cfg.LLM.ChatModel
			}
		case StageInit:
			if cfg.LLM.InitModel != "" {
				return cfg.LLM.InitModel
			}
		}
	}
	if m, ok := StageDefaults[stage]; ok {
		return m
	}
	return llm.ModelDefault
}

// ProviderNameFor returns the named provider key (from GlobalConfig.Providers)
// to use for a stage. An empty string means "use the global default provider".
// cfg may be nil.
func ProviderNameFor(stage Stage, cfg *config.BookConfig) string {
	if cfg == nil {
		return ""
	}
	switch stage {
	case StageWrite:
		return cfg.LLM.WriteProvider
	case StageQA, StageGap, StageConflict:
		return cfg.LLM.QAProvider
	case StageFix:
		if cfg.LLM.FixProvider != "" {
			return cfg.LLM.FixProvider
		}
		return cfg.LLM.WriteProvider
	case StageChat:
		return cfg.LLM.ChatProvider
	case StageInit:
		return cfg.LLM.InitProvider
	}
	return ""
}
