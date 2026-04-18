package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/runtime"
)

// Event is emitted by RunDAG to report pipeline progress.
type Event struct {
	// StageID is the stage that produced this event.
	StageID string
	// Type is "start", "done", "error", "blocked".
	Type string
	// Output is the StageOutput (populated for "done" events).
	Output StageOutput
	// Err is set for "error" events.
	Err error
}

// RunDAG executes a DAG-ordered pipeline. Stages in the same topological batch
// run concurrently. Results from completed batches are available to subsequent
// batches via StageInput — the last successful StageOutput is merged into the
// next StageInput.
//
// emit is called for every event (may be nil for fire-and-forget).
func RunDAG(ctx context.Context, dag *DAG, input StageInput, emit func(Event)) error {
	if emit == nil {
		emit = func(Event) {}
	}

	// Load completed stages for resumption.
	completedStages := make(map[string]bool)
	if input.JobID != "" && input.DB != nil {
		var err error
		completedStages, err = dbCompletedStages(input.DB, input.JobID)
		if err != nil {
			return fmt.Errorf("run dag: load stage progress: %w", err)
		}
	}

	stageImpl := resolveStages(dag)

	graph := runtime.NewStateGraph[PipelineState]()
	for id, stage := range stageImpl {
		graph.AddNode(id, dagStageNode(id, stage, emit))
	}
	for id, decl := range dag.Stages() {
		for _, dep := range decl.DependsOn {
			graph.AddEdge(dep, id)
		}
	}

	state := PipelineState{
		BookDir:         input.BookDir,
		Cfg:             input.Cfg,
		Client:          input.Client,
		ChapterNum:      input.ChapterNum,
		Out:             input.Out,
		DB:              input.DB,
		FantasyProvider: input.FantasyProvider,
		FantasyModelID:  input.FantasyModelID,
		JobID:           input.JobID,
		GatesPassed:     input.GatesPassed,
		Completed:       completedStages,
	}

	compiled := graph.Compile()
	finalState, err := compiled.InvokeParallel(ctx, state)
	_ = finalState
	return err
}

func dagStageNode(id string, stage Stage, emit func(Event)) runtime.Node[PipelineState] {
	return func(ctx context.Context, state PipelineState, cfg *runtime.RunConfig) (PipelineState, error) {
		if state.Completed[id] {
			emit(Event{StageID: id, Type: "done", Output: StageOutput{StageName: id, Message: "already completed"}})
			return state, nil
		}

		// Human-in-the-loop gate check.
		if gate := stage.Gate(); gate != GateNone {
			if !containsString(state.GatesPassed, id) {
				if gate == GateAlways {
					return state, &runtime.InterruptedError{
						NodeID: id,
						Reason: fmt.Sprintf("human approval required for stage %s", id),
					}
				}
				if gate == GateAuto {
					for _, prev := range state.Stages {
						if strings.Contains(prev.Message, "issue") || strings.Contains(prev.Message, "gaps found") ||
							strings.Contains(prev.Message, "conflicts found") || strings.Contains(prev.Message, "⚠") {
							return state, &runtime.InterruptedError{
								NodeID: id,
								Reason: fmt.Sprintf("auto-gate triggered for %s due to previous warnings", id),
							}
						}
					}
				}
			}
		}

		emit(Event{StageID: id, Type: "start"})
		out, err := stage.Run(ctx, state.toInput())
		out.StageName = id
		if err != nil {
			emit(Event{StageID: id, Type: "error", Err: err})
			return state, fmt.Errorf("run dag: stage %q: %w", id, err)
		}
		emit(Event{StageID: id, Type: "done", Output: out})
		state.Stages = append(state.Stages, out)
		state.Completed[id] = true
		return state, nil
	}
}

// resolveStages maps stage IDs to built-in Stage implementations via the global registry.
func resolveStages(dag *DAG) map[string]Stage {
	impl := make(map[string]Stage, len(dag.Stages()))
	for id, decl := range dag.Stages() {
		stage, ok := ResolveStage(decl.Type)
		if !ok {
			impl[id] = &unknownStage{id: id, stageType: string(decl.Type)}
			continue
		}
		// Inject level config for conflict/gap stages via reflection on the cloned instance.
		// A type switch is used here because Stage is an interface; we set the level
		// before the stage runs.
		switch s := stage.(type) {
		case *ConflictStage:
			level := decl.Model
			if level == "" {
				level = "standard"
			}
			s.Level = level
			if decl.HumanGate != "" {
				s.HumanGate = decl.HumanGate
			}
		case *GapStage:
			level := decl.Model
			if level == "" {
				level = "standard"
			}
			s.Level = level
			if decl.HumanGate != "" {
				s.HumanGate = decl.HumanGate
			}
		}
		impl[id] = stage
	}
	return impl
}

// unknownStage returns an error when executed, flagging unrecognized stage types.
type unknownStage struct {
	id        string
	stageType string
}

func (s *unknownStage) Name() string              { return s.id }
func (s *unknownStage) CommitMessage(_ int) string { return "" }
func (s *unknownStage) Gate() GateType             { return GateNone }
func (s *unknownStage) Run(_ context.Context, _ StageInput) (StageOutput, error) {
	return StageOutput{}, fmt.Errorf("unknown stage type %q for stage %q", s.stageType, s.id)
}
