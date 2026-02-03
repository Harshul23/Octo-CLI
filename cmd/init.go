package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/analyzer"
	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Analyze the codebase and generate a .octo.yaml file",
	Long: `The init command analyzes your current directory to detect:
- Programming languages and frameworks
- Package managers and dependencies
- Build systems and scripts
- Runtime requirements

It then generates a .octo.yaml configuration file that can be used
with 'octo run' to deploy your application locally.`,
	RunE: runInit,
}

func init() {
	// Add flags specific to the init command
	initCmd.Flags().StringP("output", "o", ".octo.yaml", "Output file path for the configuration")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing .octo.yaml file")
	initCmd.Flags().BoolP("interactive", "i", false, "Run in interactive mode with prompts")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get flag values
	outputPath, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	interactive, _ := cmd.Flags().GetBool("interactive")

	// Resolve output path
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(cwd, outputPath)
	}

	// Check if file already exists
	if _, err := os.Stat(outputPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists at %s. Use --force to overwrite", outputPath)
	}

	// Start the initialization process
	spinner := ui.NewSpinner("Analyzing codebase...")
	spinner.Start()

	// Analyze the codebase
	analysis, err := analyzer.Analyze(cwd)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("analysis failed: %w", err)
	}

	spinner.Stop()

	// If interactive mode, prompt user for confirmation/modifications
	if interactive {
		analysis, err = ui.PromptForConfirmation(analysis)
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}
	}

	// Generate the blueprint
	bp := blueprint.FromAnalysis(analysis)

	// Write the configuration file
	if err := blueprint.Write(outputPath, bp); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	ui.Success(fmt.Sprintf("Configuration written to %s", outputPath))
	ui.Info("Run 'octo run' to start your application")

	return nil
}
