package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information (can be set at build time)
var (
	version = "0.1.0"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "octo",
	Short: "Automate local deployment of any software with zero configuration",
	Long: `Octo is a CLI tool that automates the local deployment of any software 
with zero configuration. It analyzes your codebase, detects the tech stack,
and generates a deployment configuration file.

Usage:
  octo init    Analyze the codebase and generate a .octo.yaml file
  octo run     Execute the software based on the .octo.yaml file`,
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
