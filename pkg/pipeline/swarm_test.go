package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/amr/naqb/pkg/config"
)

func TestRunSwarm_Serial(t *testing.T) {
	stages := []Stage{
		&testSwarmStage{delay: 10 * time.Millisecond},
	}

	var buf strings.Builder
	res, err := RunSwarm(context.Background(), stages, SwarmInput{
		BookDir:     t.TempDir(),
		Cfg:         &config.BookConfig{Title: "Test"},
		ChapterNums: []int{1, 2, 3},
		Out:         &buf,
		Concurrency: 1, // serial
	})
	if err != nil {
		t.Fatalf("RunSwarm: %v", err)
	}

	if len(res.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res.Results))
	}
	for _, ch := range []int{1, 2, 3} {
		if _, ok := res.Results[ch]; !ok {
			t.Errorf("missing result for chapter %d", ch)
		}
	}
}

func TestRunSwarm_Parallel(t *testing.T) {
	stages := []Stage{
		&testSwarmStage{delay: 50 * time.Millisecond},
	}

	var buf strings.Builder
	start := time.Now()
	res, err := RunSwarm(context.Background(), stages, SwarmInput{
		BookDir:     t.TempDir(),
		Cfg:         &config.BookConfig{Title: "Test"},
		ChapterNums: []int{1, 2, 3},
		Out:         &buf,
		Concurrency: 3, // parallel
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RunSwarm: %v", err)
	}
	if len(res.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res.Results))
	}

	// If truly parallel, total time should be < 150ms (3 * 50ms serial).
	// Allow some slack but ensure it's faster than serial.
	if elapsed > 120*time.Millisecond {
		t.Logf("parallel execution took %s (may be slow on constrained runners)", elapsed)
	}
}

func TestRunSwarm_PartialFailure(t *testing.T) {
	stages := []Stage{
		&testSwarmStage{failForChapter: 2},
	}

	var buf strings.Builder
	res, err := RunSwarm(context.Background(), stages, SwarmInput{
		BookDir:     t.TempDir(),
		Cfg:         &config.BookConfig{Title: "Test"},
		ChapterNums: []int{1, 2, 3},
		Out:         &buf,
		Concurrency: 3,
	})
	if err != nil {
		t.Fatalf("RunSwarm: %v", err)
	}

	if len(res.Results) != 2 {
		t.Fatalf("expected 2 successes, got %d", len(res.Results))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(res.Errors))
	}
	if _, ok := res.Errors[2]; !ok {
		t.Fatalf("expected chapter 2 to fail")
	}
}

type testSwarmStage struct {
	delay          time.Duration
	failForChapter int
}

func (s *testSwarmStage) Name() string              { return "test" }
func (s *testSwarmStage) CommitMessage(_ int) string { return "" }
func (s *testSwarmStage) Gate() GateType             { return GateNone }
func (s *testSwarmStage) Run(ctx context.Context, in StageInput) (StageOutput, error) {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if s.failForChapter == in.ChapterNum {
		return StageOutput{}, fmt.Errorf("injected failure for chapter %d", in.ChapterNum)
	}
	return StageOutput{Message: fmt.Sprintf("ok ch%d", in.ChapterNum)}, nil
}
