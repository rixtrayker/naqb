package pipeline

import (
	"fmt"
)

// StageType classifies the role of a pipeline stage.
type StageType string

const (
	StageTypeContext    StageType = "CONTEXT"
	StageTypeWrite     StageType = "WRITE"
	StageTypeQA        StageType = "QA"
	StageTypeConflict  StageType = "CONFLICT"
	StageTypeGap       StageType = "GAP"
	StageTypeResearch  StageType = "RESEARCH"
	StageTypeSynthesize StageType = "SYNTHESIZE"
	StageTypeCustom    StageType = "CUSTOM"
)

// GateType specifies the human-review policy for a stage.
type GateType string

const (
	GateNone  GateType = "NONE"   // automatic, no gate
	GateAuto  GateType = "AUTO"   // gate only on failures/low confidence
	GateAlways GateType = "ALWAYS" // always require human approval
)

// StageDecl is a declarative stage descriptor loaded from a template YAML.
type StageDecl struct {
	// ID is the unique identifier within this DAG.
	ID string `yaml:"id"`
	// Type determines which built-in stage implementation to use.
	Type StageType `yaml:"type"`
	// DependsOn lists IDs of stages that must complete before this one starts.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// Model overrides the LLM model for this stage.
	Model string `yaml:"model,omitempty"`
	// HumanGate controls when this stage requires human review.
	HumanGate GateType `yaml:"human_gate,omitempty"`
	// Concurrency is the max parallel sub-tasks for this stage (0 = serial).
	Concurrency int `yaml:"concurrency,omitempty"`
}

// DAG is a directed acyclic graph of stage declarations.
type DAG struct {
	stages map[string]StageDecl
	edges  map[string][]string // stageID → depends_on IDs
}

// NewDAG builds and validates a DAG from a slice of StageDecl.
// Returns an error if there are duplicate IDs, unknown dependencies, or cycles.
func NewDAG(decls []StageDecl) (*DAG, error) {
	d := &DAG{
		stages: make(map[string]StageDecl, len(decls)),
		edges:  make(map[string][]string, len(decls)),
	}

	// Register stages
	for _, decl := range decls {
		if decl.ID == "" {
			return nil, fmt.Errorf("dag: stage declaration has empty ID")
		}
		if _, exists := d.stages[decl.ID]; exists {
			return nil, fmt.Errorf("dag: duplicate stage ID %q", decl.ID)
		}
		d.stages[decl.ID] = decl
		d.edges[decl.ID] = decl.DependsOn
	}

	// Validate dependencies exist
	for id, deps := range d.edges {
		for _, dep := range deps {
			if _, ok := d.stages[dep]; !ok {
				return nil, fmt.Errorf("dag: stage %q depends on unknown stage %q", id, dep)
			}
		}
	}

	// Validate no cycles via topological sort
	if _, err := d.TopologicalOrder(); err != nil {
		return nil, err
	}

	return d, nil
}

// TopologicalOrder returns the stages grouped into batches where all stages in
// a batch can execute in parallel (their dependencies are all in earlier batches).
func (d *DAG) TopologicalOrder() ([][]string, error) {
	// Compute in-degree for each node
	inDegree := make(map[string]int, len(d.stages))
	for id := range d.stages {
		inDegree[id] = 0
	}
	// inDegree[id] = number of dependencies this stage has
	for id := range inDegree {
		inDegree[id] = len(d.edges[id])
	}

	var batches [][]string
	remaining := len(d.stages)

	for remaining > 0 {
		// Collect all nodes with in-degree 0
		var batch []string
		for id, deg := range inDegree {
			if deg == 0 {
				batch = append(batch, id)
			}
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("dag: cycle detected in stage dependencies")
		}

		// Sort for determinism
		sortStrings(batch)
		batches = append(batches, batch)

		// Remove these nodes from the graph
		for _, id := range batch {
			delete(inDegree, id)
			remaining--
		}

		// Decrement in-degree for nodes that depended on the completed batch
		for id, deps := range d.edges {
			if _, stillPresent := inDegree[id]; !stillPresent {
				continue
			}
			for _, dep := range deps {
				for _, completed := range batch {
					if dep == completed {
						inDegree[id]--
					}
				}
			}
		}
	}

	return batches, nil
}

// Stages returns all stage declarations in the DAG.
func (d *DAG) Stages() map[string]StageDecl {
	return d.stages
}

// sortStrings sorts a string slice in place (simple insertion sort for small slices).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
