package intelligence

import (
	"context"
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
		adapters: []RuntimeAdapter{recordingAdapterForReport{}},
	}, ResolvedEnvironment{Values: map[string]string{}})

	if !report.Success {
		t.Fatalf("report=%+v", report)
	}
	if len(report.Steps) != 1 || report.Steps[0].Status != StepSucceeded {
		t.Fatalf("steps=%+v", report.Steps)
	}
	if report.Steps[0].Adapter != "recording" {
		t.Fatalf("step=%+v", report.Steps[0])
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
		t.Fatalf("steps=%+v", report.Steps)
	}
	if report.FailureReason == "" {
		t.Fatal("expected failure reason")
	}
}

type recordingAdapterForReport struct{}

func (recordingAdapterForReport) Name() string { return "recording" }
func (recordingAdapterForReport) Supports(step ExecutionStep) bool { return true }
func (recordingAdapterForReport) Execute(context.Context, ExecutionStep, ResolvedEnvironment) error { return nil }
