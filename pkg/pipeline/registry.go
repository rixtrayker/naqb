package pipeline

// StageFactory creates a new Stage instance.
type StageFactory func() Stage

// StageRegistry maps stage types to their constructors.
type StageRegistry struct {
	factories map[StageType]StageFactory
}

// NewStageRegistry creates an empty registry.
func NewStageRegistry() *StageRegistry {
	return &StageRegistry{factories: make(map[StageType]StageFactory)}
}

// Register adds a stage factory.
func (r *StageRegistry) Register(t StageType, f StageFactory) {
	r.factories[t] = f
}

// Resolve creates a Stage for the given type.
func (r *StageRegistry) Resolve(t StageType) (Stage, bool) {
	f, ok := r.factories[t]
	if !ok {
		return nil, false
	}
	return f(), true
}

var defaultRegistry = NewStageRegistry()

// RegisterStage registers a built-in stage globally.
func RegisterStage(t StageType, f StageFactory) {
	defaultRegistry.Register(t, f)
}

// ResolveStage looks up a built-in stage globally.
func ResolveStage(t StageType) (Stage, bool) {
	return defaultRegistry.Resolve(t)
}
