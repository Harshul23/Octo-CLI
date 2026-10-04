package intelligence

import (
	"context"
	"fmt"
	"path/filepath"
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
	Component   string         `json:"component,omitempty" yaml:"component,omitempty"`
	NodeID      string         `json:"node_id" yaml:"node_id"`
	Phase       ExecutionPhase `json:"phase" yaml:"phase"`
	Command     string         `json:"command,omitempty" yaml:"command,omitempty"`
	WorkDir     string         `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Explanation string         `json:"explanation,omitempty" yaml:"explanation,omitempty"`
}

// ExecutionPlan is the provider-neutral result of planning how a ProjectModel runs.
type ExecutionPlan struct {
	ProjectName string          `json:"project_name" yaml:"project_name"`
	Root        string          `json:"root" yaml:"root"`
	Steps       []ExecutionStep `json:"steps" yaml:"steps"`
	Ports       []PortAssignment `json:"ports,omitempty" yaml:"ports,omitempty"`
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
	if len(model.Components) == 0 && len(model.Services) == 0 {
		return ExecutionPlan{}, fmt.Errorf("cannot plan project with no components or services")
	}

	topology, err := BuildTopologyGraph(model)
	if err != nil {
		return ExecutionPlan{}, fmt.Errorf("build topology: %w", err)
	}

	steps := make([]ExecutionStep, 0, len(model.Components)*4+len(model.Services))
	components := append([]Component(nil), model.Components...)
	sort.SliceStable(components, func(i, j int) bool { return components[i].Name < components[j].Name })

	for _, component := range components {
		if strings.TrimSpace(component.Name) == "" {
			return ExecutionPlan{}, fmt.Errorf("component has no name")
		}

		nodeID := topologyID(NodeComponent, component.Name)
		prefix := "component." + component.Name
		prepareID := prefix + ".prepare"
		steps = append(steps, ExecutionStep{
			ID: prepareID, Component: component.Name, NodeID: nodeID, Phase: PhasePrepare,
			WorkDir: component.Path,
			Explanation: "Prepare the component working directory.",
		})

		setupDepends := []string{prepareID}
		if install := installCommand(component.PackageManager); install != "" {
			installID := prefix + ".install"
			steps = append(steps, ExecutionStep{
				ID: installID, Component: component.Name, NodeID: nodeID, Phase: PhaseInstall,
				Command: install, WorkDir: component.Path, DependsOn: []string{prepareID},
				Explanation: "Install dependencies using the detected package manager.",
			})
			setupDepends = []string{installID}
		}

		if strings.TrimSpace(model.SetupCommand) != "" && component.Name == model.Name {
			setupID := prefix + ".setup"
			steps = append(steps, ExecutionStep{
				ID: setupID, Component: component.Name, NodeID: nodeID, Phase: PhaseSetup,
				Command: model.SetupCommand, WorkDir: component.Path, DependsOn: setupDepends,
				Explanation: "Run the repository setup command discovered by the analyzer.",
			})
			setupDepends = []string{setupID}
		}

		if strings.TrimSpace(component.RunCommand) == "" {
			return ExecutionPlan{}, fmt.Errorf("component %q has no run command", component.Name)
		}

		deps := append([]string(nil), setupDepends...)
		for _, edge := range topology.Edges {
			if edge.From == nodeID {
				deps = append(deps, readinessStepID(edge.To, model))
			}
		}

		steps = append(steps, ExecutionStep{
			ID: prefix + ".start", Component: component.Name, NodeID: nodeID, Phase: PhaseStart,
			Command: component.RunCommand, WorkDir: component.Path, DependsOn: uniqueStrings(deps),
			Explanation: "Start the component after all topology dependencies are started.",
		})
	}

	services := append([]Service(nil), model.Services...)
	sort.SliceStable(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, service := range services {
		if strings.TrimSpace(service.Name) == "" {
			return ExecutionPlan{}, fmt.Errorf("service has no name")
		}
		nodeID := topologyID(NodeService, service.Name)
		stepID := startStepID(nodeID)
		deps := make([]string, 0)
		for _, edge := range topology.Edges {
			if edge.From == nodeID {
				deps = append(deps, startStepID(edge.To))
			}
		}

		command, explanation := serviceStartCommand(service)
		steps = append(steps, ExecutionStep{
			ID: stepID, Component: service.Name, NodeID: nodeID, Phase: PhaseStart,
			Command: command, WorkDir: model.Root, DependsOn: uniqueStrings(deps),
			Explanation: explanation,
		})

		if service.HealthCheck != nil && strings.TrimSpace(service.HealthCheck.Command) != "" {
			healthID := readinessStepID(nodeID, model)
			healthCommand, healthExplanation := serviceHealthCommand(service, model)
			steps = append(steps, ExecutionStep{
				ID: healthID, Component: service.Name, NodeID: nodeID, Phase: PhaseHealth,
				Command: healthCommand, WorkDir: model.Root, DependsOn: []string{stepID},
				Explanation: healthExplanation,
			})
		}
	}

	ports, err := AllocateComponentPorts(model.Components)
	if err != nil {
		return ExecutionPlan{}, err
	}
	portByComponent := make(map[string]int, len(ports))
	for _, assignment := range ports {
		portByComponent[assignment.Component] = assignment.Resolved
	}
	for i := range steps {
		if steps[i].Phase == PhaseStart {
			if port, ok := portByComponent[steps[i].Component]; ok {
				if steps[i].Environment == nil {
					steps[i].Environment = make(map[string]string)
				}
				steps[i].Environment["PORT"] = fmt.Sprintf("%d", port)
			}
		}
	}

	plan := ExecutionPlan{ProjectName: model.Name, Root: model.Root, Steps: steps, Ports: ports}
	if err := ValidateExecutionPlan(plan); err != nil {
		return ExecutionPlan{}, err
	}
	return plan, nil
}

func startStepID(nodeID string) string {
	return strings.ReplaceAll(nodeID, ":", ".") + ".start"
}

func serviceStartCommand(service Service) (string, string) {
	for _, evidence := range service.Evidence {
		base := filepath.Base(evidence.Path)
		if evidence.Kind == EvidenceConfig && strings.Contains(base, "compose") {
			return fmt.Sprintf("docker compose -f %s up -d %s", evidence.Path, service.Name),
				"Start the Compose service using the repository's declared Compose configuration."
		}
	}
	return "", "Service topology was discovered, but no runtime adapter is currently known for this service."
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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


func readinessStepID(nodeID string, model ProjectModel) string {
	for _, service := range model.Services {
		if topologyID(NodeService, service.Name) == nodeID && service.HealthCheck != nil && strings.TrimSpace(service.HealthCheck.Command) != "" {
			return startStepID(nodeID)[:len(startStepID(nodeID))-len(".start")] + ".health"
		}
	}
	return startStepID(nodeID)
}


func serviceHealthCommand(service Service, model ProjectModel) (string, string) {
	command := strings.TrimSpace(service.HealthCheck.Command)
	for _, evidence := range service.Evidence {
		base := filepath.Base(evidence.Path)
		if evidence.Kind == EvidenceConfig && strings.Contains(base, "compose") {
			return fmt.Sprintf("docker compose -f %s exec -T %s sh -c %s", evidence.Path, service.Name, shellQuote(command)),
				"Run the declared Compose health check inside the service container."
		}
	}
	return command, "Run the explicitly declared service health check."
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
