// Package youtube provides transcript fetching for YouTube videos.
package youtube

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript"
	"github.com/horiagug/youtube-transcript-api-go/pkg/yt_transcript_formatters"
)

// ExtractVideoID parses a YouTube URL and returns the video ID.
func ExtractVideoID(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	switch u.Host {
	case "www.youtube.com", "youtube.com", "m.youtube.com":
		q := u.Query()
		if v := q.Get("v"); v != "" {
			return v, nil
		}
		// Handle /embed/ or /v/ paths
		path := strings.Trim(u.Path, "/")
		if strings.HasPrefix(path, "embed/") {
			return strings.TrimPrefix(path, "embed/"), nil
		}
		if strings.HasPrefix(path, "v/") {
			return strings.TrimPrefix(path, "v/"), nil
		}
	case "youtu.be":
		return strings.Trim(u.Path, "/"), nil
	case "www.youtu.be":
		return strings.Trim(u.Path, "/"), nil
	}

	// If no host parsed, treat raw as potential video ID (11 chars)
	if !strings.Contains(raw, "://") && len(raw) == 11 {
		return raw, nil
	}

	return "", fmt.Errorf("could not extract video ID from %q", raw)
}

// FetchTranscript retrieves the transcript for a YouTube video.
// languages is a priority list (e.g., ["en", "ar"]).
func FetchTranscript(videoID string, languages []string) (string, string, error) {
	client := yt_transcript.NewClient(
		yt_transcript.WithFormatter(yt_transcript_formatters.NewTextFormatter(
			yt_transcript_formatters.WithTimestamps(false),
		)),
	)

	if len(languages) == 0 {
		languages = []string{"en"}
	}

	transcript, err := client.GetFormattedTranscripts(videoID, languages, true)
	if err != nil {
		return "", "", fmt.Errorf("transcript fetch failed: %w", err)
	}

	lang := languages[0]
	// Try to determine actual language from raw transcripts
	rawTranscripts, _ := client.GetTranscripts(videoID, languages)
	if len(rawTranscripts) > 0 && rawTranscripts[0].Language != "" {
		lang = rawTranscripts[0].Language
	}

	return transcript, lang, nil
}
