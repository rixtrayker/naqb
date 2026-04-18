package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/amr/naqb/internal/mcpserver"
)

// MCPCmd returns the `nqb mcp` command, which starts the MCP server over stdio.
func MCPCmd() *cobra.Command {
	var apiKey string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the nqb MCP server over stdio",
		GroupID: "config",
		Long: `Start the نقب MCP server using stdio transport.

Add to Claude Desktop's claude_desktop_config.json:

  {
    "mcpServers": {
      "nqb": {
        "command": "nqb",
        "args": ["mcp"],
        "env": { "ANTHROPIC_API_KEY": "sk-ant-..." }
      }
    }
  }

Available MCP tools:
  nqb_status    List chapters and completion state
  nqb_context   Build the golden context file for a chapter
  nqb_write     Write a chapter draft
  nqb_qa        Run QA checks on a chapter
  nqb_research  Run the research pipeline for a chapter
  nqb_export    Export the book (pdf/epub/docx/web)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			s := mcpserver.New(apiKey)
			if err := mcpserver.ServeStdio(s); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "Anthropic API key (defaults to ANTHROPIC_API_KEY env var)")
	return cmd
}
