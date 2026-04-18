package runtime

// ToolRegistry is a mutable registry of tools.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Resolve looks up a tool by name.
func (r *ToolRegistry) Resolve(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}
