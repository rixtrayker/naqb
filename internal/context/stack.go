// Package context provides the ContextStack and BraidedField system for
// multi-perspective analytical processing of scholarly Arabic texts.
package context

// ContextLayer is a single analytical lens within a ContextStack.
// Layers are numbered 0–5: 0 = foundational, 5 = most analytical.
type ContextLayer struct {
	// Position in the stack (0–5).
	Position int `yaml:"position"`
	// Name is the human-readable layer identifier.
	Name string `yaml:"name"`
	// Description is a ~100-token summary stored in the vector index.
	Description string `yaml:"description"`
	// Content is the full layer definition, loaded on demand.
	Content string `yaml:"content,omitempty"`
	// Language is the primary language of this layer ("ar", "en", etc.).
	Language string `yaml:"language,omitempty"`
}

// DebtPolicy describes what to do when a ContextStack exceeds token budget.
type DebtPolicy struct {
	// TokenBudget is the maximum tokens allowed for this stack (0 = unlimited).
	TokenBudget int `yaml:"token_budget,omitempty"`
	// OnExceed is the action: "fail", "degrade", "substitute", "human_gate".
	OnExceed string `yaml:"on_exceed,omitempty"`
}

// ContextStack is a named, ordered collection of analytical layers.
type ContextStack struct {
	// Name is the unique identifier for this stack.
	Name string `yaml:"name"`
	// Version tracks schema evolution.
	Version string `yaml:"version,omitempty"`
	// Layers are ordered analytical perspectives (position 0–5).
	Layers []ContextLayer `yaml:"layers"`
	// Debt controls resource budget for this stack.
	Debt DebtPolicy `yaml:"debt,omitempty"`
}

// LayerAt returns the layer at a given position, or nil if not found.
func (s *ContextStack) LayerAt(position int) *ContextLayer {
	for i := range s.Layers {
		if s.Layers[i].Position == position {
			return &s.Layers[i]
		}
	}
	return nil
}

// PromptContext assembles the stack's layers into a single prompt string,
// ordered by position. Only layers with non-empty Content are included.
func (s *ContextStack) PromptContext() string {
	var result string
	for _, layer := range s.Layers {
		if layer.Content == "" {
			continue
		}
		result += "### " + layer.Name + "\n"
		result += layer.Content + "\n\n"
	}
	return result
}
