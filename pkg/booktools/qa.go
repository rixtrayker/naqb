package booktools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/runtime"
	"github.com/amr/naqb/pkg/wordcount"
)

// RunQAInput is the input schema for the run_qa tool.
type RunQAInput struct {
	ChapterNum int `json:"chapter_num" jsonschema:"description=Chapter number to run QA on"`
}

// RunQATool runs deterministic QA on a chapter.
type RunQATool struct {
	BookDir string
	Cfg     *config.BookConfig
}

func NewRunQATool(bookDir string, cfg *config.BookConfig) runtime.Tool {
	return &RunQATool{BookDir: bookDir, Cfg: cfg}
}

func (t *RunQATool) Name() string        { return "run_qa" }
func (t *RunQATool) Description() string { return "Run quality assurance checks (deterministic) on a chapter. Returns a report of findings." }
func (t *RunQATool) Schema() any         { return nil }

func (t *RunQATool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args RunQAInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.ChapterNum <= 0 {
		return "chapter_num must be >= 1", nil
	}

	chapterPath := filepath.Join(t.BookDir, "chapters", config.ChapterFilename(args.ChapterNum))
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		return fmt.Sprintf("chapter %d not found: %v", args.ChapterNum, err), nil
	}

	var findings []string
	content := string(data)

	wc := wordcount.Count(content)
	target := 3000
	if t.Cfg != nil && t.Cfg.TargetWords > 0 {
		target = t.Cfg.TargetWords
	}
	pct := float64(wc) / float64(target) * 100
	findings = append(findings, fmt.Sprintf("Word count: %d / %d target (%.0f%%)", wc, target, pct))
	if wc < target/2 {
		findings = append(findings, "⚠ Chapter is very short (< 50% of target)")
	}

	if strings.TrimSpace(content) == "" {
		findings = append(findings, "✗ Chapter file is empty")
	}

	lines := strings.Split(content, "\n")
	headings := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			headings++
		}
	}
	findings = append(findings, fmt.Sprintf("Headings found: %d", headings))
	if headings == 0 {
		findings = append(findings, "⚠ No headings found — consider adding structure")
	}

	return strings.Join(findings, "\n"), nil
}

func (t *RunQATool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input RunQAInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}
