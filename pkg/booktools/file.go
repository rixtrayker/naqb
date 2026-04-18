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

// ReadFileInput is the input schema for the read_file tool.
type ReadFileInput struct {
	Path string `json:"path" jsonschema:"description=Relative path from book root (e.g. chapters/ch-01.md)"`
}

// ReadFileTool reads any file within a book directory.
type ReadFileTool struct {
	BookDir string
}

func NewReadFileTool(bookDir string) runtime.Tool { return &ReadFileTool{BookDir: bookDir} }

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read a file from the book project. Path is relative to the book root (e.g. 'chapters/ch-01.md', 'contexts/ch-01-context.md', 'outline.md')." }
func (t *ReadFileTool) Schema() any         { return nil }

func (t *ReadFileTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args ReadFileInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Path == "" {
		return "path is required", nil
	}
	clean := filepath.Clean(args.Path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "path must be relative and within the book directory", nil
	}
	fullPath := filepath.Join(t.BookDir, clean)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("read error: %v", err), nil
	}
	return string(data), nil
}

func (t *ReadFileTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input ReadFileInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

// WriteFileInput is the input schema for the write_file tool.
type WriteFileInput struct {
	Path    string `json:"path"    jsonschema:"description=Relative path from book root to write"`
	Content string `json:"content" jsonschema:"description=Full file content to write"`
}

// WriteFileTool writes a file within a book directory.
type WriteFileTool struct {
	BookDir string
}

func NewWriteFileTool(bookDir string) runtime.Tool { return &WriteFileTool{BookDir: bookDir} }

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file in the book project. Path is relative to the book root. Creates a .bak backup of existing files before overwriting." }
func (t *WriteFileTool) Schema() any         { return nil }

func (t *WriteFileTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	var args WriteFileInput
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	if args.Path == "" {
		return "path is required", nil
	}
	if args.Content == "" {
		return "content is required", nil
	}
	clean := filepath.Clean(args.Path)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "path must be relative and within the book directory", nil
	}
	fullPath := filepath.Join(t.BookDir, clean)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Sprintf("mkdir error: %v", err), nil
	}

	if _, err := os.Stat(fullPath); err == nil {
		bakPath := fullPath + ".bak"
		existing, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return fmt.Sprintf("backup read error: %v", readErr), nil
		}
		if writeErr := os.WriteFile(bakPath, existing, 0o644); writeErr != nil {
			return fmt.Sprintf("backup write error: %v", writeErr), nil
		}
	}

	if err := os.WriteFile(fullPath, []byte(args.Content), 0o644); err != nil {
		return fmt.Sprintf("write error: %v", err), nil
	}
	wc := wordcount.Count(args.Content)
	return fmt.Sprintf("wrote %d bytes (%d words) to %s", len(args.Content), wc, args.Path), nil
}

func (t *WriteFileTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input WriteFileInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}

// ListChaptersInput is the input schema for the list_chapters tool.
type ListChaptersInput struct{}

// ListChaptersTool lists all chapters with status and word count.
type ListChaptersTool struct {
	BookDir string
	Cfg     *config.BookConfig
}

func NewListChaptersTool(bookDir string, cfg *config.BookConfig) runtime.Tool {
	return &ListChaptersTool{BookDir: bookDir, Cfg: cfg}
}

func (t *ListChaptersTool) Name() string        { return "list_chapters" }
func (t *ListChaptersTool) Description() string { return "List all book chapters with their title, word count, and whether a draft exists." }
func (t *ListChaptersTool) Schema() any         { return nil }

func (t *ListChaptersTool) Invoke(ctx context.Context, input string, opts ...runtime.Option) (string, error) {
	if t.Cfg == nil {
		return "book config not loaded", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — Chapter List\n\n", t.Cfg.Title)
	for _, ch := range t.Cfg.Chapters {
		path := filepath.Join(t.BookDir, "chapters", config.ChapterFilename(ch.Number))
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(&sb, "Ch %02d: %s — [no draft]\n", ch.Number, ch.Title)
		} else {
			wc := wordcount.Count(string(data))
			target := 3000
			if t.Cfg.TargetWords > 0 {
				target = t.Cfg.TargetWords
			}
			pct := float64(wc) / float64(target) * 100
			fmt.Fprintf(&sb, "Ch %02d: %s — %d words (%.0f%% of target)\n",
				ch.Number, ch.Title, wc, pct)
		}
	}
	return sb.String(), nil
}

func (t *ListChaptersTool) FantasyTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		t.Name(), t.Description(),
		func(ctx context.Context, input ListChaptersInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			raw, _ := json.Marshal(input)
			result, err := t.Invoke(ctx, string(raw))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(result), nil
		},
	)
}
