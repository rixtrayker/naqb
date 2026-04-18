package runtime

import "context"

// CallbackHandler receives lifecycle events during graph and tool execution.
type CallbackHandler interface {
	OnNodeStart(ctx context.Context, nodeID string, state any)
	OnNodeEnd(ctx context.Context, nodeID string, state any, err error)
	OnToolStart(ctx context.Context, toolName string, input string)
	OnToolEnd(ctx context.Context, toolName string, output string, err error)
	OnLLMStart(ctx context.Context, model string, messages []any)
	OnLLMEnd(ctx context.Context, model string, output string, usage any)
}

// NoOpCallbackHandler is a callback handler that does nothing.
type NoOpCallbackHandler struct{}

func (NoOpCallbackHandler) OnNodeStart(ctx context.Context, nodeID string, state any) {}
func (NoOpCallbackHandler) OnNodeEnd(ctx context.Context, nodeID string, state any, err error) {
}
func (NoOpCallbackHandler) OnToolStart(ctx context.Context, toolName string, input string)  {}
func (NoOpCallbackHandler) OnToolEnd(ctx context.Context, toolName string, output string, err error) {
}
func (NoOpCallbackHandler) OnLLMStart(ctx context.Context, model string, messages []any)    {}
func (NoOpCallbackHandler) OnLLMEnd(ctx context.Context, model string, output string, usage any) {
}
