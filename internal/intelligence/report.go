package intelligence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StepStatus describes the outcome of one planned execution step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepSkipped   StepStatus = "skipped"
	StepFailed    StepStatus = "failed"
)

// ExecutionStepResult is safe to serialize and contains no command output or environment values.
type ExecutionStepResult struct {
	ID          string     `json:"id" yaml:"id"`
	Status      StepStatus `json:"status" yaml:"status"`
	Adapter     string     `json:"adapter,omitempty" yaml:"adapter,omitempty"`
	CandidateID string     `json:"candidate_id,omitempty" yaml:"candidate_id,omitempty"`
	Reason      string     `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type ExecutionFailure struct {
	StepID         string                `json:"step_id" yaml:"step_id"`
	CandidateID    string                `json:"candidate_id,omitempty" yaml:"candidate_id,omitempty"`
	Classification FailureClassification `json:"classification,omitempty" yaml:"classification,omitempty"`
	Reason         string                `json:"reason" yaml:"reason"`
	Evidence       Evidence              `json:"evidence" yaml:"evidence"`
}

// ActiveProcess describes an active long-running process managed by Octo.
type ActiveProcess struct {
	StepID    string `json:"step_id" yaml:"step_id"`
	Component string `json:"component,omitempty" yaml:"component,omitempty"`
	PID       int    `json:"pid,omitempty" yaml:"pid,omitempty"`
	Port      int    `json:"port,omitempty" yaml:"port,omitempty"`
}

// ExecutionOptions configures execution behavior.
type ExecutionOptions struct {
	Detach            bool                               `json:"detach" yaml:"detach"`
	Silent            bool                               `json:"silent" yaml:"silent"`
	Sandbox           bool                               `json:"sandbox" yaml:"sandbox"`
	Args              []string                           `json:"args,omitempty" yaml:"args,omitempty"`
	OwnedServices     []string                           `json:"owned_services,omitempty" yaml:"owned_services,omitempty"`
	OnVerifiedStartup func(report ExecutionReport) error `json:"-" yaml:"-"`
}

// AppendPassthroughArgs appends trailing CLI arguments to a run command.
func AppendPassthroughArgs(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	escaped := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\n\"'") {
			escaped[i] = fmt.Sprintf("%q", a)
		} else {
			escaped[i] = a
		}
	}
	joined := strings.Join(escaped, " ")

	// If command is npm/pnpm/yarn start or run, append -- if not already present
	if (strings.HasPrefix(command, "npm run ") || strings.HasPrefix(command, "npm start") ||
		strings.HasPrefix(command, "pnpm run ") || strings.HasPrefix(command, "pnpm start")) &&
		!strings.Contains(command, " -- ") {
		return command + " -- " + joined
	}

	// If command is cargo run, append -- if not already present
	if strings.HasPrefix(command, "cargo run") && !strings.Contains(command, " -- ") {
		return command + " -- " + joined
	}

	return command + " " + joined
}

type executionContextKey string

const (
	silentExecutionKey   executionContextKey = "octo.silent"
	detachedExecutionKey executionContextKey = "octo.detach"
)

func withSilentExecution(ctx context.Context, silent bool) context.Context {
	return context.WithValue(ctx, silentExecutionKey, silent)
}

func isSilentExecution(ctx context.Context) bool {
	v, ok := ctx.Value(silentExecutionKey).(bool)
	return ok && v
}

func withDetachedExecution(ctx context.Context, detach bool) context.Context {
	return context.WithValue(ctx, detachedExecutionKey, detach)
}

func isDetachedExecution(ctx context.Context) bool {
	v, ok := ctx.Value(detachedExecutionKey).(bool)
	return ok && v
}

func isSandboxEnv() bool {
	v := strings.TrimSpace(os.Getenv("OCTO_SANDBOX"))
	return v == "1" || strings.EqualFold(v, "true")
}

// ExecutionReport is the canonical result of an intelligence execution.
type ExecutionReport struct {
	// runningProcesses retains ownership of live runtime handles in this process.
	// It is deliberately private: serialized reports contain only PID metadata.
	runningProcesses []RunningProcess

	ProjectName       string                `json:"project_name" yaml:"project_name"`
	Confidence        float64               `json:"confidence" yaml:"confidence"`
	Plan              ExecutionPlan         `json:"plan" yaml:"plan"`
	Steps             []ExecutionStepResult `json:"steps" yaml:"steps"`
	Verification      []VerificationResult  `json:"verification,omitempty" yaml:"verification,omitempty"`
	Failures          []ExecutionFailure    `json:"failures,omitempty" yaml:"failures,omitempty"`
	Decisions         []DecisionTraceEntry  `json:"decisions,omitempty" yaml:"decisions,omitempty"`
	ActiveProcesses   []ActiveProcess       `json:"active_processes,omitempty" yaml:"active_processes,omitempty"`
	OwnedServices     []string              `json:"owned_services,omitempty" yaml:"owned_services,omitempty"`
	Notes             []string              `json:"notes,omitempty" yaml:"notes,omitempty"`
	TeardownPerformed bool                  `json:"teardown_performed,omitempty" yaml:"teardown_performed,omitempty"`
	VerifiedStartup   bool                  `json:"verified_startup" yaml:"verified_startup"`
	Success           bool                  `json:"success" yaml:"success"`
	FailureReason     string                `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`
}

// VerifiedStartupResult is an immutable snapshot of verified startup state.
type VerifiedStartupResult struct {
	ProjectName     string                `json:"project_name" yaml:"project_name"`
	Confidence      float64               `json:"confidence" yaml:"confidence"`
	Plan            ExecutionPlan         `json:"plan" yaml:"plan"`
	Steps           []ExecutionStepResult `json:"steps" yaml:"steps"`
	Verification    []VerificationResult  `json:"verification,omitempty" yaml:"verification,omitempty"`
	Decisions       []DecisionTraceEntry  `json:"decisions,omitempty" yaml:"decisions,omitempty"`
	ActiveProcesses []ActiveProcess       `json:"active_processes,omitempty" yaml:"active_processes,omitempty"`
	OwnedServices   []string              `json:"owned_services,omitempty" yaml:"owned_services,omitempty"`
	Notes           []string              `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// StartupResult returns an immutable snapshot of the verified startup state.
func (r ExecutionReport) StartupResult() VerifiedStartupResult {
	steps := make([]ExecutionStepResult, len(r.Steps))
	copy(steps, r.Steps)
	verification := make([]VerificationResult, len(r.Verification))
	copy(verification, r.Verification)
	decisions := make([]DecisionTraceEntry, len(r.Decisions))
	copy(decisions, r.Decisions)
	procs := make([]ActiveProcess, len(r.ActiveProcesses))
	copy(procs, r.ActiveProcesses)
	owned := make([]string, len(r.OwnedServices))
	copy(owned, r.OwnedServices)
	notes := make([]string, len(r.Notes))
	copy(notes, r.Notes)
	return VerifiedStartupResult{
		ProjectName:     r.ProjectName,
		Confidence:      r.Confidence,
		Plan:            r.Plan,
		Steps:           steps,
		Verification:    verification,
		Decisions:       decisions,
		ActiveProcesses: procs,
		OwnedServices:   owned,
		Notes:           notes,
	}
}

func parseComposeUpCommand(command string) (file string, service string, ok bool) {
	parts := strings.Fields(command)
	if len(parts) >= 6 &&
		(parts[0] == "docker" || parts[0] == "podman") &&
		parts[1] == "compose" &&
		parts[2] == "-f" {
		filePath := parts[3]
		for i := 4; i < len(parts)-1; i++ {
			if parts[i] == "up" && parts[i+1] == "-d" && i+2 < len(parts) {
				return filePath, parts[i+2], true
			}
		}
	}
	return "", "", false
}

type composeRunState int

const (
	composeStateUnknown composeRunState = iota
	composeStateStopped
	composeStateRunning
)

// composeServiceRunState reports whether a Compose service is running.
// A failing `ps` query yields composeStateUnknown, never composeStateStopped.
func composeServiceRunState(composeFile, serviceName, workDir string) composeRunState {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "ps", "--status", "running", "-q", serviceName)
	if workDir != "" {
		cmd.Dir = workDir
	}
	out, err := cmd.Output()
	if err != nil {
		return composeStateUnknown
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return composeStateRunning
	}
	return composeStateStopped
}

func hasOwnedService(owned map[string]struct{}, name string) bool {
	_, ok := owned[name]
	return ok
}

const (
	defaultHealthInterval = 2 * time.Second
	defaultHealthRetries  = 10
	maxHealthInterval     = 30 * time.Second
)

// healthCheckForStep returns the parsed health check for a PhaseHealth step.
func healthCheckForStep(model ProjectModel, step ExecutionStep) *HealthCheck {
	if step.Phase != PhaseHealth {
		return nil
	}
	for _, svc := range model.Services {
		if svc.Name == step.Component && svc.HealthCheck != nil {
			return svc.HealthCheck
		}
	}
	return nil
}

// executeStep runs a step. Health steps use a bounded retry loop driven by the
// service's declared interval, retries and timeout so a cold service is not
// failed by a single immediate probe.
func executeStep(ctx context.Context, adapter RuntimeAdapter, step ExecutionStep, env ResolvedEnvironment, hc *HealthCheck) error {
	if hc == nil {
		return adapter.Execute(ctx, step, env)
	}
	interval := defaultHealthInterval
	if d, err := time.ParseDuration(strings.TrimSpace(hc.Interval)); err == nil && d > 0 {
		interval = d
	}
	if interval > maxHealthInterval {
		interval = maxHealthInterval
	}
	retries := hc.Retries
	if retries <= 0 {
		retries = defaultHealthRetries
	}
	var timeout time.Duration
	if d, err := time.ParseDuration(strings.TrimSpace(hc.Timeout)); err == nil && d > 0 {
		timeout = d
	}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		attemptCtx, cancel := ctx, context.CancelFunc(func() {})
		if timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		lastErr = adapter.Execute(attemptCtx, step, env)
		cancel()
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return lastErr
		}
	}
	return fmt.Errorf("health check %q still failing after %d attempts: %w", step.ID, retries, lastErr)
}

// ErrProcessExitedCleanly indicates that a process exited with exit status 0 during the stability window.
var ErrProcessExitedCleanly = errors.New("process exited cleanly with code 0")

// portlessStabilityWindow trades ~3s of startup latency for catching early crashes
// (e.g. late import errors, compilation failures, or immediate exit) across all port-less services.
const portlessStabilityWindow = 3 * time.Second

func verifyProcessLiveness(ctx context.Context, process RunningProcess, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-ticker.C:
			if la, ok := process.(interface{ IsAlive() bool }); ok {
				if !la.IsAlive() {
					err := process.Wait()
					if err == nil {
						return ErrProcessExitedCleanly
					}
					// shellProcess logs output to .octo/logs/<component>.log and exposes
					// it via LastOutput(). If available, include recent output so callers
					// and failure classifiers can detect root causes (e.g. port bind conflicts).
					var output string
					if lo, ok := process.(interface{ LastOutput() string }); ok {
						output = lo.LastOutput()
					}
					msg := fmt.Sprintf("process exited prematurely: %v", err)
					if output != "" {
						msg = fmt.Sprintf("%s\n%s", msg, output)
					}
					return errors.New(msg)
				}
			}
		}
	}
}

// ExecutePlanReport executes a plan with default options and returns a serializable execution report.
func ExecutePlanReport(ctx context.Context, model ProjectModel, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment) ExecutionReport {
	return ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, ExecutionOptions{})
}

// ExecutePlanReportWithOptions executes a plan with specific options (such as detached mode).
func ExecutePlanReportWithOptions(ctx context.Context, model ProjectModel, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment, opts ExecutionOptions) (report ExecutionReport) {
	if resolver.adapters == nil {
		sandbox := opts.Sandbox || isSandboxEnv()
		resolver = NewRuntimeResolverWithOptions(RuntimeResolverOptions{
			Sandbox:        sandbox,
			Root:           model.Root,
			Language:       model.Language,
			RuntimeVersion: model.RuntimeVersion,
		})
	}
	if opts.Silent {
		ctx = withSilentExecution(ctx, true)
	}
	if opts.Detach {
		ctx = withDetachedExecution(ctx, true)
	}
	report = ExecutionReport{
		ProjectName: model.Name,
		Confidence:  model.Confidence,
		Plan:        plan,
		Success:     false,
		Steps:       make([]ExecutionStepResult, 0, len(plan.Steps)),
	}
	var running []RunningProcess
	ownedServicesMap := make(map[string]struct{})
	defer func() {
		if !report.Success || ctx.Err() != nil {
			if len(running) > 0 || len(ownedServicesMap) > 0 {
				teardownOpts := opts
				teardownOpts.OwnedServices = make([]string, 0, len(ownedServicesMap))
				for svc := range ownedServicesMap {
					teardownOpts.OwnedServices = append(teardownOpts.OwnedServices, svc)
				}
				_ = TeardownExecution(model, plan, running, teardownOpts)
				report.TeardownPerformed = true
			}
		}
	}()

	if err := ValidateExecutionPlan(plan); err != nil {
		report.FailureReason = fmt.Sprintf("invalid execution plan: %v", err)
		return report
	}

	order, err := TopologicalOrder(plan)
	if err != nil {
		report.FailureReason = err.Error()
		return report
	}

	steps := make(map[string]ExecutionStep, len(plan.Steps))
	for _, step := range plan.Steps {
		steps[step.ID] = step
	}

	for _, id := range order {
		if err := ctx.Err(); err != nil {
			report.FailureReason = err.Error()
			report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepFailed, Reason: err.Error()})
			return report
		}

		step := steps[id]
		if step.Command == "" {
			report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepSkipped, Reason: "no executable command"})
			continue
		}

		executed := false
		candidates := executionCandidatesForStep(step)
		for _, candidate := range candidates {
			attempt := step
			attempt.Command = candidate.Command
			if attempt.Phase == PhaseStart && len(opts.Args) > 0 {
				attempt.Command = AppendPassthroughArgs(attempt.Command, opts.Args)
			}
			report.Decisions = append(report.Decisions, DecisionTraceEntry{
				Name: "execution." + id, OptionID: candidate.ID, Value: candidate.Command,
				Confidence: candidate.Confidence, Evidence: candidate.Evidence, Outcome: "selected",
				Reason: "Selected from the bounded evidence-backed candidate set.",
			})
			adapter, err := resolver.Resolve(attempt)
			if err != nil {
				report.FailureReason = err.Error()
				report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepFailed, CandidateID: candidate.ID, Reason: err.Error()})
				return report
			}

			composeFile, composeSvc, isComposeStart := parseComposeUpCommand(attempt.Command)
			// Ownership is claimed only when we positively know the service was
			// not running before. An unknown state is never treated as "not
			// running", so cleanup cannot stop a service this run did not start.
			canClaimOwnership := false
			if isComposeStart {
				state := composeServiceRunState(composeFile, composeSvc, attempt.WorkDir)
				canClaimOwnership = state == composeStateStopped
			}

			result := ExecutionStepResult{ID: id, Status: StepRunning, Adapter: adapter.Name(), CandidateID: candidate.ID}
			// Resolve the environment at the execution boundary so component-local
			// values are visible only to steps belonging to that component.
			stepEnv := env.ForStep(attempt)
			// In detach mode, start steps run asynchronously only when the adapter
			// can start them. Adapters that only implement Execute (Compose
			// `up -d` already returns immediately) run synchronously.
			_, adapterStartable := adapter.(StartableRuntimeAdapter)
			isStartable := attempt.LongRunning || (opts.Detach && attempt.Phase == PhaseStart && adapterStartable)
			if isStartable {
				startable, ok := adapter.(StartableRuntimeAdapter)
				if !ok {
					err := fmt.Errorf("runtime adapter %q cannot start long-running step %q", adapter.Name(), id)
					result.Status = StepFailed
					result.Reason = err.Error()
					report.Steps = append(report.Steps, result)
					classification := ClassifyExecutionFailure(err.Error(), err)
					report.Failures = append(report.Failures, ExecutionFailure{
						StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: err.Error(),
						Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: fmt.Sprintf("[%s] %s", classification, err.Error()), Strength: 1},
					})
					report.FailureReason = err.Error()
					continue
				}
				process, err := startable.Start(ctx, attempt, stepEnv)
				if err != nil {
					result.Status = StepFailed
					result.Reason = err.Error()
					report.Steps = append(report.Steps, result)
					classification := ClassifyExecutionFailure(err.Error(), err)
					detail := fmt.Sprintf("[%s] Execution candidate failed to start: %s", classification, err.Error())
					if classification == FailureClassMissingArguments {
						detail = fmt.Sprintf("[%s] Application appears to require arguments. Pass them using: octo run -- [args...]", classification)
					}
					report.Failures = append(report.Failures, ExecutionFailure{
						StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: err.Error(),
						Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: detail, Strength: 1},
					})
					report.FailureReason = err.Error()
					continue
				}
				port := portForComponent(plan.Ports, attempt.Component)
				cleanExited := false
				if port > 0 {
					if err := verifyPortWithProcess(ctx, "127.0.0.1", port, 5*time.Second, process); err != nil {
						_ = process.Stop()
						_ = process.Wait()
						result.Status = StepFailed
						result.Reason = err.Error()
						for i := len(report.Decisions) - 1; i >= 0; i-- {
							if report.Decisions[i].Name == "execution."+id && report.Decisions[i].OptionID == candidate.ID {
								report.Decisions[i].Outcome = "failed"
								break
							}
						}
						report.Steps = append(report.Steps, result)
						classification := ClassifyExecutionFailure("Startup verification failed: "+err.Error(), err)
						report.Failures = append(report.Failures, ExecutionFailure{
							StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: err.Error(),
							Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: fmt.Sprintf("[%s] Startup verification failed: %s", classification, err.Error()), Strength: 1},
						})
						report.FailureReason = err.Error()
						continue
					}
				} else if attempt.LongRunning {
					livenessErr := verifyProcessLiveness(ctx, process, portlessStabilityWindow)
					if errors.Is(livenessErr, ErrProcessExitedCleanly) {
						cleanExited = true
						compName := attempt.Component
						if compName == "" {
							compName = id
						}
						report.Notes = append(report.Notes, fmt.Sprintf("note: %s exited cleanly (code 0) during the startup window; treating it as a finished command, not a server.", compName))
					} else if livenessErr != nil {
						_ = process.Stop()
						_ = process.Wait()
						result.Status = StepFailed
						result.Reason = livenessErr.Error()
						for i := len(report.Decisions) - 1; i >= 0; i-- {
							if report.Decisions[i].Name == "execution."+id && report.Decisions[i].OptionID == candidate.ID {
								report.Decisions[i].Outcome = "failed"
								break
							}
						}
						report.Steps = append(report.Steps, result)
						classification := ClassifyExecutionFailure("Startup verification failed: "+livenessErr.Error(), livenessErr)
						report.Failures = append(report.Failures, ExecutionFailure{
							StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: livenessErr.Error(),
							Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: fmt.Sprintf("[%s] Startup verification failed: %s", classification, livenessErr.Error()), Strength: 1},
						})
						report.FailureReason = livenessErr.Error()
						continue
					}
				} else {
					// One-shot portless executable: if it already completed, check if it exited cleanly with code 0.
					if la, ok := process.(interface{ IsAlive() bool }); ok && !la.IsAlive() {
						if err := process.Wait(); err != nil {
							result.Status = StepFailed
							result.Reason = err.Error()
							for i := len(report.Decisions) - 1; i >= 0; i-- {
								if report.Decisions[i].Name == "execution."+id && report.Decisions[i].OptionID == candidate.ID {
									report.Decisions[i].Outcome = "failed"
									break
								}
							}
							report.Steps = append(report.Steps, result)
							classification := ClassifyExecutionFailure("Execution failed: "+err.Error(), err)
							report.Failures = append(report.Failures, ExecutionFailure{
								StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: err.Error(),
								Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: fmt.Sprintf("[%s] Execution failed: %s", classification, err.Error()), Strength: 1},
							})
							report.FailureReason = err.Error()
							continue
						}
					}
				}
				if !cleanExited {
					if la, ok := process.(interface{ IsAlive() bool }); !ok || la.IsAlive() {
						running = append(running, process)
						report.runningProcesses = append(report.runningProcesses, process)
						pid := 0
						if p, ok := process.(interface{ Pid() int }); ok {
							pid = p.Pid()
						}
						report.ActiveProcesses = append(report.ActiveProcesses, ActiveProcess{
							StepID:    id,
							Component: attempt.Component,
							PID:       pid,
							Port:      port,
						})
					}
				}
			} else if err := executeStep(ctx, adapter, attempt, stepEnv, healthCheckForStep(model, attempt)); err != nil {
				if isComposeStart && canClaimOwnership && !hasOwnedService(ownedServicesMap, composeSvc) {
					// Partial failure: record the service if Compose actually started it.
					if composeServiceRunState(composeFile, composeSvc, attempt.WorkDir) == composeStateRunning {
						ownedServicesMap[composeSvc] = struct{}{}
						report.OwnedServices = append(report.OwnedServices, composeSvc)
					}
				}
				result.Status = StepFailed
				result.Reason = err.Error()
				for i := len(report.Decisions) - 1; i >= 0; i-- {
					if report.Decisions[i].Name == "execution."+id && report.Decisions[i].OptionID == candidate.ID {
						report.Decisions[i].Outcome = "failed"
						break
					}
				}
				report.Steps = append(report.Steps, result)
				classification := ClassifyExecutionFailure(err.Error(), err)
				detail := fmt.Sprintf("[%s] Execution candidate failed: %s", classification, err.Error())
				if classification == FailureClassMissingArguments {
					detail = fmt.Sprintf("[%s] Application appears to require arguments. Pass them using: octo run -- [args...]", classification)
				}
				report.Failures = append(report.Failures, ExecutionFailure{
					StepID: id, CandidateID: candidate.ID, Classification: classification, Reason: err.Error(),
					Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: detail, Strength: 1},
				})
				report.FailureReason = err.Error()
				continue
			}
			if isComposeStart && canClaimOwnership {
				ownedServicesMap[composeSvc] = struct{}{}
				report.OwnedServices = append(report.OwnedServices, composeSvc)
			}
			result.Status = StepSucceeded
			for i := len(report.Decisions) - 1; i >= 0; i-- {
				if report.Decisions[i].Name == "execution."+id && report.Decisions[i].OptionID == candidate.ID {
					report.Decisions[i].Outcome = "succeeded"
					break
				}
			}
			report.Steps = append(report.Steps, result)
			executed = true
			break
		}
		if !executed {
			return report
		}
	}

	checks := BuildVerificationChecks(model, plan)
	verification, err := Verify(ctx, checks)
	report.Verification = verification
	if err != nil {
		report.FailureReason = err.Error()
		return report
	}
	for _, res := range verification {
		if !res.Passed {
			report.FailureReason = fmt.Sprintf("verification %q failed: %s", res.CheckID, res.Reason)
			return report
		}
	}

	// Verified-startup checkpoint:
	// Every planned startup/setup step and all deterministic verification checks have passed.
	report.VerifiedStartup = true

	if opts.OnVerifiedStartup != nil {
		checkpointReport := report
		checkpointReport.Steps = append([]ExecutionStepResult(nil), report.Steps...)
		checkpointReport.Verification = append([]VerificationResult(nil), report.Verification...)
		checkpointReport.Decisions = append([]DecisionTraceEntry(nil), report.Decisions...)
		checkpointReport.ActiveProcesses = append([]ActiveProcess(nil), report.ActiveProcesses...)
		checkpointReport.OwnedServices = append([]string(nil), report.OwnedServices...)
		checkpointReport.Notes = append([]string(nil), report.Notes...)

		if err := opts.OnVerifiedStartup(checkpointReport); err != nil {
			report.FailureReason = fmt.Sprintf("verified-startup hook failed: %v", err)
			return report
		}
	}

	if !opts.Detach && len(running) > 0 {
		if err := waitForRunningProcesses(ctx, running); err != nil {
			report.FailureReason = err.Error()
			return report
		}
	}

	report.Success = true
	report.FailureReason = ""
	return report
}

// SaveDetachedState writes active detached process information to .octo/processes.json.
func SaveDetachedState(root string, report ExecutionReport) error {
	if len(report.ActiveProcesses) == 0 {
		return nil
	}
	dir := filepath.Join(root, ".octo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report.ActiveProcesses, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "processes.json"), data, 0o644)
}

func portForComponent(ports []PortAssignment, component string) int {
	for _, port := range ports {
		if port.Component == component {
			return port.Resolved
		}
	}
	return 0
}

func waitForRunningProcesses(ctx context.Context, processes []RunningProcess) error {
	done := make(chan error, len(processes))
	for _, process := range processes {
		go func(process RunningProcess) {
			done <- process.Wait()
		}(process)
	}
	select {
	case err := <-done:
		for _, process := range processes {
			_ = process.GracefulStop(2 * time.Second)
		}
		if err != nil {
			return fmt.Errorf("long-running process exited: %w", err)
		}
		return fmt.Errorf("long-running process exited unexpectedly")
	case <-ctx.Done():
		for _, process := range processes {
			_ = process.GracefulStop(3 * time.Second)
		}
		return ctx.Err()
	}
}

// ExecutePlan remains a small compatibility wrapper for callers that only need an error.
func ExecutePlan(ctx context.Context, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment) error {
	report := ExecutePlanReport(ctx, ProjectModel{Name: plan.ProjectName, Root: plan.Root}, plan, resolver, env)
	if !report.Success {
		return fmt.Errorf("%s", report.FailureReason)
	}
	return nil
}

func executionCandidatesForStep(step ExecutionStep) []ExecutionCandidate {
	if len(step.Candidates) == 0 {
		if step.Command == "" {
			return nil
		}
		return []ExecutionCandidate{{ID: step.SelectedCandidate, Command: step.Command}}
	}
	ordered := make([]ExecutionCandidate, 0, len(step.Candidates))
	for _, candidate := range step.Candidates {
		if candidate.ID == step.SelectedCandidate || (step.SelectedCandidate == "" && candidate.Command == step.Command) {
			ordered = append(ordered, candidate)
		}
	}
	for _, candidate := range step.Candidates {
		if candidate.ID == step.SelectedCandidate || (step.SelectedCandidate == "" && candidate.Command == step.Command) {
			continue
		}
		ordered = append(ordered, candidate)
	}
	return ordered
}

func stopComposeServices(services []Service, root string, ownedServices []string) int {
	if len(services) == 0 {
		return 0
	}
	var ownedFilter map[string]struct{}
	if ownedServices != nil {
		ownedFilter = make(map[string]struct{}, len(ownedServices))
		for _, name := range ownedServices {
			ownedFilter[name] = struct{}{}
		}
	}

	stopped := 0
	for _, svc := range services {
		if ownedFilter != nil {
			if _, ok := ownedFilter[svc.Name]; !ok {
				continue
			}
		}
		for _, ev := range svc.Evidence {
			if ev.Kind == EvidenceConfig && (strings.HasSuffix(ev.Path, "compose.yml") || strings.HasSuffix(ev.Path, "compose.yaml") || strings.HasSuffix(ev.Path, "docker-compose.yml") || strings.HasSuffix(ev.Path, "docker-compose.yaml")) {
				cmd := exec.Command("docker", "compose", "-f", ev.Path, "stop", svc.Name)
				cmd.Dir = root
				if err := cmd.Run(); err == nil {
					stopped++
				}
				break
			}
		}
	}
	return stopped
}

// TeardownSummary records completed cleanup tasks during shutdown.
type TeardownSummary struct {
	ProcessesStopped int  `json:"processes_stopped"`
	ServicesStopped  int  `json:"services_stopped"`
	StateCleaned     bool `json:"state_cleaned"`
}

// TeardownExecution performs a graceful shutdown of all active processes, backing services, and temporary state.
func TeardownExecution(model ProjectModel, plan ExecutionPlan, running []RunningProcess, opts ExecutionOptions) TeardownSummary {
	summary := TeardownSummary{}

	// 1. Gracefully stop all application processes
	for _, process := range running {
		if err := process.GracefulStop(3 * time.Second); err == nil {
			summary.ProcessesStopped++
		} else {
			_ = process.Stop()
			summary.ProcessesStopped++
		}
	}

	// 2. Stop backing Docker Compose services owned by this execution
	// A detached run keeps healthy services running, but a failed detached
	// run passes OwnedServices so the services it started are stopped.
	if len(model.Services) > 0 && (!opts.Detach || opts.OwnedServices != nil) {
		summary.ServicesStopped = stopComposeServices(model.Services, model.Root, opts.OwnedServices)
	}

	// 3. Clean up non-detached process state file
	if !opts.Detach {
		path := filepath.Join(model.Root, ".octo", "processes.json")
		if _, err := os.Stat(path); err == nil {
			_ = os.Remove(path)
			summary.StateCleaned = true
		}
	}

	return summary
}

// StopActiveProcesses stops and reaps processes owned by this live report.
// Killing a saved PID alone bypasses adapter-specific process-tree termination
// and cannot wait for the original command's log handles to close.
func StopActiveProcesses(report ExecutionReport) error {
	if len(report.runningProcesses) == 0 && len(report.ActiveProcesses) > 0 {
		return fmt.Errorf("cannot safely stop processes from a serialized report: runtime handles are unavailable")
	}
	var errs []error
	for _, process := range report.runningProcesses {
		if err := process.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop process %d: %w", process.Pid(), err))
			continue
		}
		// A killed command normally returns an exit-status error. The important
		// guarantee here is that Wait has completed, including output/log cleanup.
		_ = process.Wait()
	}
	return errors.Join(errs...)
}
