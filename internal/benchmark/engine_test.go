package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateRepository(t *testing.T) {
	root := t.TempDir()
	packageJSON := `{
  "name": "benchmark-demo",
  "scripts": {
    "start": "node index.js -p 3000"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EvaluateRepository(context.Background(), root, "Node.js Single App")
	if err != nil {
		t.Fatalf("EvaluateRepository failed: %v", err)
	}

	if result.Repository != filepath.Base(root) {
		t.Fatalf("Repository = %q, want %q", result.Repository, filepath.Base(root))
	}
	if !result.OctoScore.OneShotSuccess {
		t.Fatal("expected Octo OneShotSuccess to be true")
	}
	if result.OctoScore.HallucinationRate != 0.0 {
		t.Fatalf("Octo HallucinationRate = %v, want 0.0", result.OctoScore.HallucinationRate)
	}
	if !result.OctoScore.VerifiedReadiness {
		t.Fatal("expected VerifiedReadiness to be true")
	}
	if result.Comparison.TokenReductionPct <= 0 {
		t.Fatalf("expected positive token reduction, got %v%%", result.Comparison.TokenReductionPct)
	}
}

func TestRunBuiltinSuite(t *testing.T) {
	report, err := RunBuiltinSuite(context.Background())
	if err != nil {
		t.Fatalf("RunBuiltinSuite failed: %v", err)
	}

	if report.TotalCases < 8 {
		t.Fatalf("expected at least 8 test cases in suite, got %d", report.TotalCases)
	}
	if report.OctoPassRate != 100.0 {
		t.Fatalf("OctoPassRate = %v, want 100.0", report.OctoPassRate)
	}
	if !report.ZeroHallucinationConfirmed {
		t.Fatal("expected ZeroHallucinationConfirmed to be true")
	}
	if report.AvgTokenSavingsPct < 50.0 {
		t.Fatalf("expected AvgTokenSavingsPct >= 50%%, got %v%%", report.AvgTokenSavingsPct)
	}
	if len(report.Results) != report.TotalCases {
		t.Fatalf("Results count %d != TotalCases %d", len(report.Results), report.TotalCases)
	}
}
