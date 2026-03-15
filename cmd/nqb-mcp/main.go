// Command nqb-mcp starts the nqb MCP server over stdio.
//
// Usage:
//
//	nqb-mcp [--api-key KEY]
//
// The server communicates over stdin/stdout using the JSON-RPC MCP protocol.
// Add it to Claude Desktop's config or any MCP-compatible host:
//
//	{
//	  "mcpServers": {
//	    "nqb": {
//	      "command": "/path/to/nqb-mcp",
//	      "args": [],
//	      "env": { "ANTHROPIC_API_KEY": "sk-ant-..." }
//	    }
//	  }
//	}
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/mcpserver"
)

func main() {
	var apiKey string

	root := &cobra.Command{
		Use:   "nqb-mcp",
		Short: "nqb MCP server — expose book-writing tools to AI agents",
		Long: `nqb-mcp starts the نقب MCP server over stdio.

Add to Claude Desktop or any MCP host:

  {
    "mcpServers": {
      "nqb": {
        "command": "nqb-mcp",
        "env": { "ANTHROPIC_API_KEY": "sk-ant-..." }
      }
    }
  }

Available tools:
  nqb_status    List chapters and their completion state
  nqb_context   Build the golden context file for a chapter
  nqb_write     Write a chapter draft with the LLM
  nqb_qa        Run QA checks on a chapter
  nqb_research  Run the research pipeline for a chapter
  nqb_export    Export the book (pdf/epub/docx/web)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fall back to environment variable
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			s := mcpserver.New(apiKey)
			return mcpserver.ServeStdio(s)
		},
	}

	root.Flags().StringVar(&apiKey, "api-key", "", "Anthropic API key (defaults to ANTHROPIC_API_KEY env var)")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
