package agents

import (
	"strings"

	"github.com/amr/naqb/pkg/llm"
)

// AnalysisLevel is the depth setting for gap analysis and conflict checks.
type AnalysisLevel string

const (
	LevelOff      AnalysisLevel = "off"
	LevelLight    AnalysisLevel = "light"
	LevelModerate AnalysisLevel = "moderate"
	LevelMax      AnalysisLevel = "max"
)

// CharsForLevel returns the maximum character budget for chapter content
// at a given analysis depth level. Shared by gap analysis and conflict check.
func CharsForLevel(level AnalysisLevel) int {
	switch level {
	case LevelLight:
		return llm.MaxCharsDepthLight
	case LevelMax:
		return llm.MaxCharsDepthMax
	default: // moderate or unknown → safe middle ground
		return llm.MaxCharsDepthModerate
	}
}

// DepthInstruction returns the analysis focus instruction for the LLM prompt.
// mode is "gap" or "conflict". Shared between the two analysis agents to
// eliminate the duplicate gapDepthInstruction / depthInstruction functions.
func DepthInstruction(level AnalysisLevel, mode string) string {
	switch mode {
	case "gap":
		switch level {
		case LevelLight:
			return "Focus only on clearly missing major topics from the outline."
		case LevelMax:
			return "Provide a thorough coverage audit: missing topics, shallow coverage, out-of-scope content, and suggestions."
		default:
			return "Identify missing topics and superficially-covered sections."
		}
	case "conflict":
		switch level {
		case LevelLight:
			return "Focus on obvious factual contradictions (names, dates, stated facts)."
		case LevelMax:
			return "Perform a deep review: facts, arguments, terminology consistency, narrative arc, and tone."
		default:
			return "Check factual accuracy, argument consistency, and narrative continuity."
		}
	}
	return ""
}

// parseVerdict extracts a VERDICT: YES/NO line from LLM output.
// The analysis prompts instruct the LLM to end with exactly this line.
// Falls back to heuristic matching if the model didn't follow the format.
func parseVerdict(findings string, heuristic func(string) bool) bool {
	lines := strings.Split(strings.TrimSpace(findings), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "VERDICT:") {
			return strings.Contains(strings.ToUpper(line), "YES")
		}
	}
	// Fallback: the model didn't include a verdict line.
	return heuristic(findings)
}
