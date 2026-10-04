package intelligence

import (
	"context"
	"testing"
)

type recordingAdapter struct {
	name  string
	seen  []string
	match func(ExecutionStep) bool
}

func (a *recordingAdapter) Name() string { return a.name }
func (a *recordingAdapter) Supports(step ExecutionStep) bool { return a.match(step) }
func (a *recordingAdapter) Execute(ctx context.Context, step ExecutionStep) error {
	a.seen = append(a.seen, step.ID)
	return nil
}

func TestRuntimeResolverSelectsComposeBeforeShell(t *testing.T) {
	resolver := NewRuntimeResolver()

	compose, err := resolver.Resolve(ExecutionStep{
		ID: "service.postgres.start",
		Command: "docker compose -f compose.yaml up -d postgres",
	})
	if err != nil {
		t.Fatal(err)
	}
	if compose.Name() != "compose" {
		t.Fatalf("adapter=%q, want compose", compose.Name())
	}

	shell, err := resolver.Resolve(ExecutionStep{
		ID: "component.api.start",
		Command: "npm run dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if shell.Name() != "shell" {
		t.Fatalf("adapter=%q, want shell", shell.Name())
	}
}

func TestRuntimeResolverRejectsUnsupportedStep(t *testing.T) {
	resolver := RuntimeResolver{}
	_, err := resolver.Resolve(ExecutionStep{ID: "unknown", Command: ""})
	if err == nil {
		t.Fatal("expected unsupported step error")
	}
}

func TestExecutePlanFollowsDependencyOrder(t *testing.T) {
	adapter := &recordingAdapter{
		name: "test",
		match: func(step ExecutionStep) bool { return step.Command != "" },
	}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}

	plan := ExecutionPlan{
		ProjectName: "demo",
		Steps: []ExecutionStep{
			{ID: "component.api.start", Command: "api"},
			{ID: "component.web.start", Command: "web", DependsOn: []string{"component.api.start"}},
		},
	}

	if err := ExecutePlan(context.Background(), plan, resolver); err != nil {
		t.Fatal(err)
	}
	if len(adapter.seen) != 2 {
		t.Fatalf("executed=%v", adapter.seen)
	}
	if adapter.seen[0] != "component.api.start" || adapter.seen[1] != "component.web.start" {
		t.Fatalf("execution order=%v", adapter.seen)
	}
}

func TestAdaptersDoNotClaimEmptyCommands(t *testing.T) {
	step := ExecutionStep{ID: "prepare"}
	if (ShellAdapter{}).Supports(step) {
		t.Fatal("shell adapter must not claim empty command")
	}
	if (ComposeAdapter{}).Supports(step) {
		t.Fatal("compose adapter must not claim empty command")
	}
}
