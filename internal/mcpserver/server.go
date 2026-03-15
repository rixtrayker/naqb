// Package mcpserver exposes naqb's book-writing capabilities as an
// MCP (Model Context Protocol) server.
//
// Tools provided:
//
//	nqb_status   — list chapters and their completion state
//	nqb_context  — build the golden context file for a chapter
//	nqb_write    — write a chapter draft with the LLM
//	nqb_qa       — run QA checks on a chapter
//	nqb_research — run the research pipeline for a chapter
//	nqb_export   — export the book (pdf/epub/docx/web)
package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amr/naqb/internal/agents"
	"github.com/amr/naqb/internal/config"
	"github.com/amr/naqb/internal/exporter"
	"github.com/amr/naqb/internal/llm"
	"github.com/amr/naqb/internal/research"
	"github.com/amr/naqb/internal/wordcount"
)

// New creates and configures the nqb MCP server.
func New(apiKey string) *server.MCPServer {
	s := server.NewMCPServer(
		"nqb",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	registerTools(s, apiKey)
	return s
}

// ServeStdio starts the MCP server reading from stdin / writing to stdout.
// This is the standard transport for Claude Desktop and similar hosts.
func ServeStdio(s *server.MCPServer) error {
	return server.ServeStdio(s)
}

// ── Tool registration ─────────────────────────────────────────────────────────

func registerTools(s *server.MCPServer, apiKey string) {
	// nqb_status
	s.AddTool(
		mcp.NewTool("nqb_status",
			mcp.WithDescription("List all chapters in a book project and their completion status (written / not written), word counts, and QA state."),
			mcp.WithString("book_dir",
				mcp.Required(),
				mcp.Description("Absolute path to the book project directory (contains book.yaml)."),
			),
		),
		statusHandler,
	)

	// nqb_context
	s.AddTool(
		mcp.NewTool("nqb_context",
			mcp.WithDescription("Build (or rebuild) the golden context file for a chapter. The context file assembles book rules, outline section, prior chapter summaries, and research notes into a single LLM prompt."),
			mcp.WithString("book_dir", mcp.Required(), mcp.Description("Absolute path to the book project directory.")),
			mcp.WithNumber("chapter", mcp.Required(), mcp.Description("Chapter number to build context for.")),
		),
		contextHandler,
	)

	// nqb_write
	s.AddTool(
		mcp.NewTool("nqb_write",
			mcp.WithDescription("Write a chapter draft using the configured LLM. Builds the context file first, then calls the LLM writer agent. Returns the path to the written file and the first 500 chars of the content."),
			mcp.WithString("book_dir", mcp.Required(), mcp.Description("Absolute path to the book project directory.")),
			mcp.WithNumber("chapter", mcp.Required(), mcp.Description("Chapter number to write.")),
			mcp.WithString("api_key", mcp.Description("Anthropic API key (falls back to ANTHROPIC_API_KEY env var).")),
		),
		writeHandler(apiKey),
	)

	// nqb_qa
	s.AddTool(
		mcp.NewTool("nqb_qa",
			mcp.WithDescription("Run quality checks on a written chapter: heading hierarchy, code block labels, word count, callout syntax, and an LLM editorial review. Returns a structured report."),
			mcp.WithString("book_dir", mcp.Required(), mcp.Description("Absolute path to the book project directory.")),
			mcp.WithNumber("chapter", mcp.Required(), mcp.Description("Chapter number to QA.")),
			mcp.WithBoolean("deterministic_only", mcp.Description("If true, skip the LLM audit (faster, offline).")),
			mcp.WithString("api_key", mcp.Description("Anthropic API key.")),
		),
		qaHandler(apiKey),
	)

	// nqb_research
	s.AddTool(
		mcp.NewTool("nqb_research",
			mcp.WithDescription("Run the automated research pipeline for a chapter: Scout (LLM generates queries) → Explorer (web search) → Scribe (LLM synthesises notes). Notes are saved to .naqb/research/ and indexed in the vector store. Requires a search provider API key (TAVILY_API_KEY or EXA_API_KEY)."),
			mcp.WithString("book_dir", mcp.Required(), mcp.Description("Absolute path to the book project directory.")),
			mcp.WithNumber("chapter", mcp.Required(), mcp.Description("Chapter number to research.")),
			mcp.WithString("api_key", mcp.Description("Anthropic API key.")),
		),
		researchHandler(apiKey),
	)

	// nqb_export
	s.AddTool(
		mcp.NewTool("nqb_export",
			mcp.WithDescription("Export the book to a given format. Supported: pdf, epub, docx, web. For pdf/epub/docx, pandoc must be installed."),
			mcp.WithString("book_dir", mcp.Required(), mcp.Description("Absolute path to the book project directory.")),
			mcp.WithString("format", mcp.Required(), mcp.Description("Export format: pdf | epub | docx | web")),
		),
		exportHandler,
	)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func statusHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bookDir := mcp.ParseString(req, "book_dir", "")
	if bookDir == "" {
		return mcp.NewToolResultError("book_dir is required"), nil
	}

	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n", cfg.Title))
	sb.WriteString(fmt.Sprintf("Author: %s | Domain: %s | Language: %s\n\n", cfg.Author, cfg.Domain, cfg.Language))

	total, written := 0, 0
	for _, ch := range cfg.Chapters {
		total++
		path := filepath.Join(bookDir, "chapters", config.ChapterFilename(ch.Number))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			sb.WriteString(fmt.Sprintf("○ Ch%02d: %s  [not written]\n", ch.Number, ch.Title))
			continue
		}
		written++
		wc := wordcount.Count(string(data))
		sb.WriteString(fmt.Sprintf("● Ch%02d: %s  [%d words]\n", ch.Number, ch.Title, wc))
	}

	sb.WriteString(fmt.Sprintf("\n%d/%d chapters written\n", written, total))
	return mcp.NewToolResultText(sb.String()), nil
}

func contextHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bookDir := mcp.ParseString(req, "book_dir", "")
	chNum := mcp.ParseInt(req, "chapter", 0)
	if bookDir == "" || chNum == 0 {
		return mcp.NewToolResultError("book_dir and chapter are required"), nil
	}

	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
	}

	path, err := agents.WriteContextFile(bookDir, cfg, chNum)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("building context", err), nil
	}

	data, _ := os.ReadFile(path)
	return mcp.NewToolResultText(fmt.Sprintf("Context written to %s\n\n%s", path, string(data))), nil
}

func writeHandler(defaultKey string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bookDir := mcp.ParseString(req, "book_dir", "")
		chNum := mcp.ParseInt(req, "chapter", 0)
		if bookDir == "" || chNum == 0 {
			return mcp.NewToolResultError("book_dir and chapter are required"), nil
		}

		cfg, err := config.LoadBook(bookDir)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
		}

		client, err := resolveClient(req, defaultKey, cfg.LLM.WriteProvider)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("resolving LLM client", err), nil
		}

		path, err := agents.WriteChapter(ctx, client, bookDir, cfg, chNum, nil)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("writing chapter", err), nil
		}

		data, _ := os.ReadFile(path)
		preview := string(data)
		if len(preview) > 500 {
			preview = preview[:500] + "…"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Chapter written to %s\n\nPreview:\n%s", path, preview)), nil
	}
}

func qaHandler(defaultKey string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bookDir := mcp.ParseString(req, "book_dir", "")
		chNum := mcp.ParseInt(req, "chapter", 0)
		detOnly := mcp.ParseBoolean(req, "deterministic_only", false)
		if bookDir == "" || chNum == 0 {
			return mcp.NewToolResultError("book_dir and chapter are required"), nil
		}

		cfg, err := config.LoadBook(bookDir)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
		}

		var client llm.Provider
		if !detOnly {
			client, err = resolveClient(req, defaultKey, cfg.LLM.QAProvider)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("resolving LLM client", err), nil
			}
		}

		result, err := agents.RunQA(ctx, client, bookDir, cfg, chNum)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("running QA", err), nil
		}
		_ = agents.WriteQAReport(bookDir, result)

		var sb strings.Builder
		if result.DeterministicOK {
			sb.WriteString("✓ Deterministic checks: PASSED\n")
		} else {
			sb.WriteString("✗ Deterministic checks: ISSUES\n")
			for _, issue := range result.Issues {
				sb.WriteString("  - " + issue + "\n")
			}
		}
		if result.LLMReport != "" && result.LLMReport != "(LLM audit skipped)" {
			sb.WriteString("\n## LLM Audit\n\n")
			sb.WriteString(result.LLMReport)
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func researchHandler(defaultKey string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		bookDir := mcp.ParseString(req, "book_dir", "")
		chNum := mcp.ParseInt(req, "chapter", 0)
		if bookDir == "" || chNum == 0 {
			return mcp.NewToolResultError("book_dir and chapter are required"), nil
		}

		cfg, err := config.LoadBook(bookDir)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
		}

		client, err := resolveClient(req, defaultKey, cfg.LLM.WriteProvider)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("resolving LLM client", err), nil
		}

		rules, _ := config.LoadRules(bookDir)
		result, err := research.Run(ctx, client, bookDir, cfg, chNum, rules, os.Stderr)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("research pipeline", err), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Research complete for chapter %d\n", chNum))
		sb.WriteString(fmt.Sprintf("Queries: %d | Results: %d | Notes saved: %d\n\n", result.Queries, result.Results, len(result.Notes)))
		for _, note := range result.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note.Filename))
		}
		return mcp.NewToolResultText(sb.String()), nil
	}
}

func exportHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	bookDir := mcp.ParseString(req, "book_dir", "")
	format := mcp.ParseString(req, "format", "web")
	if bookDir == "" {
		return mcp.NewToolResultError("book_dir is required"), nil
	}

	cfg, err := config.LoadBook(bookDir)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("loading book.yaml", err), nil
	}

	paths, err := exporter.Export(format, bookDir, cfg)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("export failed", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Export complete (%s):\n%s", format, strings.Join(paths, "\n"))), nil
}

// ── Client resolver ───────────────────────────────────────────────────────────

// resolveClient picks the LLM provider in this priority:
// 1. api_key arg in the MCP request
// 2. defaultKey from the server startup (CLI flag / env)
// 3. Named provider from book.yaml via global config
func resolveClient(req mcp.CallToolRequest, defaultKey, namedProvider string) (llm.Provider, error) {
	if key := mcp.ParseString(req, "api_key", ""); key != "" {
		return llm.New(key), nil
	}
	if defaultKey != "" {
		return llm.New(defaultKey), nil
	}
	// Fall through to named provider or env var
	pcfg, err := config.ProviderConfigFor(namedProvider)
	if err != nil {
		// Try ANTHROPIC_API_KEY as last resort
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return llm.New(key), nil
		}
		return nil, fmt.Errorf("no API key available: set ANTHROPIC_API_KEY or pass api_key in the tool call")
	}
	return llm.NewProvider(llm.ProviderConfig{
		Type:    pcfg.Type,
		APIKey:  pcfg.APIKey,
		BaseURL: pcfg.BaseURL,
	})
}

// RunResultNotes is an alias to avoid importing research.Note in the pipeline summary.
type RunResultNotes = research.Note
