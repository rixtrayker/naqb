// Package jobs provides a persistent background job queue backed by SQLite.
// Jobs are enqueued via the CLI (nqb batch enqueue) and processed by a worker
// pool (nqb batch run).
package jobs

// JobType identifies the kind of work a job represents.
type JobType string

const (
	// JobWrite queues the WriteStage for a single chapter via the agent loop.
	JobWrite JobType = "write"
	// JobQA queues the QA stage for a chapter.
	JobQA JobType = "qa"
	// JobResearch queues a research run for a chapter.
	JobResearch JobType = "research"
	// JobPipeline queues the full pipeline (context+write+qa) for a chapter.
	JobPipeline JobType = "pipeline"
)

// WritePayload is the JSON payload for a write job.
type WritePayload struct {
	ChapterNum int    `json:"chapter_num"`
	Model      string `json:"model,omitempty"`
}

// QAPayload is the JSON payload for a QA job.
type QAPayload struct {
	ChapterNum int `json:"chapter_num"`
}

// ResearchPayload is the JSON payload for a research job.
type ResearchPayload struct {
	ChapterNum int  `json:"chapter_num"`
	Deep       bool `json:"deep,omitempty"`
}

// PipelinePayload is the JSON payload for a pipeline job.
type PipelinePayload struct {
	ChapterNum int `json:"chapter_num"`
}
