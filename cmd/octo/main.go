package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/Octo-sh/Octo-CLI/internal/ui"
	"github.com/spf13/cobra"
)

// Version information (can be set at build time via ldflags)
var (
	version = "dev"
)

// Command groups for organizing help output
var (
	mainGroup = &cobra.Group{
		ID:    "main",
		Title: "Main Commands:",
	}
	otherGroup = &cobra.Group{
		ID:    "other",
		Title: "Other Commands:",
	}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           "octo",
	Short:         "Execution intelligence engine for software repositories and AI agents",
	SilenceErrors: true,
	Long: `Octo is an execution intelligence CLI that analyzes codebases, detects
runtimes and dependencies, resolves network ports, plans execution deterministically,
and runs and verifies software stacks locally with zero configuration.`,
	Version: version,
}

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version

	// Register command groups to separate main and other commands in help output
	rootCmd.AddGroup(mainGroup)
	rootCmd.AddGroup(otherGroup)

	// Add subcommands
	runCmd.GroupID = "main"
	rootCmd.AddCommand(runCmd)

	// Setup tasteful colorized help
	ui.SetupColoredHelp(rootCmd)
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
