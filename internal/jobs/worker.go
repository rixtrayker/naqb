package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/amr/naqb/pkg/agent"
	"github.com/amr/naqb/pkg/booktools"
	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/internal/knowledge"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/pipeline"
	"github.com/amr/naqb/pkg/research"
	"github.com/amr/naqb/pkg/runtime"
)

// WorkerResult is the summary result of a worker run.
type WorkerResult struct {
	Processed int
	Failed    int
}

// Worker processes jobs from the queue using a pool of goroutines.
type Worker struct {
	queue       *Queue
	concurrency int
	drain       bool // if true, stop when queue is empty
	out         io.Writer
}

// NewWorker creates a Worker with the given concurrency.
// If drain is true, the worker stops when the queue empties (for batch runs).
// If drain is false, the worker polls continuously until ctx is cancelled.
func NewWorker(q *Queue, concurrency int, drain bool, out io.Writer) *Worker {
	if out == nil {
		out = os.Stdout
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Worker{
		queue:       q,
		concurrency: concurrency,
		drain:       drain,
		out:         out,
	}
}

// Run starts the worker pool and blocks until all workers exit.
// Workers exit when ctx is cancelled or (if drain=true) when the queue is empty.
func (w *Worker) Run(ctx context.Context) (*WorkerResult, error) {
	var (
		mu     sync.Mutex
		result WorkerResult
	)

	var wg sync.WaitGroup
	for i := range w.concurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				job, err := w.queue.Next(ctx)
				if err != nil {
					log.Error("worker: dequeue error", "worker", workerID, "err", err)
					time.Sleep(5 * time.Second)
					continue
				}
				if job == nil {
					if w.drain {
						return // queue empty, done
					}
					time.Sleep(2 * time.Second)
					continue
				}

				_, _ = fmt.Fprintf(w.out, "[worker-%d] job %s type=%s chapter=%d batch=%v\n",
					workerID, job.ID[:8], job.Type, job.ChapterNum, job.Batch)

				execErr := w.dispatch(ctx, job)
				if execErr != nil {
					log.Error("worker: job failed", "job", job.ID, "err", execErr)
					_ = w.queue.Fail(ctx, job.ID, execErr)
					mu.Lock()
					result.Failed++
					mu.Unlock()
					_, _ = fmt.Fprintf(w.out, "[worker-%d] ✗ job %s failed: %v\n", workerID, job.ID[:8], execErr)
					// Persist a notification for provider errors so the user
					// sees it even after the terminal is closed.
					if llm.IsProviderError(execErr) {
						n := Notification{
							JobID:      job.ID,
							JobType:    job.Type,
							BookDir:    job.BookDir,
							ChapterNum: job.ChapterNum,
							ErrorKind:  errorKind(execErr),
							Message:    execErr.Error(),
						}
						if writeErr := WriteNotification(n); writeErr != nil {
							log.Warn("worker: could not write notification", "err", writeErr)
						}
						_, _ = fmt.Fprintf(w.out, "\n[ALERT] Job %s failed: %s\n  Run `nqb keys` to check your API keys.\n\n",
							job.ID[:8], n.ErrorKind)
					}
				} else {
					// Clear any stale failure notifications for this job
					ClearNotificationsForJob(job.ID)
					mu.Lock()
					result.Processed++
					mu.Unlock()
					_, _ = fmt.Fprintf(w.out, "[worker-%d] ✓ job %s done\n", workerID, job.ID[:8])
				}
			}
		}(i + 1)
	}
	wg.Wait()
	return &result, nil
}

// dispatch routes a job to the correct handler and sets batch pricing if needed.
func (w *Worker) dispatch(ctx context.Context, job *db.Job) error {
	// Apply batch pricing for batch jobs
	if job.Batch {
		old := llm.ActivePricingTier
		llm.ActivePricingTier = llm.PricingBatch
		defer func() { llm.ActivePricingTier = old }()
	}

	switch JobType(job.Type) {
	case JobWrite:
		return w.runWriteJob(ctx, job)
	case JobQA:
		return w.runQAJob(ctx, job)
	case JobResearch:
		return w.runResearchJob(ctx, job)
	case JobPipeline:
		return w.runPipelineJob(ctx, job)
	default:
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

// ── Job handlers ──────────────────────────────────────────────────────────────

func (w *Worker) runWriteJob(ctx context.Context, job *db.Job) error {
	var payload WritePayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode write payload: %w", err)
	}
	chapterNum := payload.ChapterNum
	if chapterNum == 0 {
		chapterNum = job.ChapterNum
	}

	cfg, err := config.LoadBook(job.BookDir)
	if err != nil {
		return fmt.Errorf("load book config: %w", err)
	}

	// Build a fantasy provider from global config
	fantasyProvider, _, err := agent.NewProviderFromGlobalConfig()
	if err != nil {
		return fmt.Errorf("build provider: %w", err)
	}

	modelID := payload.Model
	if modelID == "" {
		modelID = cfg.LLM.WriteModel
	}
	if modelID == "" {
		modelID = llm.ModelDefault
	}

	tools := []runtime.Tool{
		booktools.NewReadFileTool(job.BookDir),
		booktools.NewWriteFileTool(job.BookDir),
		booktools.NewSearchResearchTool(job.BookDir),
		booktools.NewRunQATool(job.BookDir, cfg),
		booktools.NewWebFetchTool(),
		booktools.NewListChaptersTool(job.BookDir, cfg),
		booktools.NewKnowledgeSearchTool(job.BookDir),
		booktools.NewGrepChunksTool(job.BookDir),
	}
	a := agent.New(fantasyProvider, modelID, job.BookDir, cfg,
		agent.WithTools(tools),
		agent.WithSessionStore(db.NewSessionStore(w.queue.db)),
		agent.WithEpistemicStore(knowledge.NewEpistemicStore(w.queue.db)),
	)
	task := agent.BuildChapterTask(job.BookDir, cfg, chapterNum, nil)
	result, err := a.Run(ctx, task, "", nil)
	if err != nil {
		return fmt.Errorf("agent run: %w", err)
	}

	return w.queue.Complete(ctx, job.ID, map[string]any{
		"session_id": result.SessionID,
		"steps":      result.Steps,
		"tokens_in":  result.TokensIn,
		"tokens_out": result.TokensOut,
	})
}

func (w *Worker) runQAJob(ctx context.Context, job *db.Job) error {
	var payload QAPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode qa payload: %w", err)
	}
	chapterNum := payload.ChapterNum
	if chapterNum == 0 {
		chapterNum = job.ChapterNum
	}

	cfg, err := config.LoadBook(job.BookDir)
	if err != nil {
		return fmt.Errorf("load book config: %w", err)
	}

	client, err := buildLegacyProvider(cfg)
	if err != nil {
		return err
	}

	_, runErr := pipeline.RunChapterPipeline(ctx, client, job.BookDir, cfg, chapterNum, w.out)
	if runErr != nil {
		return runErr
	}
	return w.queue.Complete(ctx, job.ID, map[string]any{"chapter": chapterNum})
}

func (w *Worker) runResearchJob(ctx context.Context, job *db.Job) error {
	var payload ResearchPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode research payload: %w", err)
	}
	chapterNum := payload.ChapterNum
	if chapterNum == 0 {
		chapterNum = job.ChapterNum
	}

	cfg, err := config.LoadBook(job.BookDir)
	if err != nil {
		return fmt.Errorf("load book config: %w", err)
	}

	client, err := buildLegacyProvider(cfg)
	if err != nil {
		return err
	}

	rules, _ := config.LoadRules(job.BookDir)
	result, err := research.Run(ctx, client, job.BookDir, cfg, chapterNum, rules, w.out)
	if err != nil {
		return fmt.Errorf("research run: %w", err)
	}
	return w.queue.Complete(ctx, job.ID, map[string]any{
		"queries": result.Queries,
		"results": result.Results,
		"notes":   len(result.Notes),
	})
}

func (w *Worker) runPipelineJob(ctx context.Context, job *db.Job) error {
	var payload PipelinePayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("decode pipeline payload: %w", err)
	}
	chapterNum := payload.ChapterNum
	if chapterNum == 0 {
		chapterNum = job.ChapterNum
	}

	cfg, err := config.LoadBook(job.BookDir)
	if err != nil {
		return fmt.Errorf("load book config: %w", err)
	}

	client, err := buildLegacyProvider(cfg)
	if err != nil {
		return err
	}

	rules, _ := config.LoadRules(job.BookDir)
	result, err := pipeline.Run(ctx, pipeline.DefaultStagesFor(rules), pipeline.StageInput{
		BookDir:    job.BookDir,
		Cfg:        cfg,
		Client:     client,
		ChapterNum: chapterNum,
		Out:        w.out,
		DB:         w.queue.DB(),
		JobID:      job.ID,
	})
	if err != nil {
		return err
	}
	return w.queue.Complete(ctx, job.ID, map[string]any{
		"tokens_in":  result.TotalTokensIn(),
		"tokens_out": result.TotalTokensOut(),
		"cost":       result.TotalCost(),
	})
}

// buildLegacyProvider creates an llm.Provider from the book's config.
func buildLegacyProvider(cfg *config.BookConfig) (llm.Provider, error) {
	pcfg, err := config.ProviderConfigFor(cfg.LLM.WriteProvider)
	if err != nil {
		return nil, fmt.Errorf("load provider config: %w", err)
	}
	p, err := llm.NewProvider(pcfg)
	if err != nil {
		return nil, fmt.Errorf("build legacy provider: %w", err)
	}
	return p, nil
}
