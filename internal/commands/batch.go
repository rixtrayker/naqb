package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/internal/db"
	"github.com/amr/naqb/internal/jobs"
)

// BatchCmd returns the `nqb batch` command group.
func BatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "batch",
		Aliases: []string{"b"},
		Short:   "Manage the background job queue",
		Long: `Manage the background job queue for bulk chapter processing.

Use 'nqb batch enqueue' to add chapters to the queue, 'nqb batch run'
to start the worker, and 'nqb batch status' to monitor progress.`,
		GroupID: "management",
	}
	cmd.AddCommand(
		batchEnqueueCmd(),
		batchStatusCmd(),
		batchRunCmd(),
		batchCancelCmd(),
	)
	return cmd
}

// batchEnqueueCmd is `nqb batch enqueue`.
func batchEnqueueCmd() *cobra.Command {
	var chapterNum int
	var all bool
	var jobType string
	var batch bool
	var priority int

	cmd := &cobra.Command{
		Use:     "enqueue",
		Aliases: []string{"add", "eq"},
		Short:   "Add one or more chapters to the job queue",
		Long: `Enqueue chapters for background processing.

Job types:
  write     — write chapter draft via agent loop
  qa        — run QA stage only
  research  — run research pipeline
  pipeline  — run full pipeline (context + write + qa)

Use --batch to request batch pricing (50% discount on supported providers).
Use --priority to control processing order (higher = processed first).`,
		Example: `  nqb batch enqueue --chapter 3
  nqb batch enqueue --chapter 3 --type write --batch
  nqb batch enqueue --all --type pipeline
  nqb batch add -c 5 -t qa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			bookDir, err := config.FindBookRoot()
			if err != nil {
				return err
			}
			cfg, err := config.LoadBook(bookDir)
			if err != nil {
				return err
			}

			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			q := jobs.New(sqlDB)
			ctx := context.Background()

			jt := jobs.JobType(jobType)
			if jt == "" {
				jt = jobs.JobPipeline
			}

			// Validate job type
			switch jt {
			case jobs.JobWrite, jobs.JobQA, jobs.JobResearch, jobs.JobPipeline:
				// valid
			default:
				return fmt.Errorf("unknown job type %q; use: write, qa, research, pipeline", jobType)
			}

			chapters := []int{chapterNum}
			if all {
				chapters = nil
				for _, ch := range cfg.Chapters {
					chapters = append(chapters, ch.Number)
				}
			} else if chapterNum <= 0 {
				return fmt.Errorf("specify --chapter N or --all")
			}

			var failures int
			for _, n := range chapters {
				payload := buildPayload(jt, n)
				id, err := q.Enqueue(ctx, jt, bookDir, n, payload, batch, priority)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  enqueue chapter %d: %v\n", n, err)
					failures++
					continue
				}
				batchStr := ""
				if batch {
					batchStr = " (batch pricing)"
				}
				fmt.Printf("  ✓ enqueued %s job for chapter %d → %s%s\n", jt, n, id[:8], batchStr)
			}
			if failures > 0 {
				return fmt.Errorf("%d of %d chapters failed to enqueue", failures, len(chapters))
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&chapterNum, "chapter", "c", 0, "Chapter number")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Enqueue all chapters")
	cmd.Flags().StringVarP(&jobType, "type", "t", "pipeline", "Job type: write, qa, research, pipeline")
	cmd.Flags().BoolVar(&batch, "batch", false, "Mark job for batch pricing (50% cost discount where supported)")
	cmd.Flags().IntVar(&priority, "priority", 0, "Job priority (higher = processed first)")
	return cmd
}

// batchStatusCmd is `nqb batch status`.
func batchStatusCmd() *cobra.Command {
	var jobID string
	var showAll bool
	var limit int
	var clearAlerts bool

	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"st", "ls"},
		Short:   "Show job queue status",
		Long: `Display the current state of the job queue.

Shows pending, running, completed, and failed jobs with timing info.
Use --all to include completed/failed jobs. Use --job-id to inspect
a specific job in detail. Provider-error alerts are shown at the top
when present.`,
		Example: `  nqb batch status
  nqb batch status --all
  nqb batch status --job-id abc12345
  nqb batch st --clear-alerts`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show and optionally clear provider-error notifications.
			notifications, _ := jobs.ReadNotifications()
			if len(notifications) > 0 {
				fmt.Printf("\n  ⚠ ALERTS (%d provider error(s) detected)\n", len(notifications))
				fmt.Printf("  %s\n", repeat("─", 60))
				for _, n := range notifications {
					fmt.Printf("  Job %-10s  type=%-10s  ch=%-3d  kind=%s\n",
						n.JobID[:min(8, len(n.JobID))], n.JobType, n.ChapterNum, n.ErrorKind)
					fmt.Printf("    %s\n", n.Message)
				}
				fmt.Printf("  %s\n", repeat("─", 60))
				fmt.Println("  Run `nqb keys` to check your API keys.")
				fmt.Println("  Run `nqb batch status --clear-alerts` to dismiss.")
				fmt.Println()
				if clearAlerts {
					if err := jobs.ClearNotifications(); err != nil {
						fmt.Fprintf(os.Stderr, "  clear alerts: %v\n", err)
					} else {
						fmt.Println("  ✓ Alerts cleared.")
					}
					return nil
				}
			} else if clearAlerts {
				fmt.Println("  No alerts to clear.")
				return nil
			}

			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			if jobID != "" {
				job, err := db.GetJob(sqlDB, jobID)
				if err != nil {
					return err
				}
				printJobDetail(job)
				return nil
			}

			statusFilter := "pending"
			if showAll {
				statusFilter = ""
			}
			jobList, err := db.ListJobs(sqlDB, statusFilter, limit)
			if err != nil {
				return err
			}

			if len(jobList) == 0 {
				if showAll {
					fmt.Println("  No jobs in queue.")
				} else {
					fmt.Println("  No pending jobs. Use --all to see all jobs.")
				}
				return nil
			}

			fmt.Printf("  %-10s  %-8s  %-6s  %-10s  %-20s  %s\n",
				"ID", "Type", "Ch", "Status", "Created", "Info")
			fmt.Printf("  %s\n", repeat("─", 75))
			for _, j := range jobList {
				info := j.Error
				if info == "" && j.Result != "" {
					info = "done"
				}
				batchStr := ""
				if j.Batch {
					batchStr = " [batch]"
				}
				fmt.Printf("  %-10s  %-8s  %-6d  %-10s  %-20s  %s%s\n",
					j.ID[:8], j.Type, j.ChapterNum, j.Status,
					humanize.Time(j.CreatedAt), info, batchStr)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&jobID, "job-id", "", "Show details for a specific job ID")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all jobs (not just pending)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of jobs to display")
	cmd.Flags().BoolVar(&clearAlerts, "clear-alerts", false, "Dismiss all provider-error alert notifications")
	return cmd
}

// batchRunCmd is `nqb batch run`.
func batchRunCmd() *cobra.Command {
	var concurrency int
	var drain bool

	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"start"},
		Short:   "Start the background job worker",
		Long: `Start the background worker that processes queued jobs.

Use --drain to process all pending jobs and exit when the queue is empty.
Without --drain the worker runs continuously until interrupted (Ctrl+C).
Use --workers to increase concurrency for faster parallel processing.`,
		Example: `  nqb batch run
  nqb batch run --drain
  nqb batch run --workers 3
  nqb batch start --drain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := RunPreflight("batch"); err != nil {
				return err
			}

			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			q := jobs.New(sqlDB)
			w := jobs.NewWorker(q, concurrency, drain, os.Stdout)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle interrupt — gracefully cancel worker on Ctrl+C
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			go func() {
				<-sigCh
				fmt.Fprintln(os.Stderr, "\nInterrupt received, shutting down workers...")
				cancel()
			}()

			fmt.Printf("  Starting worker (concurrency=%d, drain=%v)...\n", concurrency, drain)
			result, err := w.Run(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("\n  Worker done: %d processed, %d failed\n", result.Processed, result.Failed)
			return nil
		},
	}

	cmd.Flags().IntVar(&concurrency, "workers", 1, "Number of concurrent worker goroutines")
	cmd.Flags().BoolVar(&drain, "drain", false, "Exit when queue is empty instead of polling indefinitely")
	return cmd
}

// batchCancelCmd is `nqb batch cancel`.
func batchCancelCmd() *cobra.Command {
	var jobID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a pending or running job",
		Long: `Cancel a job by its ID. Only pending or running jobs can be cancelled.
The job ID is shown by 'nqb batch status' (first 8 characters suffice).`,
		Example: `  nqb batch cancel --job-id abc12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if jobID == "" {
				return fmt.Errorf("--job-id is required")
			}
			dbPath, err := db.DefaultPath()
			if err != nil {
				return err
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = sqlDB.Close() }()

			q := jobs.New(sqlDB)
			if err := q.Cancel(context.Background(), jobID); err != nil {
				return err
			}
			fmt.Printf("  ✓ job %s cancelled\n", jobID[:min(8, len(jobID))])
			return nil
		},
	}
	cmd.Flags().StringVar(&jobID, "job-id", "", "Job ID to cancel")
	_ = cmd.MarkFlagRequired("job-id")
	return cmd
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildPayload(jt jobs.JobType, chapterNum int) any {
	switch jt {
	case jobs.JobWrite:
		return jobs.WritePayload{ChapterNum: chapterNum}
	case jobs.JobQA:
		return jobs.QAPayload{ChapterNum: chapterNum}
	case jobs.JobResearch:
		return jobs.ResearchPayload{ChapterNum: chapterNum}
	default:
		return jobs.PipelinePayload{ChapterNum: chapterNum}
	}
}

func printJobDetail(j *db.Job) {
	fmt.Printf("  ID:          %s\n", j.ID)
	fmt.Printf("  Type:        %s\n", j.Type)
	fmt.Printf("  Book:        %s\n", j.BookDir)
	fmt.Printf("  Chapter:     %d\n", j.ChapterNum)
	fmt.Printf("  Status:      %s\n", j.Status)
	fmt.Printf("  Batch:       %v\n", j.Batch)
	fmt.Printf("  Priority:    %d\n", j.Priority)
	fmt.Printf("  Attempt:     %d / %d\n", j.Attempt, j.MaxAttempts)
	fmt.Printf("  Created:     %s (%s)\n", j.CreatedAt.Format("2006-01-02 15:04:05"), humanize.Time(j.CreatedAt))
	if j.StartedAt != nil {
		fmt.Printf("  Started:     %s\n", j.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if j.FinishedAt != nil {
		fmt.Printf("  Finished:    %s\n", j.FinishedAt.Format("2006-01-02 15:04:05"))
		if j.StartedAt != nil {
			dur := j.FinishedAt.Sub(*j.StartedAt)
			fmt.Printf("  Duration:    %s\n", humanize.RelTime(*j.StartedAt, *j.FinishedAt, "", ""))
			_ = dur
		}
	}
	if j.Payload != "" && j.Payload != "{}" {
		fmt.Printf("  Payload:     %s\n", j.Payload)
	}
	if j.Result != "" {
		fmt.Printf("  Result:      %s\n", j.Result)
	}
	if j.Error != "" {
		fmt.Printf("  Error:       %s\n", j.Error)
	}
}

func repeat(s string, n int) string {
	result := ""
	for range n {
		result += s
	}
	return result
}
