package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version information (can be set at build time via ldflags)
var (
	version = "dev"
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
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version

	// Add subcommands
	rootCmd.AddCommand(runCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		var reported reportedError
		if !errors.As(err, &reported) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
