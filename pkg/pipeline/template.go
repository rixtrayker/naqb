package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Template is a named DAG pipeline specification loaded from YAML.
type Template struct {
	// Name is the human-readable template identifier.
	Name string `yaml:"name"`
	// Description explains the template's purpose.
	Description string `yaml:"description,omitempty"`
	// Stages are the DAG stage declarations.
	Stages []StageDecl `yaml:"stages"`
}

// LoadTemplate loads a pipeline template by name.
// It searches (in order):
//  1. ~/.naqb/templates/<name>.yaml
//  2. <bookDir>/.naqb/templates/<name>.yaml (if bookDir provided)
//
// Returns an error if the template is not found or has an invalid DAG shape.
func LoadTemplate(name string, bookDir ...string) (*Template, error) {
	candidates := []string{}

	// Global templates dir
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".naqb", "templates", name+".yaml"))
	}

	// Per-project templates dir
	if len(bookDir) > 0 && bookDir[0] != "" {
		candidates = append(candidates, filepath.Join(bookDir[0], ".naqb", "templates", name+".yaml"))
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("template %q: read %s: %w", name, path, err)
		}
		return parseTemplate(name, data)
	}

	// Try built-in template name
	if tmpl, ok := builtinTemplates[name]; ok {
		return parseTemplate(name, []byte(tmpl))
	}

	return nil, fmt.Errorf("template %q not found (searched %v and built-ins)", name, candidates)
}

// parseTemplate parses YAML and validates the DAG.
func parseTemplate(name string, data []byte) (*Template, error) {
	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("template %q: parse YAML: %w", name, err)
	}
	if tmpl.Name == "" {
		tmpl.Name = name
	}

	// Validate DAG shape (no cycles, no unknown dependencies)
	if _, err := NewDAG(tmpl.Stages); err != nil {
		return nil, fmt.Errorf("template %q: invalid DAG: %w", name, err)
	}

	return &tmpl, nil
}

// ToDAG converts the template's stage declarations into a validated DAG.
func (t *Template) ToDAG() (*DAG, error) {
	return NewDAG(t.Stages)
}

// builtinTemplates maps template names to inline YAML definitions.
// These cover the core book-writing pipeline variants.
var builtinTemplates = map[string]string{
	"standard": `
name: standard
description: Standard chapter pipeline (context → write → qa)
stages:
  - id: context
    type: CONTEXT
    depends_on: []
  - id: write
    type: WRITE
    depends_on: [context]
  - id: qa
    type: QA
    depends_on: [write]
`,
	"thorough": `
name: thorough
description: Full chapter pipeline with conflict and gap checks
stages:
  - id: context
    type: CONTEXT
    depends_on: []
  - id: write
    type: WRITE
    depends_on: [context]
  - id: qa
    type: QA
    depends_on: [write]
  - id: conflict
    type: CONFLICT
    depends_on: [write]
  - id: gap
    type: GAP
    depends_on: [write]
`,
	"qa-only": `
name: qa-only
description: Run QA checks on an existing chapter draft
stages:
  - id: qa
    type: QA
    depends_on: []
  - id: conflict
    type: CONFLICT
    depends_on: []
  - id: gap
    type: GAP
    depends_on: []
`,
}
