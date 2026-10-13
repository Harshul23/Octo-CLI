package main

import (
	"context"
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
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

func TestRunCommandRegistersPhase1Flags(t *testing.T) {
	for _, flagName := range []string{"detach", "watch", "json", "engine", "sandbox"} {
		flag := runCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Fatalf("expected flag %q to be registered on runCmd", flagName)
		}
	}
}

func TestRunCommandRegistersTargetSelectionFlags(t *testing.T) {
	onlyFlag := runCmd.Flags().Lookup("only")
	if onlyFlag == nil {
		t.Fatal("expected --only flag to be registered on runCmd")
	}
	if onlyFlag.Shorthand != "o" {
		t.Fatalf("expected --only shorthand 'o', got %q", onlyFlag.Shorthand)
	}

	allFlag := runCmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("expected --all flag to be registered on runCmd")
	}

	// Verify documentation explains target selection and non-interactive behavior
	if !strings.Contains(runCmd.Long, "--only") || !strings.Contains(runCmd.Long, "--all") {
		t.Fatal("expected runCmd.Long to document --only and --all")
	}
	if !strings.Contains(runCmd.Long, "mutually exclusive") {
		t.Fatal("expected runCmd.Long to document mutual exclusion")
	}
	if !strings.Contains(runCmd.Long, "non-interactive") {
		t.Fatal("expected runCmd.Long to document non-interactive behavior")
	}
}

func TestRunTargetSelectionResolutionRules(t *testing.T) {
	model := intelligence.ProjectModel{
		Name: "multi-demo",
		Components: []intelligence.Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
			{Name: "api", Path: "apps/api", RunCommand: "go run .", Role: "application"},
		},
		Services: []intelligence.Service{
			{Name: "postgres", Image: "postgres:16"},
		},
	}

	// Mutual exclusion error
	_, err := intelligence.ResolveSelection(model, intelligence.SelectionOptions{
		Only: []string{"component:web"},
		All:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot combine --only and --all") {
		t.Fatalf("expected mutual exclusion error, got: %v", err)
	}

	// Non-interactive multi-target requires --only or --all
	_, err = intelligence.ResolveSelection(model, intelligence.SelectionOptions{})
	if err == nil || !strings.Contains(err.Error(), "multiple runnable targets detected") {
		t.Fatalf("expected multiple runnable targets error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "component:web") || !strings.Contains(err.Error(), "service:postgres") {
		t.Fatalf("expected error to include target list, got: %v", err)
	}

	// Single target repository auto-selects
	singleModel := intelligence.ProjectModel{
		Name: "single-demo",
		Components: []intelligence.Component{
			{Name: "web", Path: ".", RunCommand: "npm run dev", Role: "application"},
		},
	}
	sel, err := intelligence.ResolveSelection(singleModel, intelligence.SelectionOptions{})
	if err != nil {
		t.Fatalf("unexpected error for single target: %v", err)
	}
	if len(sel.SelectedComponents) != 1 || sel.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected web to be auto-selected, got: %#v", sel.SelectedComponents)
	}
}

