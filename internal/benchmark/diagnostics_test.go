package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func TestFailedExecutionDiagnostics(t *testing.T) {
	report := intelligence.ExecutionReport{
		FailureReason: "npm install failed",
		Steps:         []intelligence.ExecutionStepResult{{ID: "component.web.install", Status: intelligence.StepFailed, Reason: "exit status 1"}},
	}
	d := failedExecutionDiagnostics(report, nil, nil, context.DeadlineExceeded)
	if d == nil || d.FailureReason != report.FailureReason || d.ContextError != "context deadline exceeded" || len(d.Steps) != 1 || d.Steps[0].Reason != "exit status 1" {
		t.Fatalf("diagnostics=%+v", d)
	}
	report.Steps[0].Reason = "changed"
	if d.Steps[0].Reason != "exit status 1" {
		t.Fatal("diagnostics alias report steps")
	}
	data, err := json.Marshal(BenchmarkResult{Diagnostics: d})
	if err != nil || !strings.Contains(string(data), `"diagnostics"`) || !strings.Contains(string(data), `"context_error"`) {
		t.Fatalf("JSON=%s, err=%v", data, err)
	}
}

func TestVerificationFailureDiagnostics(t *testing.T) {
	checks := []intelligence.VerificationResult{{CheckID: "web.port", Passed: false, Reason: "connection refused"}}
	d := failedExecutionDiagnostics(intelligence.ExecutionReport{Success: true}, checks, errors.New("port not ready"), nil)
	if d == nil || d.FailureReason != "port not ready" || len(d.Verification) != 1 {
		t.Fatalf("diagnostics=%+v", d)
	}
	d = failedExecutionDiagnostics(intelligence.ExecutionReport{Success: true}, checks, nil, nil)
	if d == nil || !strings.Contains(d.FailureReason, "connection refused") {
		t.Fatalf("diagnostics=%+v", d)
	}
	if got := failedExecutionDiagnostics(intelligence.ExecutionReport{Success: true}, nil, nil, nil); got != nil {
		t.Fatalf("success diagnostics=%+v", got)
	}
}

func TestEvaluateRepositoryPreservesCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"cancelled","scripts":{"start":"node index.js"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter := &cancellingBenchmarkAdapter{cancel: cancel}
	intelligence.RegisterRuntimeAdapter(adapter)
	defer intelligence.UnregisterRuntimeAdapter(adapter)
	result, err := EvaluateRepository(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.OctoScore.OneShotSuccess || result.OctoScore.VerifiedReadiness {
		t.Fatalf("cancelled score=%+v", result.OctoScore)
	}
	if result.Diagnostics == nil || result.Diagnostics.ContextError != "context canceled" || !strings.Contains(result.Diagnostics.FailureReason, "context canceled") {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

type cancellingBenchmarkAdapter struct {
	cancel  context.CancelFunc
	failure error
}

func (*cancellingBenchmarkAdapter) Name() string { return "benchmark-cancel-test" }
func (*cancellingBenchmarkAdapter) Supports(step intelligence.ExecutionStep) bool {
	return step.Component == "cancelled"
}
func (a *cancellingBenchmarkAdapter) Execute(ctx context.Context, _ intelligence.ExecutionStep, _ intelligence.ResolvedEnvironment) error {
	if a.failure != nil {
		return a.failure
	}
	a.cancel()
	return ctx.Err()
}

func TestEvaluateRepositoryPreservesCommandFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"cancelled","scripts":{"start":"node index.js"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	adapter := &cancellingBenchmarkAdapter{failure: errors.New("simulated npm command failed: exit status 1")}
	intelligence.RegisterRuntimeAdapter(adapter)
	defer intelligence.UnregisterRuntimeAdapter(adapter)
	result, err := EvaluateRepository(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.OctoScore.OneShotSuccess || result.OctoScore.VerifiedReadiness || result.Diagnostics == nil {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(result.Diagnostics.FailureReason, "simulated npm command failed") || len(result.Diagnostics.Steps) < 2 || result.Diagnostics.Steps[1].Status != intelligence.StepFailed {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	if result.Diagnostics.ContextError != "" {
		t.Fatalf("unexpected context error: %s", result.Diagnostics.ContextError)
	}
}
