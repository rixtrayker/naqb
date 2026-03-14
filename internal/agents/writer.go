package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/log"
)

// WriteChapter reads the context file, calls the LLM, and writes the chapter file.
// onDelta is called with each streamed text chunk (can be nil).
func WriteChapter(ctx context.Context, client *llm.Client, bookDir string, cfg *config.BookConfig, chapterNum int, onDelta llm.StreamFunc) (string, error) {
	log.Info("write chapter start", "chapter", chapterNum, "book", cfg.Title)

	// Read system prompt
	systemPrompt, err := readPrompt(bookDir, "write.md")
	if err != nil {
		log.Warn("write prompt not found, using default", "err", err)
		systemPrompt = "You are an expert author. Write the requested chapter in full."
	}

	// Read context file
	contextFile := filepath.Join(bookDir, "contexts", config.ContextFilename(chapterNum))
	contextData, err := os.ReadFile(contextFile)
	if err != nil {
		log.Warn("context file missing, building on-the-fly", "chapter", chapterNum, "err", err)
		contextContent, buildErr := BuildContext(bookDir, cfg, chapterNum)
		if buildErr != nil {
			log.Error("failed to build context", "chapter", chapterNum, "err", buildErr)
			return "", fmt.Errorf("context file not found and could not build: %w", buildErr)
		}
		contextData = []byte(contextContent)
	}
	log.Debug("context loaded", "chapter", chapterNum, "bytes", len(contextData))

	model := cfg.LLM.WriteModel
	if model == "" {
		model = llm.ModelSonnet
	}

	messages := []llm.Message{
		{Role: "user", Content: string(contextData)},
	}

	content, err := client.Stream(ctx, model, systemPrompt, messages, 8192, onDelta)
	if err != nil {
		log.Error("write chapter LLM failed", "chapter", chapterNum, "err", err)
		return "", fmt.Errorf("writer LLM call failed: %w", err)
	}
	log.Debug("chapter content received", "chapter", chapterNum, "chars", len(content))

	// Save chapter file
	chaptersDir := filepath.Join(bookDir, "chapters")
	if err := os.MkdirAll(chaptersDir, 0o750); err != nil {
		return "", err
	}

	outPath := filepath.Join(chaptersDir, config.ChapterFilename(chapterNum))
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		log.Error("failed to write chapter file", "path", outPath, "err", err)
		return "", fmt.Errorf("writing chapter file: %w", err)
	}

	log.Info("write chapter done", "chapter", chapterNum, "path", outPath, "chars", len(content))
	return outPath, nil
}

func readPrompt(bookDir, name string) (string, error) {
	path := filepath.Join(bookDir, "config", "prompts", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
