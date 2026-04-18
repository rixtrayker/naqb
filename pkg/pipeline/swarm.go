package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"charm.land/fantasy"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
)

// SwarmInput configures a multi-chapter parallel pipeline run.
type SwarmInput struct {
	BookDir         string
	Cfg             *config.BookConfig
	Client          llm.Provider
	ChapterNums     []int
	Out             io.Writer
	DB              *sql.DB
	FantasyProvider fantasy.Provider
	FantasyModelID  string
	JobID           string
	// Concurrency limits the number of chapters processed in parallel.
	// 0 or negative means no limit.
	Concurrency int
}

// SwarmResult aggregates per-chapter pipeline results.
type SwarmResult struct {
	Results map[int]*PipelineResult
	Errors  map[int]error
}

// RunSwarm runs the same stage list for multiple chapters in parallel,
// optionally checkpointing each chapter under a sub-job ID.
func RunSwarm(ctx context.Context, stages []Stage, in SwarmInput) (*SwarmResult, error) {
	if len(in.ChapterNums) == 0 {
		return &SwarmResult{}, nil
	}

	concurrency := in.Concurrency
	if concurrency <= 0 {
		concurrency = len(in.ChapterNums)
	}

	log.Info("swarm start", "chapters", len(in.ChapterNums), "concurrency", concurrency, "book", in.Cfg.Title)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	res := &SwarmResult{
		Results: make(map[int]*PipelineResult, len(in.ChapterNums)),
		Errors:  make(map[int]error, len(in.ChapterNums)),
	}
	var mu sync.Mutex

	for _, ch := range in.ChapterNums {
		ch := ch // capture loop var
		g.Go(func() error {
			var buf strings.Builder
			w := io.MultiWriter(in.Out, &buf)

			jobID := in.JobID
			if jobID != "" {
				jobID = fmt.Sprintf("%s-ch%d", in.JobID, ch)
			}

			result, err := Run(ctx, stages, StageInput{
				BookDir:         in.BookDir,
				Cfg:             in.Cfg,
				Client:          in.Client,
				ChapterNum:      ch,
				Out:             w,
				DB:              in.DB,
				FantasyProvider: in.FantasyProvider,
				FantasyModelID:  in.FantasyModelID,
				JobID:           jobID,
			})

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.Errors[ch] = err
				fmt.Fprintf(in.Out, "  Chapter %d failed: %v\n", ch, err)
			} else {
				res.Results[ch] = result
			}
			return nil // errgroup collects no global error; we track per-chapter
		})
	}

	_ = g.Wait() // we already store per-chapter errors in res.Errors

	log.Info("swarm complete", "succeeded", len(res.Results), "failed", len(res.Errors))
	return res, nil
}

// RunSwarmWithRules is a convenience wrapper that loads rules and uses DefaultStagesFor.
func RunSwarmWithRules(ctx context.Context, in SwarmInput) (*SwarmResult, error) {
	rules, _ := config.LoadRules(in.BookDir)
	return RunSwarm(ctx, DefaultStagesFor(rules), in)
}
