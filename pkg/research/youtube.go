package research

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"github.com/amr/naqb/pkg/youtube"
)

// RunYouTubeResearch fetches a YouTube transcript and synthesises it into
// atomic research notes via the Scribe pipeline.
func RunYouTubeResearch(ctx context.Context, client llm.Provider, bookDir string, cfg *config.BookConfig, chapterNum int, videoURL string, out io.Writer) (*RunResult, error) {
	log.Info("youtube research start", "chapter", chapterNum, "url", videoURL)
	fmt.Fprintf(out, "  [YouTube] Extracting video ID...\n")

	videoID, err := youtube.ExtractVideoID(videoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid youtube URL: %w", err)
	}
	fmt.Fprintf(out, "  [YouTube] Video ID: %s\n", videoID)

	// Determine language preference from book config
	langs := []string{"en"}
	if cfg.Language != "" && cfg.Language != "en" {
		langs = []string{cfg.Language, "en"}
	}

	fmt.Fprintf(out, "  [YouTube] Fetching transcript...\n")
	transcript, lang, err := youtube.FetchTranscript(videoID, langs)
	if err != nil {
		return nil, fmt.Errorf("youtube transcript failed: %w", err)
	}
	fmt.Fprintf(out, "  [YouTube] Transcript fetched (%s, %d chars)\n", lang, len(transcript))

	if strings.TrimSpace(transcript) == "" {
		return nil, fmt.Errorf("no transcript available for this video")
	}

	// Build a pseudo-RawResult so Scribe can synthesise it
	raw := []RawResult{{
		Query: videoURL,
		Results: []SearchResult{{
			Title:   fmt.Sprintf("YouTube Transcript (%s)", videoID),
			URL:     videoURL,
			Snippet: transcript,
			Body:    transcript,
		}},
	}}

	fmt.Fprintf(out, "  [YouTube] Synthesising research notes...\n")
	notes, err := Scribe(ctx, client, cfg, chapterNum, raw, bookDir)
	if err != nil {
		return nil, fmt.Errorf("scribe failed: %w", err)
	}
	fmt.Fprintf(out, "        %d notes saved to .naqb/research/\n", len(notes))

	// Index new notes into the vector store (best-effort).
	indexResearchNotes(ctx, bookDir, notes)

	log.Info("youtube research done", "chapter", chapterNum, "notes", len(notes))
	return &RunResult{Queries: 1, Results: 1, Notes: notes}, nil
}
