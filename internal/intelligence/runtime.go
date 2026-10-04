package intelligence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RuntimeAdapter executes one execution step for a specific runtime kind.
type RuntimeAdapter interface {
	Name() string
	Supports(step ExecutionStep) bool
	Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error
}

// RuntimeResolver selects an adapter for an execution step.
type RuntimeResolver struct {
	adapters []RuntimeAdapter
}

// NewRuntimeResolver creates a resolver with the built-in adapters.
func NewRuntimeResolver() RuntimeResolver {
	return RuntimeResolver{
		adapters: []RuntimeAdapter{
			ComposeAdapter{},
			ShellAdapter{},
		},
	}
}

// Resolve returns the first adapter that explicitly supports the step.
func (r RuntimeResolver) Resolve(step ExecutionStep) (RuntimeAdapter, error) {
	for _, adapter := range r.adapters {
		if adapter.Supports(step) {
			return adapter, nil
		}
	}
	return nil, fmt.Errorf("no runtime adapter supports step %q", step.ID)
}

// ShellAdapter executes ordinary shell commands.
type ShellAdapter struct{}

func (ShellAdapter) Name() string { return "shell" }

func (ShellAdapter) Supports(step ExecutionStep) bool {
	return step.Command != "" && !strings.HasPrefix(step.Command, "docker compose ")
}

func (ShellAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if step.Command == "" {
		return fmt.Errorf("step %q has no command", step.ID)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = mergedEnvironmentWithStep(env.Values, step.Environment)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell step %q failed: %w", step.ID, err)
	}
	return nil
}

// ComposeAdapter executes Docker Compose service steps.
type ComposeAdapter struct{}

func (ComposeAdapter) Name() string { return "compose" }

func (ComposeAdapter) Supports(step ExecutionStep) bool {
	return strings.HasPrefix(step.Command, "docker compose ")
}

func (ComposeAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if !(ComposeAdapter{}).Supports(step) {
		return fmt.Errorf("step %q is not a Compose step", step.ID)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = mergedEnvironmentWithStep(env.Values, step.Environment)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Compose step %q failed: %w", step.ID, err)
	}
	return nil
}

// ExecutePlan executes a plan in its validated topological order.
// This is intentionally separate from the legacy orchestrator.
func ExecutePlan(ctx context.Context, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment) error {
	if err := ValidateExecutionPlan(plan); err != nil {
		return fmt.Errorf("invalid execution plan: %w", err)
	}

	order, err := TopologicalOrder(plan)
	if err != nil {
		return err
	}

	steps := make(map[string]ExecutionStep, len(plan.Steps))
	for _, step := range plan.Steps {
		steps[step.ID] = step
	}

	for _, id := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		step := steps[id]
		if step.Command == "" {
			// Preparation/metadata steps may intentionally have no executable command.
			continue
		}
		adapter, err := resolver.Resolve(step)
		if err != nil {
			return err
		}
		if err := adapter.Execute(ctx, step, env); err != nil {
			return err
		}
	}

	return nil
}


func mergedEnvironment(values map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for name, value := range values {
		replaced := false
		prefix := name + "="
		for i, entry := range env {
			if strings.HasPrefix(entry, prefix) {
				env[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}


func mergedEnvironmentWithStep(values, stepValues map[string]string) []string {
	merged := make(map[string]string, len(values)+len(stepValues))
	for name, value := range values {
		merged[name] = value
	}
	for name, value := range stepValues {
		merged[name] = value
	}
	return mergedEnvironment(merged)
}
