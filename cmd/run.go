package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/orchestrator"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the software based on the .octo.yaml file",
	Long: `The run command reads the .octo.yaml configuration file and
executes your application using the detected environment settings.

It will:
- Set up the required runtime environment
- Install dependencies if needed
- Execute build commands
- Start your application

The execution method (Docker, Nix, or Shell) is determined by your
configuration and system capabilities.`,
	RunE: runRun,
}

func init() {
	// Add flags specific to the run command
	runCmd.Flags().StringP("config", "c", ".octo.yaml", "Path to the configuration file")
	runCmd.Flags().StringP("env", "e", "development", "Environment to run (development, production)")
	runCmd.Flags().BoolP("build", "b", true, "Run build step before execution")
	runCmd.Flags().BoolP("watch", "w", false, "Watch for file changes and restart")
	runCmd.Flags().BoolP("detach", "d", false, "Run in detached mode (background)")
}

func runRun(cmd *cobra.Command, args []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get flag values
	configPath, _ := cmd.Flags().GetString("config")
	env, _ := cmd.Flags().GetString("env")
	build, _ := cmd.Flags().GetBool("build")
	watch, _ := cmd.Flags().GetBool("watch")
	detach, _ := cmd.Flags().GetBool("detach")

	// Resolve config path
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(cwd, configPath)
	}

	// Check if configuration file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found at %s. Run 'octo init' first", configPath)
	}

	// Read the blueprint
	bp, err := blueprint.Read(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	ui.Info(fmt.Sprintf("Running %s in %s mode...", bp.Name, env))

	// Create orchestrator options
	opts := orchestrator.Options{
		WorkDir:     cwd,
		Environment: env,
		RunBuild:    build,
		Watch:       watch,
		Detach:      detach,
	}

	// Create and run the orchestrator
	orch, err := orchestrator.New(bp, opts)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Execute the application
	if err := orch.Run(); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}
