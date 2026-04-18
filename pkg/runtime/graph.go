package runtime

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Node is a unit of work in a StateGraph.
type Node[State any] func(ctx context.Context, state State, cfg *RunConfig) (State, error)

// StateGraph builds compiled directed acyclic graphs of nodes.
type StateGraph[State any] struct {
	nodes       map[string]Node[State]
	edges       map[string][]string // from -> []to
	deps        map[string][]string // to -> []from (computed)
	conditional map[string]func(State) string
	entry       string
}

// NewStateGraph creates an empty graph.
func NewStateGraph[State any]() *StateGraph[State] {
	return &StateGraph[State]{
		nodes:       make(map[string]Node[State]),
		edges:       make(map[string][]string),
		deps:        make(map[string][]string),
		conditional: make(map[string]func(State) string),
	}
}

// AddNode registers a named node.
func (g *StateGraph[State]) AddNode(name string, node Node[State]) {
	g.nodes[name] = node
}

// AddEdge creates a directed edge: after "from" finishes, execute "to".
func (g *StateGraph[State]) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
	g.deps[to] = append(g.deps[to], from)
}

// AddConditionalEdges registers a routing function for a node.
func (g *StateGraph[State]) AddConditionalEdges(from string, condition func(State) string) {
	g.conditional[from] = condition
}

// SetEntryPoint defines the starting node.
func (g *StateGraph[State]) SetEntryPoint(name string) {
	g.entry = name
}

// CompiledGraph is an executable state machine.
type CompiledGraph[State any] struct {
	graph        *StateGraph[State]
	checkpointer Checkpointer[State]
}

// Compile finalizes the graph for execution.
func (g *StateGraph[State]) Compile() *CompiledGraph[State] {
	return &CompiledGraph[State]{graph: g}
}

// SetCheckpointer attaches a checkpointer to the compiled graph.
func (c *CompiledGraph[State]) SetCheckpointer(cp Checkpointer[State]) {
	c.checkpointer = cp
}

// Invoke runs the graph from the entry point to completion.
func (c *CompiledGraph[State]) Invoke(ctx context.Context, state State, opts ...Option) (State, error) {
	cfg := &RunConfig{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.ThreadID != "" && c.checkpointer != nil && !cfg.SkipCheckpointLoad {
		if loaded, err := c.checkpointer.Get(ctx, cfg.ThreadID); err == nil {
			state = loaded
		}
	}

	current := c.graph.entry
	const maxSteps = 100
	for step := 0; current != ""; step++ {
		if step >= maxSteps {
			var zero State
			return zero, fmt.Errorf("graph: max steps (%d) exceeded — possible infinite loop at node %q", maxSteps, current)
		}

		node, ok := c.graph.nodes[current]
		if !ok {
			var zero State
			return zero, fmt.Errorf("graph: unknown node %q", current)
		}

		if cfg.Callbacks != nil {
			cfg.Callbacks.OnNodeStart(ctx, current, state)
		}

		nextState, err := node(ctx, state, cfg)
		if err != nil {
			if cfg.Callbacks != nil {
				cfg.Callbacks.OnNodeEnd(ctx, current, nextState, err)
			}
			var zero State
			return zero, fmt.Errorf("graph: node %q: %w", current, err)
		}
		state = nextState

		if cfg.Callbacks != nil {
			cfg.Callbacks.OnNodeEnd(ctx, current, state, nil)
		}

		if cfg.ThreadID != "" && c.checkpointer != nil {
			_ = c.checkpointer.Put(ctx, cfg.ThreadID, state)
		}

		if cond, ok := c.graph.conditional[current]; ok {
			current = cond(state)
		} else if nexts, ok := c.graph.edges[current]; ok && len(nexts) > 0 {
			current = nexts[0]
		} else {
			current = ""
		}
	}

	return state, nil
}

// InvokeParallel executes all reachable nodes in topological batches,
// running each batch concurrently. This is the pipeline/DAG execution model.
func (c *CompiledGraph[State]) InvokeParallel(ctx context.Context, state State, opts ...Option) (State, error) {
	cfg := &RunConfig{}
	for _, o := range opts {
		o(cfg)
	}

	batches, err := c.graph.topologicalBatches()
	if err != nil {
		var zero State
		return zero, fmt.Errorf("graph: %w", err)
	}

	for _, batch := range batches {
		if len(batch) == 1 {
			id := batch[0]
			if cfg.Callbacks != nil {
				cfg.Callbacks.OnNodeStart(ctx, id, state)
			}
			nextState, err := c.graph.nodes[id](ctx, state, cfg)
			if err != nil {
				if cfg.Callbacks != nil {
					cfg.Callbacks.OnNodeEnd(ctx, id, nextState, err)
				}
				var zero State
				return zero, fmt.Errorf("graph: node %q: %w", id, err)
			}
			state = nextState
			if cfg.Callbacks != nil {
				cfg.Callbacks.OnNodeEnd(ctx, id, state, nil)
			}
		} else {
			err := c.runBatch(ctx, batch, &state, cfg)
			if err != nil {
				var zero State
				return zero, err
			}
		}

		if cfg.ThreadID != "" && c.checkpointer != nil {
			_ = c.checkpointer.Put(ctx, cfg.ThreadID, state)
		}
	}

	return state, nil
}

func (c *CompiledGraph[State]) runBatch(ctx context.Context, batch []string, state *State, cfg *RunConfig) error {
	errs := make([]error, len(batch))
	states := make([]State, len(batch))
	var wg sync.WaitGroup

	for i, id := range batch {
		wg.Add(1)
		go func(idx int, nodeID string) {
			defer wg.Done()
			if cfg.Callbacks != nil {
				cfg.Callbacks.OnNodeStart(ctx, nodeID, *state)
			}
			s, err := c.graph.nodes[nodeID](ctx, *state, cfg)
			if err != nil {
				errs[idx] = err
				if cfg.Callbacks != nil {
					cfg.Callbacks.OnNodeEnd(ctx, nodeID, s, err)
				}
				return
			}
			states[idx] = s
			if cfg.Callbacks != nil {
				cfg.Callbacks.OnNodeEnd(ctx, nodeID, s, nil)
			}
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return fmt.Errorf("graph: node %q: %w", batch[i], err)
		}
	}
	// Merge states: for simplicity, use the last non-zero state.
	// In a full implementation, state would be a mergeable map.
	for _, s := range states {
		*state = s
	}
	return nil
}

func (g *StateGraph[State]) topologicalBatches() ([][]string, error) {
	inDegree := make(map[string]int, len(g.nodes))
	for id := range g.nodes {
		inDegree[id] = 0
	}
	for id := range g.nodes {
		inDegree[id] = len(g.deps[id])
	}

	var batches [][]string
	remaining := len(g.nodes)

	for remaining > 0 {
		var batch []string
		for id, deg := range inDegree {
			if deg == 0 {
				batch = append(batch, id)
			}
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("graph: cycle detected")
		}
		sort.Strings(batch)
		batches = append(batches, batch)

		for _, id := range batch {
			delete(inDegree, id)
			remaining--
		}

		for id := range inDegree {
			for _, dep := range g.deps[id] {
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
