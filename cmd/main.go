package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (can be set at build time)
var (
	version = "0.2.0"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "octo",
	Short:         "Execution intelligence engine for software repositories and AI agents",
	SilenceErrors: true,
	Long: `Octo is an execution intelligence CLI that analyzes codebases, detects
runtimes and dependencies, resolves network ports, plans execution deterministically,
and runs and verifies software stacks locally with zero configuration.

Usage:
  octo run [path]       Analyze, plan, execute, and verify a repository locally
  octo plan [path]      Display the deterministic execution plan
  octo inspect [path]   Inspect detected runtime facts, dependencies, and evidence
  octo mcp              Start the Model Context Protocol (MCP) server for AI coding agents`,
	Version: version,
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(runCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
