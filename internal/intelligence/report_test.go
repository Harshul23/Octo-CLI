package intelligence

import (
	"context"
	"fmt"
	"testing"
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
	if len(report.Steps) != 2 || report.Steps[0].Status != StepFailed || report.Steps[1].Status != StepSucceeded {
		t.Fatalf("steps=%+v", report.Steps)
	}
	if report.Steps[1].CandidateID != "good" {
		t.Fatalf("steps=%+v", report.Steps)
	}
}

type fallbackAdapterForReport struct{}

func (a *fallbackAdapterForReport) Name() string { return "fallback-test" }
func (a *fallbackAdapterForReport) Supports(step ExecutionStep) bool { return step.Command != "" }
func (a *fallbackAdapterForReport) Execute(_ context.Context, step ExecutionStep, _ ResolvedEnvironment) error {
	if step.Command == "bad-command" {
		return fmt.Errorf("candidate rejected")
	}
	return nil
}
