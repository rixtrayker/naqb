package booktools

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
	"github.com/amr/naqb/pkg/runtime"
)

// FantasyToolProvider is implemented by tools that expose a fantasy-native binding.
type FantasyToolProvider interface {
	FantasyTool() fantasy.AgentTool
}

// ToFantasy converts a slice of runtime.Tools to fantasy.AgentTools.
func ToFantasy(tools []runtime.Tool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, 0, len(tools))
	for _, t := range tools {
		if p, ok := t.(FantasyToolProvider); ok {
			out = append(out, p.FantasyTool())
		} else {
			out = append(out, adaptGeneric(t))
		}
	}
	return out
}

func adaptGeneric(tool runtime.Tool) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		tool.Name(),
		tool.Description(),
		func(ctx context.Context, input map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := tool.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}
