package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/harshul/octo-cli/internal/orchestrator"
	"github.com/harshul/octo-cli/internal/secrets"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Understand and run the repository locally",
	Long: `The run command analyzes the repository, builds an execution plan,
resolves the required environment, executes the plan, and verifies the
result.

The intelligence engine is the default. Use --engine legacy only when
you explicitly need the older .octo.yaml-based orchestrator.`,
	RunE: runRun,
}

func init() {
	// Add flags specific to the run command
	runCmd.Flags().StringP("config", "c", ".octo.yaml", "Path to the configuration file")
	runCmd.Flags().StringP("env", "e", "development", "Environment to run (development, production)")
	runCmd.Flags().BoolP("build", "b", true, "Run build step before execution")
	runCmd.Flags().BoolP("watch", "w", false, "Watch for file changes and restart")
	runCmd.Flags().BoolP("detach", "d", false, "Run in detached mode (background)")
	runCmd.Flags().IntP("port", "p", 0, "Override the port to run on (0 = use config default)")
	runCmd.Flags().Bool("no-port-shift", false, "Disable automatic port shifting on conflicts")
	runCmd.Flags().Bool("skip-env-check", false, "Skip environment variable validation")
	runCmd.Flags().Bool("no-tui", false, "Disable TUI dashboard (use plain scrolling output)")
	runCmd.Flags().String("engine", "intelligence", "Execution engine: intelligence or legacy")
}

func runRun(cmd *cobra.Command, args []string) error {
	engine, _ := cmd.Flags().GetString("engine")
	if engine != "legacy" && engine != "intelligence" {
		return fmt.Errorf("invalid execution engine %q: use legacy or intelligence", engine)
	}
	if engine == "intelligence" {
		return runWithIntelligence(cmd)
	}

	// ========================================
	// Show intro animation
	// ========================================
	ui.RunIntro()

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
	port, _ := cmd.Flags().GetInt("port")
	noPortShift, _ := cmd.Flags().GetBool("no-port-shift")
	skipEnvCheck, _ := cmd.Flags().GetBool("skip-env-check")
	noTUI, _ := cmd.Flags().GetBool("no-tui")
	
	// Dashboard is enabled by default unless --no-tui is specified or running in detached mode
	useDashboard := !noTUI && !detach

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

	// Check if running inside the Octo project itself
	if ui.IsOctoProject(bp.Name, bp.Language, cwd) {
		ui.RunWelcomeScreen()
		return nil
	}

	// Pre-run environment validation and auto-provisioning
	if !skipEnvCheck {
		valid, _ := secrets.PreRunEnvValidation(cwd, bp.Language)
		if !valid {
			// Auto-provision missing env files with README defaults (don't show scary warnings first)
			result, err := secrets.AutoProvisionEnvFiles(cwd, bp.Language)
			if err != nil {
				ui.Warn(fmt.Sprintf("Failed to auto-provision environment: %v", err))
			} else if len(result.ProvisionedVars) > 0 || len(result.CreatedFiles) > 0 {
				// Show success message about what was auto-configured
				fmt.Println()
				fmt.Println("🔧 Auto-configuring environment...")
				
				if len(result.CreatedFiles) > 0 {
					for _, f := range result.CreatedFiles {
						ui.Success(fmt.Sprintf("Created %s", f))
					}
				}
				
				if len(result.ProvisionedVars) > 0 {
					ui.Success(fmt.Sprintf("Set %d environment variable(s) with smart defaults:", len(result.ProvisionedVars)))
					for name, value := range result.ProvisionedVars {
						fmt.Printf("   • %s=%s\n", name, maskEnvValue(value))
					}
				}
				
				if len(result.SkippedVars) > 0 {
					fmt.Println()
					ui.Warn(fmt.Sprintf("%d variable(s) still need manual configuration:", len(result.SkippedVars)))
					for _, name := range result.SkippedVars {
						fmt.Printf("   • %s\n", name)
					}
				}
				fmt.Println()
			}

			// Re-validate after auto-provisioning
			valid, issues := secrets.PreRunEnvValidation(cwd, bp.Language)
			if !valid {
				// Only show issues that remain AFTER auto-provisioning
				ui.DisplayPreRunEnvValidation(issues)
				
				// Ask if user wants to continue anyway
				if !ui.PromptContinueDespiteEnvIssues() {
					ui.Info("Run 'octo init' to configure environment variables.")
					return fmt.Errorf("aborted due to environment configuration issues")
				}
			} else {
				// Everything was auto-fixed!
				ui.Success("Environment configured successfully!")
				fmt.Println()
			}
		}
	}

	ui.Info(fmt.Sprintf("Running %s in %s mode...", bp.Name, env))

	// Create orchestrator options
	opts := orchestrator.Options{
		WorkDir:      cwd,
		Environment:  env,
		RunBuild:     build,
		Watch:        watch,
		Detach:       detach,
		PortOverride: port,
		NoPortShift:  noPortShift,
		SkipEnvCheck: skipEnvCheck,
		UseDashboard: useDashboard,
	}

	// Create and run the orchestrator
	orch, err := orchestrator.New(bp, opts)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Execute the application
	if useDashboard {
		if err := orch.RunWithDashboard(); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
	} else {
		if err := orch.Run(); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
	}

	return nil
}

// maskEnvValue masks sensitive values for display
func maskEnvValue(value string) string {
	// Don't mask URLs - they're usually not secret
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "ws://") || strings.HasPrefix(value, "wss://") ||
		strings.HasPrefix(value, "postgresql://") || strings.HasPrefix(value, "redis://") {
		return value
	}

	// Don't mask short values or common non-secrets
	if len(value) <= 10 {
		return value
	}

	// Mask the middle of longer values
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}


func runWithIntelligence(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	watch, _ := cmd.Flags().GetBool("watch")
	detach, _ := cmd.Flags().GetBool("detach")
	if watch || detach {
		return fmt.Errorf("the intelligence engine currently does not support --watch or --detach; use --engine legacy for these modes")
	}

	fmt.Println("Octo intelligence engine")
	fmt.Println("Analyzing repository...")

	model, err := intelligence.Analyze(cwd)
	if err != nil {
		return fmt.Errorf("intelligence analysis failed: %w", err)
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(cmd.Context(), model)
	if err != nil {
		return fmt.Errorf("execution planning failed: %w", err)
	}

	fmt.Printf("Detected %d component(s) and %d service(s).\n", len(model.Components), len(model.Services))
	fmt.Printf("Execution plan contains %d step(s).\n", len(plan.Steps))

	env, err := intelligence.ResolveProjectEnvironment(cwd, model)
	if err != nil {
		return fmt.Errorf("environment resolution failed: %w", err)
	}

	report := intelligence.ExecutePlanReport(cmd.Context(), model, plan, intelligence.NewRuntimeResolver(), env)
	for _, step := range report.Steps {
		switch step.Status {
		case intelligence.StepSucceeded:
			fmt.Printf("✓ %s (%s)\n", step.ID, step.Adapter)
		case intelligence.StepSkipped:
			fmt.Printf("○ %s — %s\n", step.ID, step.Reason)
		case intelligence.StepFailed:
			fmt.Printf("✗ %s — %s\n", step.ID, step.Reason)
		}
	}

	for _, check := range report.Verification {
		if check.Passed {
			fmt.Printf("✓ verification %s\n", check.CheckID)
		} else {
			fmt.Printf("✗ verification %s — %s\n", check.CheckID, check.Reason)
		}
	}

	if !report.Success {
		return fmt.Errorf("intelligence execution failed: %s", report.FailureReason)
	}

	fmt.Println("Execution verified successfully.")
	return nil
}

