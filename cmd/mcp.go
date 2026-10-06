package main

import (
	"github.com/harshul/octo-cli/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the Model Context Protocol (MCP) server over stdio",
	Long: `The mcp command starts a JSON-RPC 2.0 Model Context Protocol (MCP) server
over standard input and output (stdio).

AI coding agents (such as Claude Code, Cursor, Antigravity, Cline, and Windsurf)
can connect to this server to discover repository technologies, generate
deterministic execution plans, execute and verify stacks, and diagnose failures.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcp.NewServer()
		return server.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
