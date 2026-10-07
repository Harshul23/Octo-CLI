package intelligence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecutePlanReportRecordsSuccessfulSteps(t *testing.T) {
	model := ProjectModel{Name: "demo", Confidence: 0.91}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{
			{ID: "component.demo.start", Component: "demo", NodeID: "component:demo", Phase: PhaseStart, Command: "true"},
		},
	}
	report := ExecutePlanReport(context.Background(), model, plan, RuntimeResolver{
		adapters: []RuntimeAdapter{&recordingAdapterForReport{}},
	}, ResolvedEnvironment{Values: map[string]string{}})

	if !report.Success {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Steps) != 1 || report.Steps[0].Status != StepSucceeded {
		t.Fatalf("steps=%+v", report.Steps)
	}
	if report.Steps[0].Adapter != "recording" {
		t.Fatalf("step=%+v", report.Steps)
	}
	if report.Confidence != 0.91 {
		t.Fatalf("confidence=%v", report.Confidence)
	}
}

func TestExecutePlanReportStopsOnFailure(t *testing.T) {
	model := ProjectModel{Name: "demo"}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{
			{ID: "component.demo.start", NodeID: "component:demo", Phase: PhaseStart, Command: "false"},
		},
	}
	report := ExecutePlanReport(context.Background(), model, plan, NewRuntimeResolver(), ResolvedEnvironment{Values: map[string]string{}})
	if report.Success {
		t.Fatal("expected failure")
	}
	if len(report.Steps) != 1 || report.Steps[0].Status != StepFailed {
		t.Fatalf("steps=%v", report.Steps)
	}
	if report.FailureReason == "" {
		t.Fatal("expected failure reason")
	}
}

func TestExecutePlanReportAppliesScopedEnvironmentPerStep(t *testing.T) {
	adapter := &recordingAdapterForReport{}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{
			{ID: "component.api.start", Component: "api", NodeID: "component:api", Phase: PhaseStart, Command: "true", WorkDir: "apps/api"},
			{ID: "component.web.start", Component: "web", NodeID: "component:web", Phase: PhaseStart, Command: "true", WorkDir: "apps/web"},
		},
	}
	env := ResolvedEnvironment{
		Values: map[string]string{"SHARED": "root"},
		ScopedValues: map[string]map[string]string{
			"apps/api": {"SHARED": "api", "API_ONLY": "api-value"},
			"apps/web": {"SHARED": "web", "WEB_ONLY": "web-value"},
		},
	}

	report := ExecutePlanReport(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
		adapters: []RuntimeAdapter{adapter},
	}, env)
	if !report.Success {
		t.Fatalf("report=%+v", report)
	}

	if got := adapter.seen["component.api.start"]["SHARED"]; got != "api" {
		t.Fatalf("api shared=%q, want api", got)
	}
	if got := adapter.seen["component.api.start"]["API_ONLY"]; got != "api-value" {
		t.Fatalf("api-only=%q, want api-value", got)
	}
	if _, ok := adapter.seen["component.api.start"]["WEB_ONLY"]; ok {
		t.Fatal("web-only variable leaked into api step")
	}

	if got := adapter.seen["component.web.start"]["SHARED"]; got != "web" {
		t.Fatalf("web shared=%q, want web", got)
	}
	if got := adapter.seen["component.web.start"]["WEB_ONLY"]; got != "web-value" {
		t.Fatalf("web-only=%q, want web-value", got)
	}
	if _, ok := adapter.seen["component.web.start"]["API_ONLY"]; ok {
		t.Fatal("api-only variable leaked into web step")
	}
}

type recordingAdapterForReport struct {
	seen map[string]map[string]string
}

func (a *recordingAdapterForReport) Name() string { return "recording" }
func (a *recordingAdapterForReport) Supports(step ExecutionStep) bool { return true }
func (a *recordingAdapterForReport) Execute(_ context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if a.seen == nil {
		a.seen = make(map[string]map[string]string)
	}
	values := make(map[string]string, len(env.Values))
	for key, value := range env.Values {
		values[key] = value
	}
	a.seen[step.ID] = values
	return nil
}


func TestExecutePlanReportFallsBackToNextEvidenceBackedCandidate(t *testing.T) {
	adapter := &fallbackAdapterForReport{}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{{
			ID: "component.demo.start", Component: "demo", NodeID: "component:demo", Phase: PhaseStart,
			Command: "bad-command", SelectedCandidate: "bad",
			Candidates: []ExecutionCandidate{
				{ID: "bad", Command: "bad-command", Confidence: 0.95},
				{ID: "good", Command: "good-command", Confidence: 0.80},
			},
		}},
	}
	report := ExecutePlanReport(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
		adapters: []RuntimeAdapter{adapter},
	}, ResolvedEnvironment{Values: map[string]string{}})
	if !report.Success {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Failures) != 1 || report.Failures[0].CandidateID != "bad" {
		t.Fatalf("failures=%+v", report.Failures)
	}
	if report.Failures[0].Classification != FailureClassMissingDependency {
		t.Fatalf("expected failure classification %q, got %q", FailureClassMissingDependency, report.Failures[0].Classification)
	}
	if len(report.Steps) != 2 || report.Steps[0].Status != StepFailed || report.Steps[1].Status != StepSucceeded {
		t.Fatalf("steps=%+v", report.Steps)
	}
	if report.Steps[1].CandidateID != "good" {
		t.Fatalf("steps=%+v", report.Steps)
	}
	if len(report.Decisions) != 2 {
		t.Fatalf("decisions=%+v", report.Decisions)
	}
	if report.Decisions[0].Outcome != "failed" || report.Decisions[1].Outcome != "succeeded" {
		t.Fatalf("decision outcomes=%+v", report.Decisions)
	}
}

type fallbackAdapterForReport struct{}

func (a *fallbackAdapterForReport) Name() string { return "fallback-test" }
func (a *fallbackAdapterForReport) Supports(step ExecutionStep) bool { return step.Command != "" }
func (a *fallbackAdapterForReport) Execute(_ context.Context, step ExecutionStep, _ ResolvedEnvironment) error {
	if step.Command == "bad-command" {
		return fmt.Errorf("candidate rejected: cannot find module 'express'")
	}
	return nil
}

type mockStartableAdapter struct {
	startedProcess *mockRunningProcess
}

func (m *mockStartableAdapter) Name() string { return "mock-startable" }
func (m *mockStartableAdapter) Supports(step ExecutionStep) bool { return step.Command != "" }
func (m *mockStartableAdapter) Execute(_ context.Context, step ExecutionStep, _ ResolvedEnvironment) error {
	if step.Command == "failing-command" {
		return fmt.Errorf("step execution failed")
	}
	return nil
}
func (m *mockStartableAdapter) Start(_ context.Context, step ExecutionStep, _ ResolvedEnvironment) (RunningProcess, error) {
	proc := &mockRunningProcess{pid: 4242}
	m.startedProcess = proc
	return proc, nil
}

type mockRunningProcess struct {
	pid     int
	stopped bool
}

func (p *mockRunningProcess) Wait() error { return nil }
func (p *mockRunningProcess) Stop() error {
	p.stopped = true
	return nil
}
func (p *mockRunningProcess) GracefulStop(_ time.Duration) error {
	return p.Stop()
}
func (p *mockRunningProcess) Pid() int { return p.pid }

func TestExecutePlanReportDetachedMode(t *testing.T) {
	adapter := &mockStartableAdapter{}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{{
			ID: "component.web.start", Component: "web", Phase: PhaseStart,
			Command: "run-server", LongRunning: true,
		}},
	}
	report := ExecutePlanReportWithOptions(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
		adapters: []RuntimeAdapter{adapter},
	}, ResolvedEnvironment{Values: map[string]string{}}, ExecutionOptions{Detach: true})

	if !report.Success {
		t.Fatalf("expected success, got failure: %s", report.FailureReason)
	}
	if len(report.ActiveProcesses) != 1 {
		t.Fatalf("active processes=%d, want 1", len(report.ActiveProcesses))
	}
	if report.ActiveProcesses[0].PID != 4242 {
		t.Fatalf("pid=%d, want 4242", report.ActiveProcesses[0].PID)
	}
	if adapter.startedProcess.stopped {
		t.Fatal("detached process should not be stopped after successful start")
	}

	tmp := t.TempDir()
	if err := SaveDetachedState(tmp, report); err != nil {
		t.Fatalf("SaveDetachedState failed: %v", err)
	}
}

func TestExecutePlanReportTeardownOnPartialFailure(t *testing.T) {
	adapter := &mockStartableAdapter{}
	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{
			{
				ID: "component.web.start", Component: "web", Phase: PhaseStart,
				Command: "run-server", LongRunning: true,
			},
			{
				ID: "component.worker.start", Component: "worker", Phase: PhaseStart,
				Command: "failing-command", LongRunning: false,
			},
		},
	}
	report := ExecutePlanReportWithOptions(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
		adapters: []RuntimeAdapter{adapter},
	}, ResolvedEnvironment{Values: map[string]string{}}, ExecutionOptions{})

	if report.Success {
		t.Fatal("expected execution to fail")
	}
	if !report.TeardownPerformed {
		t.Fatal("expected teardown to be performed on partial failure")
	}
	if adapter.startedProcess == nil || !adapter.startedProcess.stopped {
		t.Fatal("expected previously started background process to be stopped during teardown")
	}
}

func TestTeardownExecutionCleanWrapUp(t *testing.T) {
	tmp := t.TempDir()
	octoDir := filepath.Join(tmp, ".octo")
	if err := os.MkdirAll(octoDir, 0755); err != nil {
		t.Fatalf("failed to create .octo dir: %v", err)
	}
	procFile := filepath.Join(octoDir, "processes.json")
	if err := os.WriteFile(procFile, []byte(`{"demo": [1234]}`), 0644); err != nil {
		t.Fatalf("failed to write process file: %v", err)
	}

	proc1 := &mockRunningProcess{pid: 1001}
	proc2 := &mockRunningProcess{pid: 1002}
	running := []RunningProcess{proc1, proc2}

	model := ProjectModel{
		Name: "demo",
		Root: tmp,
	}
	plan := ExecutionPlan{ProjectName: "demo", Root: tmp}

	summary := TeardownExecution(model, plan, running, ExecutionOptions{})

	if !proc1.stopped {
		t.Fatal("expected proc1 to be stopped")
	}
	if !proc2.stopped {
		t.Fatal("expected proc2 to be stopped")
	}
	if summary.ProcessesStopped != 2 {
		t.Fatalf("expected 2 processes stopped, got %d", summary.ProcessesStopped)
	}
	if !summary.StateCleaned {
		t.Fatal("expected StateCleaned to be true")
	}
	if _, err := os.Stat(procFile); !os.IsNotExist(err) {
		t.Fatal("expected .octo/processes.json to be deleted")
	}
}


