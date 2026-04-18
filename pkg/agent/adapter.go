package agent

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
	"github.com/amr/naqb/pkg/runtime"
)

// toFantasyTools converts runtime.Tools to fantasy.AgentTools.
// If a tool implements interface{ FantasyTool() fantasy.AgentTool }, that method is used.
func toFantasyTools(tools []runtime.Tool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, 0, len(tools))
	for _, t := range tools {
		if ftp, ok := t.(interface{ FantasyTool() fantasy.AgentTool }); ok {
			out = append(out, ftp.FantasyTool())
		} else {
			out = append(out, adaptGenericTool(t))
		}
	}
	return out
}

func adaptGenericTool(tool runtime.Tool) fantasy.AgentTool {
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
