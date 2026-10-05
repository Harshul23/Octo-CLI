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
			ExecutionCandidates: []ExecutionCandidate{{ID: "node.script.dev", Command: "pnpm dev", Confidence: 0.82}},
		}},
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	wantPhases := []ExecutionPhase{PhasePrepare, PhaseProvision, PhaseInstall, PhaseSetup, PhaseStart}
	if len(plan.Steps) != len(wantPhases) {
		t.Fatalf("steps=%d, want %d", len(plan.Steps), len(wantPhases))
	}
	if len(plan.Topology.Nodes) != 1 || plan.Topology.Nodes[0].ID != "component:demo" {
		t.Fatalf("unexpected topology nodes: %+v", plan.Topology.Nodes)
	}
	start := findExecutionStep(plan, "component.demo.start")
	if start == nil || start.SelectedCandidate != "node.script.dev" || len(start.Candidates) != 1 {
		t.Fatalf("start candidates=%+v", start)
	}
	if start.LongRunning {
		t.Fatal("component without an application port must remain synchronous")
	}
	if len(plan.Topology.Edges) != 0 {
		t.Fatalf("unexpected topology edges: %+v", plan.Topology.Edges)
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


func TestPlannerMarksPortBackedComponentAsLongRunning(t *testing.T) {
	model := ProjectModel{
		Name: "web",
		Components: []Component{{
			Name: "web", Path: ".", Language: "Node",
			RunCommand: "npm start", Port: 3000,
		}},
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	start := findExecutionStep(plan, "component.web.start")
	if start == nil {
		t.Fatal("missing start step")
	}
	if !start.LongRunning {
		t.Fatal("port-backed component must be marked long-running")
	}
	if len(plan.Ports) != 1 || plan.Ports[0].Resolved != 3000 {
		t.Fatalf("ports=%+v", plan.Ports)
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
	if !hasEdge(plan.Topology, "component:web", "component:api") {
		t.Fatalf("missing topology edge: %+v", plan.Topology.Edges)
	}
}


func TestPlannerIncludesComposeServicesAndCrossNodeDependencies(t *testing.T) {
	model := ProjectModel{
		Name: "stack",
		Root: "/tmp/stack",
		Components: []Component{{
			Name: "api", Path: "api", PackageManager: "npm",
			RunCommand: "npm run dev", DependsOn: []string{"postgres"},
		}},
		Services: []Service{{
			Name: "postgres", Image: "postgres:17",
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: "compose.yaml", Strength: 0.99}},
		}},
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	service := findExecutionStep(plan, "service.postgres.start")
	if service == nil {
		t.Fatal("missing postgres service step")
	}
	if service.Command != "docker compose -f compose.yaml up -d postgres" {
		t.Fatalf("service command=%q", service.Command)
	}

	api := findExecutionStep(plan, "component.api.start")
	if api == nil {
		t.Fatal("missing api start step")
	}
	if !contains(api.DependsOn, "service.postgres.start") {
		t.Fatalf("api dependencies=%v", api.DependsOn)
	}

	order, err := TopologicalOrder(plan)
	if err != nil {
		t.Fatal(err)
	}
	if positionIn(order, "service.postgres.start") > positionIn(order, "component.api.start") {
		t.Fatalf("postgres must start before api: %v", order)
	}
}

func findExecutionStep(plan ExecutionPlan, id string) *ExecutionStep {
	for i := range plan.Steps {
		if plan.Steps[i].ID == id {
			return &plan.Steps[i]
		}
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func positionIn(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}


func TestPlannerUsesPythonPackageManagerInstallCommands(t *testing.T) {
	tests := []struct {
		name    string
		manager string
		want    string
	}{
		{name: "uv", manager: "uv", want: "uv sync"},
		{name: "poetry", manager: "poetry", want: "poetry install"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := ProjectModel{
				Name: "demo",
				Components: []Component{{
					Name: "demo", Path: ".", Language: "Python",
					PackageManager: tt.manager, RunCommand: tt.manager + " run python main.py",
				}},
			}
			plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
			if err != nil {
				t.Fatal(err)
			}
			var install *ExecutionStep
			for i := range plan.Steps {
				if plan.Steps[i].Phase == PhaseInstall {
					install = &plan.Steps[i]
					break
				}
			}
			if install == nil || install.Command != tt.want {
				t.Fatalf("install step=%+v, want command %q", install, tt.want)
			}
		})
	}
}


type testDecisionProvider struct {
	result DecisionResult
	calls  int
}

func (p *testDecisionProvider) Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error) {
	p.calls++
	return p.result, nil
}

func TestPlannerUsesDecisionProviderForMultipleExecutionCandidates(t *testing.T) {
	provider := &testDecisionProvider{result: DecisionResult{
		OptionID: "node.script.start",
		Value: "npm start",
		Confidence: 0.94,
	}}
	model := ProjectModel{
		Name: "demo",
		Components: []Component{{
			Name: "demo", Path: ".", Language: "Node",
			RunCommand: "npm dev",
			ExecutionCandidates: []ExecutionCandidate{
				{ID: "node.script.start", Command: "npm start", Confidence: 0.94},
				{ID: "node.script.dev", Command: "npm dev", Confidence: 0.82},
			},
		}},
	}

	plan, err := (DeterministicPlanner{DecisionProvider: provider}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("decision provider calls=%d, want 1", provider.calls)
	}
	start := findExecutionStep(plan, "component.demo.start")
	if start == nil {
		t.Fatal("missing start step")
	}
	if start.Command != "npm start" || start.SelectedCandidate != "node.script.start" {
		t.Fatalf("start=%+v, want selected npm start candidate", start)
	}
}
