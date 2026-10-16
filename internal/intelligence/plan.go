package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ExecutionPhase describes the purpose of an execution step.
type ExecutionPhase string

const (
	PhasePrepare ExecutionPhase = "prepare"
	PhaseProvision ExecutionPhase = "provision"
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
	LongRunning bool           `json:"long_running,omitempty" yaml:"long_running,omitempty"`
	Candidates  []ExecutionCandidate `json:"candidates,omitempty" yaml:"candidates,omitempty"`
	SelectedCandidate string       `json:"selected_candidate,omitempty" yaml:"selected_candidate,omitempty"`
	WorkDir     string         `json:"work_dir,omitempty" yaml:"work_dir,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Explanation string         `json:"explanation,omitempty" yaml:"explanation,omitempty"`
}

// ExecutionPlan is the provider-neutral result of planning how a ProjectModel runs.
type ExecutionPlan struct {
	ProjectName  string                    `json:"project_name" yaml:"project_name"`
	Topology     TopologyGraph             `json:"topology" yaml:"topology"`
	Root         string                    `json:"root" yaml:"root"`
	Steps        []ExecutionStep           `json:"steps" yaml:"steps"`
	Ports        []PortAssignment          `json:"ports,omitempty" yaml:"ports,omitempty"`
	Provisioning []ProvisioningRequirement `json:"provisioning,omitempty" yaml:"provisioning,omitempty"`
	Environment  []EnvironmentResolution  `json:"environment,omitempty" yaml:"environment,omitempty"`
}

// Planner converts repository understanding into an executable plan.
type Planner interface {
	Plan(ctx context.Context, model ProjectModel) (ExecutionPlan, error)
}

// DeterministicPlanner creates plans using only repository facts and fixed rules.
// It deliberately has no network, model, or paid-service dependency.
type DeterministicPlanner struct {
	DecisionProvider DecisionProvider
}

// Plan creates a stable execution graph from the currently known components.
func (d DeterministicPlanner) Plan(ctx context.Context, model ProjectModel) (ExecutionPlan, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionPlan{}, err
	}
	if strings.TrimSpace(model.Name) == "" {
		return ExecutionPlan{}, newFailure(FailureInvalidProject, "", "Octo cannot build an execution plan for this repository.", "The project has no detected name.")
	}
	if len(model.Components) == 0 && len(model.Services) == 0 {
		return ExecutionPlan{}, newFailure(FailureInvalidProject, "", "Octo cannot build an execution plan for this repository.", "No runnable components or services were discovered.")
	}

	topology, err := BuildTopologyGraph(model)
	if err != nil {
		return ExecutionPlan{}, newFailure(FailureTopologyBuild, "", "Octo could not build the repository topology.", err.Error())
	}

	steps := make([]ExecutionStep, 0, len(model.Components)*5+len(model.Services))
	provisioning := BuildProvisioningRequirements(model)
	components := append([]Component(nil), model.Components...)
	sort.SliceStable(components, func(i, j int) bool { return components[i].Name < components[j].Name })

	isNodeWorkspace := hasNodeWorkspace(model.Root)
	var rootNodeInstallID string
	if isNodeWorkspace {
		hasNodeComponent := false
		for _, c := range components {
			if c.Language == "Node" || c.PackageManager == "npm" || c.PackageManager == "pnpm" || c.PackageManager == "yarn" || c.PackageManager == "bun" {
				hasNodeComponent = true
				break
			}
		}
		if hasNodeComponent {
			rootPM := detectRootNodePackageManager(model.Root)
			rootProvisionID := "workspace.root.provision"
			steps = append(steps, ExecutionStep{
				ID:          rootProvisionID,
				NodeID:      "workspace:root",
				Phase:       PhaseProvision,
				Command:     "command -v " + rootPM,
				WorkDir:     model.Root,
				Explanation: "Verify workspace package manager is available at repository root.",
			})

			rootNodeInstallID = "workspace.root.install"
			steps = append(steps, ExecutionStep{
				ID:          rootNodeInstallID,
				NodeID:      "workspace:root",
				Phase:       PhaseInstall,
				Command:     installCommand(rootPM),
				WorkDir:     model.Root,
				DependsOn:   []string{rootProvisionID},
				Explanation: "Install shared workspace dependencies once at repository root.",
			})
		}
	}

	sharedSetupExecuted := false
	var sharedSetupID string

	for _, component := range components {
		if strings.TrimSpace(component.Name) == "" {
			return ExecutionPlan{}, newFailure(FailureInvalidComponent, "", "Octo found a component without a name.", "The component metadata is incomplete.")
		}

		nodeID := topologyID(NodeComponent, component.Name)
		prefix := "component." + component.Name
		compWorkDir := componentWorkDir(model.Root, component.Path)
		prepareID := prefix + ".prepare"
		steps = append(steps, ExecutionStep{
			ID: prepareID, Component: component.Name, NodeID: nodeID, Phase: PhasePrepare,
			WorkDir: compWorkDir,
			Explanation: "Prepare the component working directory.",
		})

		setupDepends := []string{prepareID}
		isWorkspaceNodeComp := isNodeWorkspace && (component.Language == "Node" || component.PackageManager == "npm" || component.PackageManager == "pnpm" || component.PackageManager == "yarn" || component.PackageManager == "bun")

		if isWorkspaceNodeComp && rootNodeInstallID != "" {
			setupDepends = append(setupDepends, rootNodeInstallID)
		} else {
			// Independent unmarked app installs local
			if requirement := provisioningRequirementFor(component, provisioning); requirement != nil {
				provisionID := prefix + ".provision"
				steps = append(steps, ExecutionStep{
					ID: provisionID, Component: component.Name, NodeID: nodeID, Phase: PhaseProvision,
					Command: provisionCheckCommand(requirement.Name), WorkDir: compWorkDir,
					DependsOn: []string{prepareID},
					Explanation: "Verify the required package manager is available before installation.",
				})
				setupDepends = []string{provisionID}
			}
			if install := installCommand(component.PackageManager); install != "" {
				installID := prefix + ".install"
				steps = append(steps, ExecutionStep{
					ID: installID, Component: component.Name, NodeID: nodeID, Phase: PhaseInstall,
					Command: install, WorkDir: compWorkDir, DependsOn: setupDepends,
					Explanation: "Install dependencies using the detected package manager.",
				})
				setupDepends = []string{installID}
			}
		}

		if strings.TrimSpace(model.SetupCommand) != "" {
			if !sharedSetupExecuted {
				sharedSetupID = prefix + ".setup"
				steps = append(steps, ExecutionStep{
					ID: sharedSetupID, Component: component.Name, NodeID: nodeID, Phase: PhaseSetup,
					Command: model.SetupCommand, WorkDir: compWorkDir, DependsOn: setupDepends,
					Explanation: "Run the repository setup command discovered by the analyzer.",
				})
				setupDepends = []string{sharedSetupID}
				sharedSetupExecuted = true
			} else if sharedSetupID != "" {
				setupDepends = append(setupDepends, sharedSetupID)
			}
		}

		if component.Role == "library" {
			// Classified workspace library:
			// Evidence-backed setup/build step if present, but never an invented start command
			if buildCmd := detectComponentBuildCommand(model.Root, component); buildCmd != "" {
				buildID := prefix + ".build"
				steps = append(steps, ExecutionStep{
					ID: buildID, Component: component.Name, NodeID: nodeID, Phase: PhaseSetup,
					Command: buildCmd, WorkDir: compWorkDir, DependsOn: setupDepends,
					Explanation: "Build workspace library dependency.",
				})
			}
			continue
		}

		startCandidates := append([]ExecutionCandidate(nil), component.ExecutionCandidates...)
		primaryCandidates := primaryExecutionCandidates(startCandidates)
		startCommand := component.RunCommand
		selectedCandidate := selectedCandidateID(component)
		if strings.TrimSpace(startCommand) == "" && len(primaryCandidates) == 0 {
			return ExecutionPlan{}, newFailure(FailureNoExecutableCandidates, component.Name, fmt.Sprintf("Component %q cannot be executed.", component.Name), "Octo detected the component, but found no supported executable entry point.", "This usually means the component is a library, example-only project, or uses an unsupported application layout.")
		}
		if len(primaryCandidates) > 0 && selectedCandidate == "" {
			provider := d.DecisionProvider
			if provider == nil {
				provider = DeterministicDecisionProvider{}
			}
			result, err := provider.Decide(ctx, executionDecisionRequest(component.Name, primaryCandidates))
			if err != nil {
				return ExecutionPlan{}, newFailure(FailureDecision, component.Name, fmt.Sprintf("Octo could not choose how to run component %q.", component.Name), err.Error())
			}
			startCommand = result.Value
			selectedCandidate = result.OptionID
		}
		if strings.TrimSpace(startCommand) == "" {
			return ExecutionPlan{}, newFailure(FailureNoRunCommand, component.Name, fmt.Sprintf("Component %q has no executable command.", component.Name), "Octo found execution candidates, but none produced a usable command.")
		}

		deps := append([]string(nil), setupDepends...)
		for _, edge := range topology.Edges {
			if edge.From == nodeID && edge.Kind == "depends_on" {
				deps = append(deps, readinessStepID(edge.To, model))
			}
		}

		steps = append(steps, ExecutionStep{
			ID: prefix + ".start", Component: component.Name, NodeID: nodeID, Phase: PhaseStart,
			Command: startCommand, Candidates: startCandidates, SelectedCandidate: selectedCandidate,
			WorkDir: compWorkDir, DependsOn: uniqueStrings(deps),
			Explanation: "Start the component after all topology dependencies are started.",
		})
	}

	services := append([]Service(nil), model.Services...)
	sort.SliceStable(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, service := range services {
		if strings.TrimSpace(service.Name) == "" {
			return ExecutionPlan{}, newFailure(FailureInvalidService, "", "Octo found a service without a name.", "The service metadata is incomplete.")
		}
		if isComponentServiceWrapper(service, model.Components) {
			continue
		}
		nodeID := topologyID(NodeService, service.Name)
		stepID := startStepID(nodeID)
		deps := make([]string, 0)
		for _, edge := range topology.Edges {
			if edge.From == nodeID && edge.Kind == "depends_on" {
				deps = append(deps, readinessStepID(edge.To, model))
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

	ports, err := AllocateComponentPortsWithServices(model.Components, model.Services)
	if err != nil {
		return ExecutionPlan{}, err
	}
	portByComponent := make(map[string]int, len(ports))
	for _, assignment := range ports {
		portByComponent[assignment.Component] = assignment.Resolved
	}
	for i := range steps {
		if steps[i].Phase == PhaseStart {
			port, hasPort := portByComponent[steps[i].Component]
			comp := findComponentByName(model.Components, steps[i].Component)
			if isLongRunningComponent(comp, steps[i].Command, hasPort) {
				steps[i].LongRunning = true
			}
			if hasPort {
				if steps[i].Environment == nil {
					steps[i].Environment = make(map[string]string)
				}
				steps[i].Environment["PORT"] = fmt.Sprintf("%d", port)
			}
		}
	}

	plan := ExecutionPlan{
		ProjectName: model.Name,
		Root: model.Root,
		Topology: topology,
		Steps: steps,
		Ports: ports,
		Provisioning: provisioning,
		Environment: ResolveEnvironmentBindings(model),
	}
	if err := ValidateExecutionPlan(plan); err != nil {
		return ExecutionPlan{}, newFailure(FailurePlanValidation, "", "Octo generated an invalid execution plan.", err.Error())
	}
	return plan, nil
}

func executionDecisionRequest(componentName string, candidates []ExecutionCandidate) DecisionRequest {
	options := make([]DecisionOption, 0, len(candidates))
	for _, candidate := range candidates {
		options = append(options, DecisionOption{
			Kind: candidate.Kind,
			ID: candidate.ID, Value: candidate.Command, Confidence: candidate.Confidence, Evidence: candidate.Evidence,
		})
	}
	return DecisionRequest{Name: "Choose how to run " + componentName, Options: options}
}

func selectedCandidateID(component Component) string {
	for _, candidate := range component.ExecutionCandidates {
		if candidate.Command == component.RunCommand {
			return candidate.ID
		}
	}
	return ""
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
		if runtime.GOOS == "windows" {
			return `if not exist .venv if not exist venv (python -m venv .venv) && if exist venv\Scripts\python.exe (venv\Scripts\python.exe -m pip install -r requirements.txt) else (.venv\Scripts\python.exe -m pip install -r requirements.txt)`
		}
		return `if [ ! -d ".venv" ] && [ ! -d "venv" ]; then (python3 -m venv .venv || python -m venv .venv || { echo "Python virtualenv module (venv) is missing. On Debian/Ubuntu, install python3-venv: sudo apt install python3-venv" >&2; exit 1; }); fi && if [ -d "venv" ] && [ ! -d ".venv" ]; then venv/bin/python -m pip install -r requirements.txt; else .venv/bin/python -m pip install -r requirements.txt; fi`
	case "uv":
		return "uv sync"
	case "poetry":
		return "poetry install"
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
	for _, comp := range model.Components {
		if topologyID(NodeComponent, comp.Name) == nodeID {
			if comp.Role == "library" || (comp.RunCommand == "" && len(primaryExecutionCandidates(comp.ExecutionCandidates)) == 0) {
				return lastPreparationStepID(comp, model)
			}
		}
	}
	return startStepID(nodeID)
}

func lastPreparationStepID(comp Component, model ProjectModel) string {
	prefix := "component." + comp.Name
	if buildCmd := detectComponentBuildCommand(model.Root, comp); buildCmd != "" {
		return prefix + ".build"
	}
	if strings.TrimSpace(model.SetupCommand) != "" && comp.Name == model.Name {
		return prefix + ".setup"
	}
	if hasNodeWorkspace(model.Root) && (comp.Language == "Node" || comp.PackageManager != "") {
		return "workspace.root.install"
	}
	if installCommand(comp.PackageManager) != "" {
		return prefix + ".install"
	}
	return prefix + ".prepare"
}

func isLongRunningComponent(component *Component, command string, hasPort bool) bool {
	if hasPort {
		return true
	}
	if component == nil {
		return false
	}
	if component.Role == "service" || component.Role == "worker" {
		return true
	}

	cmdLower := strings.ToLower(command)
	nameLower := strings.ToLower(component.Name)
	pathLower := strings.ToLower(component.Path)

	// Framework dev servers or watch scripts
	if component.Framework != "" {
		for _, devToken := range []string{"dev", "start", "serve", "server", "watch"} {
			if strings.Contains(cmdLower, devToken) {
				return true
			}
		}
	}

	// Worker / daemon / queue / long-running server tokens
	workerTokens := []string{
		"worker", "daemon", "queue", "celery", "sidekiq", "consumer",
		"listen", "scheduler",
	}
	for _, tok := range workerTokens {
		if strings.Contains(cmdLower, tok) || strings.Contains(nameLower, tok) || strings.Contains(pathLower, tok) {
			return true
		}
	}

	// Evidence from candidate IDs
	for _, candidate := range component.ExecutionCandidates {
		if candidate.Command == command {
			idLower := strings.ToLower(candidate.ID)
			for _, tok := range []string{"worker", "daemon", "queue", "server", "serve"} {
				if strings.Contains(idLower, tok) && !strings.HasPrefix(idLower, "node.script.") {
					return true
				}
			}
		}
	}

	return false
}

func findComponentByName(components []Component, name string) *Component {
	for i := range components {
		if components[i].Name == name {
			return &components[i]
		}
	}
	return nil
}

func detectComponentBuildCommand(root string, component Component) string {
	compDir := componentWorkDir(root, component.Path)
	if data, err := os.ReadFile(filepath.Join(compDir, "package.json")); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if _, ok := pkg.Scripts["build"]; ok {
				pm := component.PackageManager
				if pm == "" {
					pm = "npm"
				}
				if pm == "npm" {
					return "npm run build"
				}
				return pm + " build"
			}
		}
	}
	if _, err := os.Stat(filepath.Join(compDir, "Cargo.toml")); err == nil {
		return "cargo build"
	}
	return ""
}

func detectRootNodePackageManager(root string) string {
	if _, err := os.Stat(filepath.Join(root, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(root, "pnpm-workspace.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(root, "bun.lock")); err == nil {
		return "bun"
	}
	if _, err := os.Stat(filepath.Join(root, "bun.lockb")); err == nil {
		return "bun"
	}
	if _, err := os.Stat(filepath.Join(root, "package-lock.json")); err == nil {
		return "npm"
	}
	return "npm"
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


func provisioningRequirementFor(component Component, requirements []ProvisioningRequirement) *ProvisioningRequirement {
	for i := range requirements {
		if requirements[i].Name == component.PackageManager {
			return &requirements[i]
		}
	}
	return nil
}

func componentWorkDir(root, compPath string) string {
	if root == "" {
		return compPath
	}
	if compPath == "" || compPath == "." {
		return root
	}
	if filepath.IsAbs(compPath) {
		return compPath
	}
	return filepath.Join(root, filepath.FromSlash(compPath))
}

func isComponentServiceWrapper(service Service, components []Component) bool {
	if service.Build == "" {
		return false
	}
	buildPath := filepath.Clean(service.Build)
	for _, comp := range components {
		compPath := filepath.Clean(comp.Path)
		if buildPath == compPath || (compPath == "." && (buildPath == "." || buildPath == "")) {
			if comp.RunCommand != "" || len(comp.ExecutionCandidates) > 0 {
				return true
			}
		}
	}
	return false
}

func provisionCheckCommand(name string) string {
	if runtime.GOOS == "windows" {
		return "where " + name
	}
	return "command -v " + name
}


