package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
	"github.com/Octo-sh/Octo-CLI/internal/ui"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:     "run [path]",
	Aliases: []string{"r", "start"},
	Short:   "Understand and run the repository locally",
	Args: func(cmd *cobra.Command, args []string) error {
		dashAt := cmd.ArgsLenAtDash()
		if dashAt == -1 {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 path argument. To pass arguments to the application, use: octo run [path] -- [args...]")
			}
			return nil
		}
		if dashAt > 1 {
			return fmt.Errorf("accepts at most 1 path argument before '--'")
		}
		return nil
	},
	SilenceUsage: true,
	Long: `The run command analyzes the repository, builds an execution plan,
resolves the required environment, executes the plan, and verifies the
result.

Target Selection:
  Use --only <target> (repeatable, e.g. --only component:web --only service:postgres)
  or --all to specify which components and services to execute.
  --only and --all are mutually exclusive.

  In non-interactive environments (--json, --no-tui, --detach, or when stdin is not
  a TTY), repositories with multiple runnable targets require either --only or --all.
  If neither is specified, octo run lists available targets and exits with an error
  rather than starting everything silently or hanging for input. Single-target repositories
  continue to run automatically without prompting.`,
	RunE: runRun,
}

func init() {
	// Add flags specific to the run command
	runCmd.Flags().BoolP("watch", "w", false, "Watch for file changes and restart")
	runCmd.Flags().BoolP("detach", "d", false, "Run in detached mode (background)")
	runCmd.Flags().IntP("port", "p", 0, "Override the port to run on (0 = use config default)")
	runCmd.Flags().BoolP("no-port-shift", "N", false, "Disable automatic port shifting on conflicts")
	runCmd.Flags().BoolP("skip-env-check", "k", false, "Skip environment variable validation")
	runCmd.Flags().Bool("skip-env", false, "Alias for --skip-env-check")
	runCmd.Flags().BoolP("no-tui", "T", false, "Disable TUI dashboard (use plain scrolling output)")
	runCmd.Flags().BoolP("sandbox", "s", false, "Execute application inside an ephemeral container (Docker/Podman) for environment isolation")
	runCmd.Flags().StringArrayP("only", "o", nil, "Run only specific targets (e.g. --only component:web --only service:postgres); repeatable, mutually exclusive with --all")
	runCmd.Flags().Bool("all", false, "Run all discovered components and services without prompting; mutually exclusive with --only")
	runCmd.Flags().BoolP("json", "j", false, "Output the ExecutionReport as JSON")
	runCmd.Flags().Bool("dry-run", false, "Preview machine changes, ports, and execution impact without running (alias for octo preview)")

	_ = runCmd.Flags().MarkHidden("only")
	_ = runCmd.Flags().MarkHidden("all")
}

func runRun(cmd *cobra.Command, args []string) error {
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return previewCmd.RunE(cmd, args)
	}
	return runWithIntelligence(cmd, args)
}

func runWithIntelligence(cmd *cobra.Command, args []string) error {
	target := "."
	var passthroughArgs []string
	dashAt := cmd.ArgsLenAtDash()
	if dashAt != -1 {
		if dashAt == 1 {
			target = args[0]
		}
		passthroughArgs = args[dashAt:]
	} else if len(args) == 1 {
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

	onlyFlags, _ := cmd.Flags().GetStringArray("only")
	allFlag, _ := cmd.Flags().GetBool("all")
	noTUI, _ := cmd.Flags().GetBool("no-tui")
	noPortShift, _ := cmd.Flags().GetBool("no-port-shift")
	portOverride, _ := cmd.Flags().GetInt("port")

	if len(onlyFlags) > 0 && allFlag {
		return fmt.Errorf("cannot combine --only and --all; choose either specific targets with --only or use --all")
	}

	var selection intelligence.ResolvedSelection
	if len(onlyFlags) > 0 || allFlag {
		sel, err := intelligence.ResolveSelection(model, intelligence.SelectionOptions{
			Only: onlyFlags,
			All:  allFlag,
		})
		if err != nil {
			return selectionFailure(jsonOutput, "invalid_selection", err, model)
		}
		selection = sel
	} else {
		runnable := intelligence.RunnableTargets(model)
		isInteractive := !noTUI && !jsonOutput && !detach && ui.InteractiveTerminal()

		if len(runnable) <= 1 {
			sel, err := intelligence.ResolveSelection(model, intelligence.SelectionOptions{})
			if err != nil {
				return err
			}
			selection = sel
		} else if isInteractive {
			sel, cancelled, err := ui.RunTargetSelector(model)
			if err != nil {
				return err
			}
			if cancelled {
				return fmt.Errorf("execution cancelled by user")
			}
			selection = sel
		} else {
			return selectionFailure(jsonOutput, "selection_required", fmt.Errorf("multiple runnable targets detected; specify --only <target> or --all in non-interactive mode.\nAvailable targets:\n%s", intelligence.FormatAvailableTargets(model)), model)
		}
	}

	model = selection.FilteredModel
	if noPortShift {
		for i := range model.Components {
			model.Components[i].PortStrict = true
		}
	}
	if portOverride > 0 && len(model.Components) == 1 {
		model.Components[0].Port = portOverride
		if noPortShift {
			model.Components[0].PortStrict = true
		}
	}
	requestedIdentities := selection.RequestedTargets

	if len(selection.ForcedDependencies) > 0 && !jsonOutput {
		var depMsgs []string
		for depID, reason := range selection.ForcedDependencies {
			depMsgs = append(depMsgs, fmt.Sprintf("%s (%s)", depID.String(), reason))
		}
		sort.Strings(depMsgs)
		fmt.Println(ui.InfoLine(fmt.Sprintf("Included dependencies: %s", strings.Join(depMsgs, ", "))))
	}

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

	skipEnvCheck, _ := cmd.Flags().GetBool("skip-env-check")
	if !skipEnvCheck {
		skipEnvCheck, _ = cmd.Flags().GetBool("skip-env")
	}
	envOpts := intelligence.ResolveEnvironmentOptions{SkipRequiredCheck: skipEnvCheck}
	env, err := intelligence.ResolveProjectEnvironmentWithOptions(cwd, model, envOpts)
	if err != nil {
		var missingErr *intelligence.MissingRequiredEnvError
		if errors.As(err, &missingErr) {
			if !jsonOutput && ui.InteractiveTerminal() {
				tpl := intelligence.GenerateEnvTemplate(cwd, model)
				res, promptErr := ui.PromptMissingEnvironment(cwd, missingErr.Missing, tpl)
				if promptErr != nil {
					return promptErr
				}
				if !res.ShouldContinue {
					return nil
				}
				if res.Skipped || len(res.EnteredValues) > 0 {
					env, err = intelligence.ResolveProjectEnvironmentWithOptions(cwd, model, intelligence.ResolveEnvironmentOptions{SkipRequiredCheck: true})
					if err != nil {
						return fmt.Errorf("environment resolution failed: %w", err)
					}
					if env.Values == nil {
						env.Values = make(map[string]string)
					}
					for k, v := range res.EnteredValues {
						env.Values[k] = v
					}
				}
			} else {
				return fmt.Errorf("%w. Pass --skip-env (or -k) to bypass or run 'octo env template' to generate a template", err)
			}
		} else {
			return fmt.Errorf("environment resolution failed: %w", err)
		}
	}

	resolver := intelligence.NewRuntimeResolverWithOptions(intelligence.RuntimeResolverOptions{
		Sandbox: sandbox,
		Root:    cwd,
	})

	if detach {
		var lockErr error
		opts := intelligence.ExecutionOptions{
			Detach:  true,
			Silent:  jsonOutput,
			Sandbox: sandbox,
			Args:    passthroughArgs,
			OnVerifiedStartup: func(rep intelligence.ExecutionReport) error {
				updatedLock, err := intelligence.RecordVerifiedStrategiesWithLock(cwd, model, plan, rep, lock)
				if err != nil {
					lockErr = err
					return err
				}
				lock = updatedLock
				return nil
			},
		}
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

		if lockErr != nil {
			return fmt.Errorf("failed to update .octo.lock: %w", lockErr)
		}

		if !report.Success {
			return fmt.Errorf("intelligence execution failed: %s", report.FailureReason)
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
	sigDone := make(chan struct{})
	defer func() {
		close(sigDone)
		signal.Stop(sigChan)
	}()

	execCtx, cancelExec := context.WithCancel(cmd.Context())
	defer cancelExec()

	var interrupted atomic.Bool
	go func() {
		select {
		case sig, ok := <-sigChan:
			if !ok {
				return
			}
			interrupted.Store(true)
			if !jsonOutput {
				fmt.Println()
				fmt.Println(ui.WarningLine(fmt.Sprintf("Interrupt received (%s). Wrapping up tasks and shutting down cleanly...", sig)))
			}
			cancelExec()

			// Secondary interrupt: user wants immediate emergency exit
			select {
			case <-sigChan:
				if !jsonOutput {
					fmt.Println()
					fmt.Println(ui.ErrorLine("Second interrupt received. Exiting immediately."))
				}
				os.Exit(130)
			case <-sigDone:
				return
			}
		case <-sigDone:
			return
		}
	}()

	if watch {
		if !jsonOutput {
			fmt.Println(ui.Muted.Render("Watching for changes... (press Ctrl+C to stop)"))
		}
		reanalyzeWatch := func() error {
			newModel, err := intelligence.Analyze(cwd)
			if err != nil {
				if !jsonOutput {
					fmt.Println(ui.WarningLine(fmt.Sprintf("Re-analysis warning: %v", err)))
				}
				return nil
			}
			newSelection, err := intelligence.ResolveSelectionTargets(newModel, requestedIdentities)
			if err != nil {
				return fmt.Errorf("watch stopped: %w", err)
			}
			newPlan, err := planner.Plan(execCtx, newSelection.FilteredModel)
			if err != nil {
				return fmt.Errorf("watch planning failed: %w", err)
			}
			newEnv, err := intelligence.ResolveProjectEnvironmentWithOptions(cwd, newSelection.FilteredModel, envOpts)
			if err != nil {
				return fmt.Errorf("watch environment resolution failed: %w", err)
			}
			model = newSelection.FilteredModel
			plan = newPlan
			env = newEnv
			return nil
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

			var lockErr error
			watchOpts := intelligence.ExecutionOptions{
				Sandbox: sandbox,
				Args:    passthroughArgs,
				OnVerifiedStartup: func(rep intelligence.ExecutionReport) error {
					updatedLock, err := intelligence.RecordVerifiedStrategiesWithLock(cwd, model, plan, rep, lock)
					if err != nil {
						lockErr = err
						return err
					}
					lock = updatedLock
					return nil
				},
			}

			report := intelligence.ExecutePlanReportWithOptions(watchExecCtx, model, plan, resolver, env, watchOpts)
			if !jsonOutput {
				printExecutionReport(report)
			}

			if lockErr != nil {
				cancelWatchExec()
				return fmt.Errorf("failed to update .octo.lock: %w", lockErr)
			}

			select {
			case <-changeChan:
				cancelWatchExec()
				if !jsonOutput {
					fmt.Println()
					fmt.Println(ui.InfoLine("↻ File change detected. Restarting application..."))
				}
				if err := reanalyzeWatch(); err != nil {
					return err
				}
				continue
			case <-execCtx.Done():
				cancelWatchExec()
				if !jsonOutput && interrupted.Load() {
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
					if err := reanalyzeWatch(); err != nil {
						return err
					}
					continue
				}
				return nil
			}
		}
	}

	var lockErr error
	foregroundOpts := intelligence.ExecutionOptions{
		Silent:  jsonOutput,
		Sandbox: sandbox,
		Args:    passthroughArgs,
		OnVerifiedStartup: func(rep intelligence.ExecutionReport) error {
			updatedLock, err := intelligence.RecordVerifiedStrategiesWithLock(cwd, model, plan, rep, lock)
			if err != nil {
				lockErr = err
				return err
			}
			lock = updatedLock
			return nil
		},
	}

	report := intelligence.ExecutePlanReportWithOptions(execCtx, model, plan, resolver, env, foregroundOpts)
	if jsonOutput {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		printExecutionReport(report)
	}

	if lockErr != nil {
		return fmt.Errorf("failed to update .octo.lock: %w", lockErr)
	}

	if interrupted.Load() || execCtx.Err() != nil {
		if !jsonOutput {
			fmt.Println(ui.SuccessLine("All services stopped. Wrap-up complete."))
		}
		return nil
	}

	if !report.Success {
		return fmt.Errorf("intelligence execution failed: %s", report.FailureReason)
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

	if report.Success || report.VerifiedStartup {
		seenPortless := make(map[string]bool)
		for _, proc := range report.ActiveProcesses {
			if proc.Port == 0 {
				name := proc.Component
				if name == "" {
					name = proc.StepID
				}
				if !seenPortless[name] {
					seenPortless[name] = true
					fmt.Println(ui.Muted.Render(fmt.Sprintf("note: no port was discoverable for %s; startup verification means the process stayed alive, not that it responds to HTTP.", name)))
				}
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

// reportedError is an error whose details were already written to stdout as
// structured JSON; main must not print it again.
type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

type selectionErrorJSON struct {
	Success bool               `json:"success"`
	Error   selectionErrorBody `json:"error"`
}

type selectionErrorBody struct {
	Code             string   `json:"code"`
	Message          string   `json:"message"`
	AvailableTargets []string `json:"available_targets,omitempty"`
}

// selectionFailure returns err unchanged for human output. With --json it
// writes a structured error object to stdout and returns a reportedError so the
// exit code stays non-zero without extra plain-text noise.
func selectionFailure(jsonOutput bool, code string, err error, model intelligence.ProjectModel) error {
	if !jsonOutput {
		return err
	}
	body := selectionErrorBody{Code: code, Message: err.Error()}
	for _, target := range intelligence.RunnableTargets(model) {
		body.AvailableTargets = append(body.AvailableTargets, target.ID.String())
	}
	data, marshalErr := json.MarshalIndent(selectionErrorJSON{Success: false, Error: body}, "", "  ")
	if marshalErr != nil {
		return err
	}
	fmt.Println(string(data))
	return reportedError{err: err}
}
