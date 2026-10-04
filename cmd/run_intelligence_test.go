package main

import (
	"context"
	"testing"

	"github.com/harshul/octo-cli/internal/intelligence"
)

func TestIntelligenceRunPlanningPath(t *testing.T) {
	model := intelligence.ProjectModel{
		Name: "demo",
		Root: t.TempDir(),
		Components: []intelligence.Component{{
			Name: "demo",
			Path: ".",
			PackageManager: "npm",
			RunCommand: "echo running",
		}},
	}

	plan, err := (intelligence.DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 4 {
		t.Fatalf("steps=%d, want 4", len(plan.Steps))
	}
	if plan.Steps[1].Phase != intelligence.PhaseProvision {
		t.Fatalf("second phase=%q, want provision", plan.Steps[1].Phase)
	}
	if plan.Steps[0].Phase != intelligence.PhasePrepare {
		t.Fatalf("first phase=%q, want prepare", plan.Steps[0].Phase)
	}
}
