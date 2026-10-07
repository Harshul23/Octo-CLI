package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/intelligence"
	"github.com/harshul/octo-cli/internal/orchestrator"
	"github.com/harshul/octo-cli/internal/secrets"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Understand and run the repository locally",
	Args:  cobra.MaximumNArgs(1),
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
	runCmd.Flags().BoolP("sandbox", "s", false, "Execute application inside an ephemeral, isolated container sandbox")
	runCmd.Flags().Bool("json", false, "Output the ExecutionReport as JSON")
}

func runRun(cmd *cobra.Command, args []string) error {
	engine, _ := cmd.Flags().GetString("engine")
	if engine != "legacy" && engine != "intelligence" {
		return fmt.Errorf("invalid execution engine %q: use legacy or intelligence", engine)
	}
	if engine == "intelligence" {
		return runWithIntelligence(cmd, args)
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


func runWithIntelligence(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) == 1 {
		target = args[0]
	}
	cwd, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("failed to resolve directory %s: %w", target, err)
	}

	watch, _ := cmd.Flags().GetBool("watch")
	detach, _ := cmd.Flags().GetBool("detach")
	sandbox, _ := cmd.Flags().GetBool("sandbox")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if watch && detach {
		return fmt.Errorf("cannot combine --watch and --detach")
	}

	if jsonOutput {
		cmd.SilenceUsage = true
	} else {
		fmt.Println(ui.HeadingStyle.Render("Octo"))
		fmt.Println(ui.Muted.Render("Analyzing repository..."))
	}

	model, err := intelligence.Analyze(cwd)
	if err != nil {
		return fmt.Errorf("intelligence analysis failed: %w", err)
	}

	lock, err := intelligence.LoadOctoLock(cwd)
	if err != nil {
		return fmt.Errorf("failed to read .octo.lock: %w", err)
	}
	strategyStatus, err := intelligence.ApplyVerifiedStrategies(cwd, &model, lock)
	if err != nil {
		return fmt.Errorf("failed to validate verified strategies: %w", err)
	}
	if !jsonOutput {
		for _, strategy := range strategyStatus.Reused {
			fmt.Println(ui.SuccessLine(fmt.Sprintf("Reusing verified strategy for %s: %s → %s", strategy.Component, strategy.Candidate, strategy.Command)))
		}
		for _, strategy := range strategyStatus.Invalidated {
			fmt.Println(ui.WarningLine(fmt.Sprintf("Verified strategy invalidated for %s: repository changed or candidate no longer matches", strategy.Component)))
		}
	}

	noTUI, _ := cmd.Flags().GetBool("no-tui")
	var decisionProvider intelligence.DecisionProvider = intelligence.DeterministicDecisionProvider{}
	if !noTUI && !jsonOutput {
		decisionProvider = ui.InteractiveDecisionProvider{}
	}

	planner := intelligence.DeterministicPlanner{DecisionProvider: decisionProvider}
	plan, err := planner.Plan(cmd.Context(), model)
	if err != nil {
		return fmt.Errorf("execution planning failed: %w", err)
	}

	if !jsonOutput {
		printExecutionOverview(model, plan)
	}

	env, err := intelligence.ResolveProjectEnvironment(cwd, model)
	if err != nil {
		return fmt.Errorf("environment resolution failed: %w", err)
	}

	resolver := intelligence.NewRuntimeResolverWithOptions(intelligence.RuntimeResolverOptions{
		Sandbox: sandbox,
		Root:    cwd,
	})

	if detach {
		opts := intelligence.ExecutionOptions{Detach: true, Silent: jsonOutput, Sandbox: sandbox}
		report := intelligence.ExecutePlanReportWithOptions(cmd.Context(), model, plan, resolver, env, opts)
		if jsonOutput {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		} else {
			printExecutionReport(report)
		}

		if !report.Success {
			return fmt.Errorf("intelligence execution failed: %s", report.FailureReason)
		}

		if err := intelligence.RecordVerifiedStrategies(cwd, model, plan, report, lock); err != nil {
			return fmt.Errorf("failed to update .octo.lock: %w", err)
		}
		_ = intelligence.SaveDetachedState(cwd, report)

		if !jsonOutput {
			fmt.Println()
			fmt.Println(ui.SuccessStyle.Render("✓ Execution verified successfully."))
			if len(report.ActiveProcesses) > 0 {
				fmt.Println()
				fmt.Println(ui.HeadingStyle.Render("Running in detached mode:"))
				for _, proc := range report.ActiveProcesses {
					portInfo := ""
					if proc.Port > 0 {
						portInfo = fmt.Sprintf(" → http://localhost:%d", proc.Port)
					}
					fmt.Printf("  • %s (PID: %d)%s\n", proc.Component, proc.PID, portInfo)
				}
				fmt.Println(ui.Muted.Render("Active processes recorded in .octo/processes.json"))
			}
		}
		return nil
	}

	// Setup graceful signal handling for SIGINT (Ctrl+C) and SIGTERM
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	execCtx, cancelExec := context.WithCancel(cmd.Context())
	defer cancelExec()

	interrupted := false
	go func() {
		sig, ok := <-sigChan
		if !ok {
			return
		}
		interrupted = true
		if !jsonOutput {
			fmt.Println()
			fmt.Println(ui.WarningLine(fmt.Sprintf("Interrupt received (%s). Wrapping up tasks and shutting down cleanly...", sig)))
		}
		cancelExec()

		// Secondary interrupt: user wants immediate emergency exit
		<-sigChan
		if !jsonOutput {
			fmt.Println()
			fmt.Println(ui.ErrorLine("Second interrupt received. Exiting immediately."))
		}
		os.Exit(130)
	}()

	if watch {
		if !jsonOutput {
			fmt.Println(ui.Muted.Render("Watching for changes... (press Ctrl+C to stop)"))
		}
		for {
			watchExecCtx, cancelWatchExec := context.WithCancel(execCtx)
			changeChan := make(chan struct{}, 1)

			go func(currModel intelligence.ProjectModel) {
				changed, _ := intelligence.WatchForChanges(watchExecCtx, cwd, currModel, 500*time.Millisecond)
				if changed {
					select {
					case changeChan <- struct{}{}:
					default:
					}
					cancelWatchExec()
				}
			}(model)

			report := intelligence.ExecutePlanReportWithOptions(watchExecCtx, model, plan, resolver, env, intelligence.ExecutionOptions{Sandbox: sandbox})
			if !jsonOutput {
				printExecutionReport(report)
			}

			if report.Success {
				_ = intelligence.RecordVerifiedStrategies(cwd, model, plan, report, lock)
			}

			select {
			case <-changeChan:
				cancelWatchExec()
				if !jsonOutput {
					fmt.Println()
					fmt.Println(ui.InfoLine("↻ File change detected. Restarting application..."))
				}
				if m, err := intelligence.Analyze(cwd); err == nil {
					model = m
					if p, err := planner.Plan(execCtx, model); err == nil {
						plan = p
					}
				}
				continue
			case <-execCtx.Done():
				cancelWatchExec()
				if !jsonOutput && interrupted {
					fmt.Println(ui.SuccessLine("All services stopped. Wrap-up complete."))
				}
				return nil
			default:
				cancelWatchExec()
				if !jsonOutput {
					fmt.Println(ui.WarningLine("Process stopped. Waiting for file changes to restart..."))
				}
				changed, _ := intelligence.WatchForChanges(execCtx, cwd, model, 500*time.Millisecond)
				if changed {
					if m, err := intelligence.Analyze(cwd); err == nil {
						model = m
						if p, err := planner.Plan(execCtx, model); err == nil {
							plan = p
						}
					}
					continue
				}
				return nil
			}
		}
	}

	report := intelligence.ExecutePlanReportWithOptions(execCtx, model, plan, resolver, env, intelligence.ExecutionOptions{Silent: jsonOutput, Sandbox: sandbox})
	if jsonOutput {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		printExecutionReport(report)
	}

	if interrupted || execCtx.Err() != nil {
		if !jsonOutput {
			fmt.Println(ui.SuccessLine("All services stopped. Wrap-up complete."))
		}
		return nil
	}

	if !report.Success {
		return fmt.Errorf("intelligence execution failed: %s", report.FailureReason)
	}

	if err := intelligence.RecordVerifiedStrategies(cwd, model, plan, report, lock); err != nil {
		return fmt.Errorf("failed to update .octo.lock: %w", err)
	}
	if !jsonOutput {
		fmt.Println()
		fmt.Println(ui.SuccessStyle.Render("✓ Execution verified successfully."))
	}
	return nil
}



func printExecutionOverview(model intelligence.ProjectModel, plan intelligence.ExecutionPlan) {
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Repository"))
	fmt.Printf("  %s  %s\n", ui.Command(model.Name), ui.Muted.Render(fmt.Sprintf("%d component(s) · %d service(s)", len(model.Components), len(model.Services))))
	fmt.Println()
	fmt.Println(ui.HeadingStyle.Render("Execution plan"))
	fmt.Printf("  %s\n", ui.Muted.Render(fmt.Sprintf("%d step(s)", len(plan.Steps))))
	for _, step := range plan.Steps {
		label := string(step.Phase)
		if step.Command == "" {
			fmt.Printf("  %s %s\n", ui.Muted.Render("○"), ui.Muted.Render(label+" · "+step.ID))
		} else {
			fmt.Printf("  %s %s\n", ui.InfoLine("•"), label+" · "+ui.Command(step.Command))
		}
	}
	fmt.Println()
}

func printExecutionReport(report intelligence.ExecutionReport) {
	fmt.Println(ui.HeadingStyle.Render("Execution"))

	// A step can have several candidate attempts. Render one final line for the
	// step and explicitly call out fallback instead of printing contradictory
	// success/failure lines for the same step.
	latest := make(map[string]int)
	failures := make(map[string]int)
	for i, step := range report.Steps {
		latest[step.ID] = i
		if step.Status == intelligence.StepFailed {
			failures[step.ID]++
		}
	}

	for i, step := range report.Steps {
		if latest[step.ID] != i {
			continue
		}
		switch step.Status {
		case intelligence.StepSucceeded:
			label := step.ID
			if step.CandidateID != "" {
				label += " · " + step.CandidateID
			}
			if failures[step.ID] > 0 {
				fmt.Println(ui.SuccessLine(label + ui.Muted.Render(fmt.Sprintf(" · fallback succeeded after %d failed candidate(s)", failures[step.ID]))))
			} else {
				fmt.Println(ui.SuccessLine(label))
			}
		case intelligence.StepSkipped:
			fmt.Println(ui.Muted.Render("○ " + step.ID + " — " + step.Reason))
		case intelligence.StepFailed:
			fmt.Println(ui.ErrorLine(fmt.Sprintf("%s — %s", step.ID, step.Reason)))
		}
	}

	if len(report.Verification) > 0 {
		fmt.Println()
		fmt.Println(ui.HeadingStyle.Render("Verification"))
		for _, check := range report.Verification {
			if check.Passed {
				fmt.Println(ui.SuccessLine(check.CheckID))
			} else {
				fmt.Println(ui.ErrorLine(fmt.Sprintf("%s — %s", check.CheckID, check.Reason)))
			}
		}
	}

	if len(report.Decisions) > 0 {
		fmt.Println()
		fmt.Println(ui.HeadingStyle.Render("Decision trace"))
		for _, decision := range report.Decisions {
			switch decision.Outcome {
			case "succeeded":
				fmt.Println(ui.SuccessLine(fmt.Sprintf("%s → %s", decision.OptionID, ui.Command(decision.Value))))
			case "failed":
				fmt.Println(ui.ErrorLine(fmt.Sprintf("%s → candidate failed", decision.OptionID)))
			}
		}
	}
}
