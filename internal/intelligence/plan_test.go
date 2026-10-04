package intelligence

import (
	"context"
	"testing"
)

func TestDeterministicPlannerCreatesStableSingleComponentPlan(t *testing.T) {
	model := ProjectModel{
		Name: "demo",
		Root: "/tmp/demo",
		SetupCommand: "npm run build",
		Components: []Component{{
			Name: "demo", Path: ".", Language: "Node",
			PackageManager: "pnpm", RunCommand: "pnpm dev",
		}},
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	wantPhases := []ExecutionPhase{PhasePrepare, PhaseInstall, PhaseSetup, PhaseStart}
	if len(plan.Steps) != len(wantPhases) {
		t.Fatalf("steps=%d, want %d", len(plan.Steps), len(wantPhases))
	}
	for i, phase := range wantPhases {
		if plan.Steps[i].Phase != phase {
			t.Fatalf("step %d phase=%q, want %q", i, plan.Steps[i].Phase, phase)
		}
	}

	order, err := TopologicalOrder(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != len(plan.Steps) {
		t.Fatalf("order=%d, steps=%d", len(order), len(plan.Steps))
	}
}

func TestTopologicalOrderRejectsCycle(t *testing.T) {
	plan := ExecutionPlan{Steps: []ExecutionStep{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}}
	if err := ValidateExecutionPlan(plan); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestPlannerSupportsComponentDependencies(t *testing.T) {
	model := ProjectModel{
		Name: "stack",
		Components: []Component{
			{Name: "web", Path: "web", PackageManager: "npm", RunCommand: "npm run dev", DependsOn: []string{"api"}},
			{Name: "api", Path: "api", PackageManager: "npm", RunCommand: "npm run dev"},
		},
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	order, err := TopologicalOrder(plan)
	if err != nil {
		t.Fatal(err)
	}

	position := func(id string) int {
		for i, v := range order {
			if v == id { return i }
		}
		return -1
	}
	if position("component.api.start") > position("component.web.start") {
		t.Fatalf("api must start before web: %v", order)
	}
}
