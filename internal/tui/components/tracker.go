package components

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// TaskStatus represents the lifecycle of a background task.
type TaskStatus string

const (
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskFailed  TaskStatus = "failed"
)

// TrackedTask represents a background goroutine managed by the tracker.
type TrackedTask struct {
	ID         string
	Label      string
	Type       string // "write", "qa", "pipeline", "research"
	ChapterNum int
	Status     TaskStatus
	StartedAt  time.Time
	Result     string
	Error      error
}

// TaskCompleteMsg is sent to the Bubble Tea program when a background task finishes.
type TaskCompleteMsg struct {
	TaskID string
	Result string
	Error  error
}

// TaskTracker manages background goroutines and reports results via Bubble Tea messages.
type TaskTracker struct {
	mu      sync.Mutex
	tasks   map[string]*TrackedTask
	program *tea.Program
}

// NewTaskTracker creates a new TaskTracker.
func NewTaskTracker() *TaskTracker {
	return &TaskTracker{
		tasks: make(map[string]*TrackedTask),
	}
}

// SetProgram sets the Bubble Tea program for sending messages.
// Must be called after tea.NewProgram but before any Spawn calls.
func (t *TaskTracker) SetProgram(p *tea.Program) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.program = p
}

// Spawn launches a background goroutine and tracks it. Returns the task ID immediately.
// The function fn runs in a new goroutine; on completion, a TaskCompleteMsg is sent to the program.
func (t *TaskTracker) Spawn(label, taskType string, chapterNum int, fn func(ctx context.Context) (string, error)) string {
	id := uuid.NewString()[:8]

	task := &TrackedTask{
		ID:         id,
		Label:      label,
		Type:       taskType,
		ChapterNum: chapterNum,
		Status:     TaskRunning,
		StartedAt:  time.Now(),
	}

	t.mu.Lock()
	t.tasks[id] = task
	prog := t.program
	t.mu.Unlock()

	go func() {
		ctx := context.Background()
		result, err := fn(ctx)

		t.mu.Lock()
		if err != nil {
			task.Status = TaskFailed
			task.Error = err
		} else {
			task.Status = TaskDone
			task.Result = result
		}
		t.mu.Unlock()

		if prog != nil {
			prog.Send(TaskCompleteMsg{
				TaskID: id,
				Result: result,
				Error:  err,
			})
		}
	}()

	return id
}

// Active returns a copy of all tasks with status "running".
func (t *TaskTracker) Active() []TrackedTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []TrackedTask
	for _, task := range t.tasks {
		if task.Status == TaskRunning {
			out = append(out, *task)
		}
	}
	return out
}

// All returns a copy of all tracked tasks.
func (t *TaskTracker) All() []TrackedTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []TrackedTask
	for _, task := range t.tasks {
		out = append(out, *task)
	}
	return out
}

// Get returns a copy of a specific task by ID.
func (t *TaskTracker) Get(id string) (TrackedTask, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	task, ok := t.tasks[id]
	if !ok {
		return TrackedTask{}, false
	}
	return *task, true
}
