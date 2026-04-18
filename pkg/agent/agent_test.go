package agent

import (
	"context"
	"testing"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/runtime"
)

// mockSessionStore records all calls for verification.
type mockSessionStore struct {
	created     []string
	appended    []mockMessage
	touched     []string
	listResults []runtime.SessionInfo
}

type mockMessage struct {
	msgID, sessionID, role, content, model string
	tokensIn, tokensOut                    int
}

func (m *mockSessionStore) CreateSession(ctx context.Context, sessionID, bookDir string, chapterNum int) error {
	m.created = append(m.created, sessionID)
	return nil
}

func (m *mockSessionStore) AppendMessage(ctx context.Context, msgID, sessionID, role, content, model string, tokensIn, tokensOut int) error {
	m.appended = append(m.appended, mockMessage{msgID, sessionID, role, content, model, tokensIn, tokensOut})
	return nil
}

func (m *mockSessionStore) TouchSession(ctx context.Context, sessionID string) error {
	m.touched = append(m.touched, sessionID)
	return nil
}

func (m *mockSessionStore) ListSessions(ctx context.Context, bookDir string, limit int) ([]runtime.SessionInfo, error) {
	return m.listResults, nil
}

// mockEpistemicStore returns a fixed summary.
type mockEpistemicStore struct {
	summary string
}

func (m *mockEpistemicStore) Load(ctx context.Context, bookID string) (runtime.EpistemicState, error) {
	return mockEpistemicState{summary: m.summary}, nil
}

type mockEpistemicState struct {
	summary string
}

func (m mockEpistemicState) Summary() string { return m.summary }

func TestAgent_WithSessionStore(t *testing.T) {
	store := &mockSessionStore{}
	agent := New(nil, "", "/books/test", nil, WithSessionStore(store))

	if agent.sessions != store {
		t.Error("expected sessions to be set")
	}
}

func TestAgent_WithEpistemicStore(t *testing.T) {
	store := &mockEpistemicStore{summary: "test"}
	agent := New(nil, "", "/books/test", nil, WithEpistemicStore(store))

	if agent.epistemic != store {
		t.Error("expected epistemic to be set")
	}
}

func TestAgent_Invoke_WithoutProvider(t *testing.T) {
	// When provider is nil, Run should fail early with a clear error
	agent := New(nil, "test-model", "/books/test", nil)
	_, err := agent.Invoke(context.Background(), "write chapter 1")
	if err == nil {
		t.Error("expected error when provider is nil")
	}
}

func TestBuildChapterTask_WithEpistemic(t *testing.T) {
	cfg := &config.BookConfig{
		Title:       "Test Book",
		TargetWords: 3000,
		Chapters:    []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	store := &mockEpistemicStore{summary: "## Epistemic Summary\nKey claim: X."}
	task := BuildChapterTask("/tmp/test", cfg, 1, store)

	if task == "" {
		t.Fatal("expected non-empty task")
	}
	if !contains(task, "Epistemic Summary") {
		t.Error("expected epistemic summary in task")
	}
}

func TestBuildChapterTask_WithoutEpistemic(t *testing.T) {
	cfg := &config.BookConfig{
		Title:       "Test Book",
		TargetWords: 3000,
		Chapters:    []config.Chapter{{Number: 1, Title: "Intro"}},
	}

	task := BuildChapterTask("/tmp/test", cfg, 1, nil)

	if task == "" {
		t.Fatal("expected non-empty task")
	}
	if contains(task, "Epistemic") {
		t.Error("did not expect epistemic summary when store is nil")
	}
}

func TestAnalyze_WithEpistemic(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.BookConfig{
		Title:    "Test",
		Chapters: []config.Chapter{},
	}
	store := &mockEpistemicStore{summary: "Claims: 3 established."}

	a := Analyze(dir, cfg, store)

	if a.EpistemicSummary != "Claims: 3 established." {
		t.Errorf("EpistemicSummary = %q, want %q", a.EpistemicSummary, "Claims: 3 established.")
	}
}

func TestAnalyze_WithoutEpistemic(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.BookConfig{
		Title:    "Test",
		Chapters: []config.Chapter{},
	}

	a := Analyze(dir, cfg, nil)

	if a.EpistemicSummary != "" {
		t.Errorf("EpistemicSummary = %q, want empty", a.EpistemicSummary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
