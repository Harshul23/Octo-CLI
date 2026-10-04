package intelligence

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ExecutionPhase describes the purpose of an execution step.
type ExecutionPhase string

const (
	PhasePrepare ExecutionPhase = "prepare"
	PhaseInstall ExecutionPhase = "install"
	PhaseSetup   ExecutionPhase = "setup"
	PhaseStart   ExecutionPhase = "start"
	PhaseHealth  ExecutionPhase = "health"
)

// ExecutionStep is one deterministic unit in an execution plan.
type ExecutionStep struct {
	ID          string         `json:"id" yaml:"id"`
	Component   string         `json:"component" yaml:"component"`
	Phase       ExecutionPhase `json:"phase" yaml:"phase"`
	Command     string         `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     string         `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Explanation string         `json:"explanation,omitempty" yaml:"explanation,omitempty"`
}

// ExecutionPlan is the provider-neutral result of planning how a ProjectModel runs.
type ExecutionPlan struct {
	ProjectName string          `json:"project_name" yaml:"project_name"`
	Root        string          `json:"root" yaml:"root"`
	Steps       []ExecutionStep `json:"steps" yaml:"steps"`
}

// Planner converts repository understanding into an executable plan.
type Planner interface {
	Plan(ctx context.Context, model ProjectModel) (ExecutionPlan, error)
}

// DeterministicPlanner creates plans using only repository facts and fixed rules.
// It deliberately has no network, model, or paid-service dependency.
type DeterministicPlanner struct{}

// Plan creates a stable execution graph from the currently known components.
func (DeterministicPlanner) Plan(ctx context.Context, model ProjectModel) (ExecutionPlan, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionPlan{}, err
	}
	if strings.TrimSpace(model.Name) == "" {
		return ExecutionPlan{}, fmt.Errorf("cannot plan unnamed project")
	}
	if len(model.Components) == 0 {
		return ExecutionPlan{}, fmt.Errorf("cannot plan project with no components")
	}

	steps := make([]ExecutionStep, 0, len(model.Components)*4)
	components := append([]Component(nil), model.Components...)
	sort.SliceStable(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	for _, component := range components {
		if strings.TrimSpace(component.Name) == "" {
			return ExecutionPlan{}, fmt.Errorf("component has no name")
		}

		prefix := "component." + component.Name
		prepareID := prefix + ".prepare"
		steps = append(steps, ExecutionStep{
			ID: prepareID, Component: component.Name, Phase: PhasePrepare,
			WorkDir: component.Path,
			Explanation: "Prepare the component working directory.",
		})

		if install := installCommand(component.PackageManager); install != "" {
			steps = append(steps, ExecutionStep{
				ID: prefix + ".install", Component: component.Name, Phase: PhaseInstall,
				Command: install, WorkDir: component.Path, DependsOn: []string{prepareID},
				Explanation: "Install dependencies using the detected package manager.",
			})
		}

		setupDepends := []string{prepareID}
		if len(steps) > 0 && steps[len(steps)-1].Component == component.Name && steps[len(steps)-1].Phase == PhaseInstall {
			setupDepends = []string{prefix + ".install"}
		}
		if strings.TrimSpace(model.SetupCommand) != "" && component.Name == model.Name {
			steps = append(steps, ExecutionStep{
				ID: prefix + ".setup", Component: component.Name, Phase: PhaseSetup,
				Command: model.SetupCommand, WorkDir: component.Path, DependsOn: setupDepends,
				Explanation: "Run the repository setup command discovered by the analyzer.",
			})
			setupDepends = []string{prefix + ".setup"}
		}

		if strings.TrimSpace(component.RunCommand) == "" {
			return ExecutionPlan{}, fmt.Errorf("component %q has no run command", component.Name)
		}
		start := ExecutionStep{
			ID: prefix + ".start", Component: component.Name, Phase: PhaseStart,
			Command: component.RunCommand, WorkDir: component.Path, DependsOn: setupDepends,
			Explanation: "Start the component using the detected run command.",
		}
		for _, dep := range component.DependsOn {
			start.DependsOn = append(start.DependsOn, "component."+dep+".start")
		}
		steps = append(steps, start)
	}

	plan := ExecutionPlan{ProjectName: model.Name, Root: model.Root, Steps: steps}
	if err := ValidateExecutionPlan(plan); err != nil {
		return ExecutionPlan{}, err
	}
	return plan, nil
}

func installCommand(packageManager string) string {
	switch strings.ToLower(packageManager) {
	case "npm":
		return "npm install"
	case "pnpm":
		return "pnpm install"
	case "yarn":
		return "yarn install"
	case "bun":
		return "bun install"
	case "pip":
		return "pip install -r requirements.txt"
	case "cargo":
		return "cargo fetch"
	default:
		return ""
	}
}

// ValidateExecutionPlan checks references, duplicate IDs, and dependency cycles.
func ValidateExecutionPlan(plan ExecutionPlan) error {
	ids := make(map[string]struct{}, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.ID == "" {
			return fmt.Errorf("execution step has empty ID")
		}
		if _, exists := ids[step.ID]; exists {
			return fmt.Errorf("duplicate execution step ID %q", step.ID)
		}
		ids[step.ID] = struct{}{}
	}

	graph := make(map[string][]string, len(plan.Steps))
	for _, step := range plan.Steps {
		for _, dep := range step.DependsOn {
			if _, exists := ids[dep]; !exists {
				return fmt.Errorf("step %q depends on unknown step %q", step.ID, dep)
			}
			graph[dep] = append(graph[dep], step.ID)
		}
	}

	_, err := TopologicalOrder(plan)
	return err
}

// TopologicalOrder returns a stable dependency-respecting step order.
func TopologicalOrder(plan ExecutionPlan) ([]string, error) {
	indegree := make(map[string]int, len(plan.Steps))
	out := make(map[string][]string, len(plan.Steps))
	for _, step := range plan.Steps {
		indegree[step.ID] = len(step.DependsOn)
		for _, dep := range step.DependsOn {
			out[dep] = append(out[dep], step.ID)
		}
	}

	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	order := make([]string, 0, len(plan.Steps))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)

		next := append([]string(nil), out[id]...)
		sort.Strings(next)
		for _, child := range next {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}

	if len(order) != len(plan.Steps) {
		return nil, fmt.Errorf("execution plan contains a dependency cycle")
	}
	return order, nil
}
