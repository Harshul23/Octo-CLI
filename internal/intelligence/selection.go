package intelligence

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// TargetKind identifies the type of an execution target.
type TargetKind string

const (
	TargetKindComponent TargetKind = "component"
	TargetKindService   TargetKind = "service"
)

// TargetID is a typed, unambiguous identifier for a component or service.
type TargetID struct {
	Kind TargetKind `json:"kind" yaml:"kind"`
	Name string     `json:"name" yaml:"name"`
}

func (t TargetID) String() string {
	return string(t.Kind) + ":" + t.Name
}

// ParseTargetID parses a typed target string formatted as "component:name" or "service:name".
func ParseTargetID(s string) (TargetID, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return TargetID{}, fmt.Errorf("invalid target ID %q: expected format <kind>:<name> (e.g. component:web or service:postgres)", s)
	}
	kind := TargetKind(strings.ToLower(parts[0]))
	if kind != TargetKindComponent && kind != TargetKindService {
		return TargetID{}, fmt.Errorf("invalid target kind %q in %q (expected 'component' or 'service')", parts[0], s)
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return TargetID{}, fmt.Errorf("empty target name in %q", s)
	}
	return TargetID{Kind: kind, Name: name}, nil
}

// TargetInfo provides detailed metadata about a detected target for display and selection.
type TargetInfo struct {
	ID         TargetID   `json:"id" yaml:"id"`
	Name       string     `json:"name" yaml:"name"`
	Kind       TargetKind `json:"kind" yaml:"kind"`
	Role       string     `json:"role" yaml:"role"` // "application", "library", "service"
	Path       string     `json:"path" yaml:"path"`
	Runtime    string     `json:"runtime" yaml:"runtime"`
	Command    string     `json:"command,omitempty" yaml:"command,omitempty"`
	Port       int        `json:"port,omitempty" yaml:"port,omitempty"`
	PortInfo   string     `json:"port_info,omitempty" yaml:"port_info,omitempty"`
	DependsOn  []string   `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	IsRunnable bool       `json:"is_runnable" yaml:"is_runnable"`
	IsWrapper  bool       `json:"is_wrapper" yaml:"is_wrapper"`
	Reason     string     `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// SelectionOptions specifies target filtering options for CLI flags and MCP tool calls.
type SelectionOptions struct {
	Only []string `json:"only,omitempty" yaml:"only,omitempty"` // Repeatable target selectors (e.g. "component:web", "service:postgres")
	All  bool     `json:"all,omitempty" yaml:"all,omitempty"`   // Select all runnable targets without prompting
}

// ResolvedSelection is the result of resolving target selection and transitive dependencies.
type ResolvedSelection struct {
	RequestedTargets   []TargetID          `json:"requested_targets"`
	AllTargets         []TargetInfo        `json:"all_targets"`
	SelectedTargets    []TargetID          `json:"selected_targets"`
	SelectedComponents []Component         `json:"selected_components"`
	SelectedServices   []Service           `json:"selected_services"`
	ForcedDependencies map[TargetID]string `json:"forced_dependencies"` // TargetID -> reason (e.g. "dependency of component:web")
	FilteredModel      ProjectModel        `json:"filtered_model"`
}

// AllTargets returns all detected components and services as TargetInfo items.
func AllTargets(model ProjectModel) []TargetInfo {
	var targets []TargetInfo

	for _, comp := range model.Components {
		role := comp.Role
		if role == "" {
			if comp.RunCommand != "" || len(comp.ExecutionCandidates) > 0 || comp.Language != "" || comp.Framework != "" || comp.Port > 0 {
				role = "application"
			} else {
				role = "library"
			}
		}
		isRunnable := role != "library"

		badgeParts := []string{}
		if comp.Language != "" {
			badgeParts = append(badgeParts, strings.ToUpper(comp.Language))
		}
		if comp.Framework != "" {
			badgeParts = append(badgeParts, strings.ToUpper(comp.Framework))
		}
		badge := strings.Join(badgeParts, " / ")
		if badge == "" {
			badge = "COMPONENT"
		}

		portStr := ""
		if comp.Port > 0 {
			portStr = fmt.Sprintf("→ port %d", comp.Port)
		}

		targets = append(targets, TargetInfo{
			ID:         TargetID{Kind: TargetKindComponent, Name: comp.Name},
			Name:       comp.Name,
			Kind:       TargetKindComponent,
			Role:       role,
			Path:       comp.Path,
			Runtime:    badge,
			Command:    comp.RunCommand,
			Port:       comp.Port,
			PortInfo:   portStr,
			DependsOn:  comp.DependsOn,
			IsRunnable: isRunnable,
			Reason:     comp.NonRunnableReason,
		})
	}

	for _, svc := range model.Services {
		isWrapper := isComponentServiceWrapper(svc, model.Components)
		portStr := ""
		if len(svc.Ports) > 0 {
			portStr = "→ " + strings.Join(svc.Ports, ", ")
		}

		runtime := svc.Image
		if runtime == "" && svc.Build != "" {
			runtime = fmt.Sprintf("build context: %s", svc.Build)
		}

		targets = append(targets, TargetInfo{
			ID:         TargetID{Kind: TargetKindService, Name: svc.Name},
			Name:       svc.Name,
			Kind:       TargetKindService,
			Role:       "service",
			Path:       svc.Image,
			Runtime:    runtime,
			PortInfo:   portStr,
			DependsOn:  svc.DependsOn,
			IsRunnable: !isWrapper,
			IsWrapper:  isWrapper,
			Reason:     map[bool]string{true: "Compose build wrapper for local component"}[isWrapper],
		})
	}

	return targets
}

// RunnableTargets returns only the targets that can be directly launched by the user.
func RunnableTargets(model ProjectModel) []TargetInfo {
	all := AllTargets(model)
	var runnable []TargetInfo
	for _, t := range all {
		if t.IsRunnable {
			runnable = append(runnable, t)
		}
	}
	return runnable
}

// FormatAvailableTargets produces a formatted bulleted list of available runnable targets.
func FormatAvailableTargets(model ProjectModel) string {
	runnable := RunnableTargets(model)
	var lines []string
	for _, t := range runnable {
		details := []string{}
		if t.Path != "" && t.Path != "." {
			details = append(details, t.Path)
		}
		if t.Runtime != "" {
			details = append(details, t.Runtime)
		}
		if t.Command != "" {
			details = append(details, t.Command)
		}
		if t.PortInfo != "" {
			details = append(details, t.PortInfo)
		}
		desc := ""
		if len(details) > 0 {
			desc = fmt.Sprintf(" (%s)", strings.Join(details, ", "))
		}
		lines = append(lines, fmt.Sprintf("  - %s%s", t.ID.String(), desc))
	}
	return strings.Join(lines, "\n")
}

// ResolveTargetSelector resolves a single selector string (e.g. "component:web", "service:postgres", or "web") to a TargetID.
func ResolveTargetSelector(selector string, model ProjectModel) (TargetID, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return TargetID{}, fmt.Errorf("empty target selector")
	}

	if strings.Contains(selector, ":") {
		parsed, err := ParseTargetID(selector)
		if err != nil {
			return TargetID{}, err
		}
		if parsed.Kind == TargetKindComponent {
			for _, c := range model.Components {
				if c.Name == parsed.Name {
					if c.Role == "library" {
						return TargetID{}, fmt.Errorf("target %q is a workspace library and cannot be requested as a runnable launch target (libraries are included as build/install dependencies)", selector)
					}
					return parsed, nil
				}
			}
			return TargetID{}, fmt.Errorf("unknown component %q; available targets:\n%s", selector, FormatAvailableTargets(model))
		}
		if parsed.Kind == TargetKindService {
			for _, s := range model.Services {
				if s.Name == parsed.Name {
					if isComponentServiceWrapper(s, model.Components) {
						// Wraps a local component: resolve to the local component to avoid starting twice
						for _, c := range model.Components {
							if isServiceWrappingComponent(s, c) {
								return TargetID{Kind: TargetKindComponent, Name: c.Name}, nil
							}
						}
					}
					return parsed, nil
				}
			}
			return TargetID{}, fmt.Errorf("unknown service %q; available targets:\n%s", selector, FormatAvailableTargets(model))
		}
		return TargetID{}, fmt.Errorf("unknown target %q", selector)
	}

	// Bare name selector
	var matchedComp *Component
	var isLib bool
	for i := range model.Components {
		if model.Components[i].Name == selector {
			if model.Components[i].Role == "library" {
				isLib = true
			} else {
				matchedComp = &model.Components[i]
			}
		}
	}

	var matchedSvc *Service
	for i := range model.Services {
		if model.Services[i].Name == selector {
			matchedSvc = &model.Services[i]
		}
	}

	if matchedComp != nil && matchedSvc != nil {
		if isComponentServiceWrapper(*matchedSvc, model.Components) {
			return TargetID{Kind: TargetKindComponent, Name: matchedComp.Name}, nil
		}
		return TargetID{}, fmt.Errorf("ambiguous selector %q matches both component:%s and service:%s; specify an explicit typed selector", selector, selector, selector)
	}

	if matchedComp != nil {
		return TargetID{Kind: TargetKindComponent, Name: matchedComp.Name}, nil
	}

	if matchedSvc != nil {
		if isComponentServiceWrapper(*matchedSvc, model.Components) {
			for _, c := range model.Components {
				if isServiceWrappingComponent(*matchedSvc, c) {
					return TargetID{Kind: TargetKindComponent, Name: c.Name}, nil
				}
			}
		}
		return TargetID{Kind: TargetKindService, Name: matchedSvc.Name}, nil
	}

	if isLib {
		return TargetID{}, fmt.Errorf("target %q is a workspace library and cannot be requested as a runnable launch target (libraries are included as build/install dependencies)", selector)
	}

	return TargetID{}, fmt.Errorf("unknown selector %q; available targets:\n%s", selector, FormatAvailableTargets(model))
}

// ResolveSelection resolves target selection options, computing full transitive dependencies.
func ResolveSelection(model ProjectModel, opts SelectionOptions) (ResolvedSelection, error) {
	if len(opts.Only) > 0 && opts.All {
		return ResolvedSelection{}, fmt.Errorf("cannot combine --only and --all; choose either specific targets or --all")
	}

	runnable := RunnableTargets(model)

	if opts.All {
		requested := make([]TargetID, len(runnable))
		for i, r := range runnable {
			requested[i] = r.ID
		}
		return ResolveSelectionTargets(model, requested)
	}

	if len(opts.Only) > 0 {
		requested := make([]TargetID, 0, len(opts.Only))
		seen := make(map[TargetID]bool)
		for _, sel := range opts.Only {
			targetID, err := ResolveTargetSelector(sel, model)
			if err != nil {
				return ResolvedSelection{}, err
			}
			if !seen[targetID] {
				seen[targetID] = true
				requested = append(requested, targetID)
			}
		}
		return ResolveSelectionTargets(model, requested)
	}

	// Neither --only nor --all specified
	if len(runnable) <= 1 {
		requested := make([]TargetID, len(runnable))
		for i, r := range runnable {
			requested[i] = r.ID
		}
		return ResolveSelectionTargets(model, requested)
	}

	return ResolvedSelection{}, fmt.Errorf("multiple runnable targets detected; specify --only <target> or --all in non-interactive mode.\nAvailable targets:\n%s", FormatAvailableTargets(model))
}

// ResolveSelectionTargets computes the transitive dependency closure for requested target IDs.
func ResolveSelectionTargets(model ProjectModel, requested []TargetID) (ResolvedSelection, error) {
	allTargets := AllTargets(model)

	// Validate requested targets exist and are not libraries
	compMap := make(map[string]Component, len(model.Components))
	for _, c := range model.Components {
		compMap[c.Name] = c
	}
	svcMap := make(map[string]Service, len(model.Services))
	for _, s := range model.Services {
		svcMap[s.Name] = s
	}

	dedupedRequested := make([]TargetID, 0, len(requested))
	requestedSet := make(map[TargetID]bool)
	for _, req := range requested {
		if req.Kind == TargetKindComponent {
			c, ok := compMap[req.Name]
			if !ok {
				return ResolvedSelection{}, fmt.Errorf("selected target %q no longer exists", req.String())
			}
			if c.Role == "library" {
				return ResolvedSelection{}, fmt.Errorf("target %q is a workspace library and cannot be requested as a runnable launch target (libraries are included as build/install dependencies)", req.String())
			}
		} else if req.Kind == TargetKindService {
			s, ok := svcMap[req.Name]
			if !ok {
				return ResolvedSelection{}, fmt.Errorf("selected target %q no longer exists", req.String())
			}
			if isComponentServiceWrapper(s, model.Components) {
				// Re-map wrapper to local component
				for _, c := range model.Components {
					if isServiceWrappingComponent(s, c) {
						req = TargetID{Kind: TargetKindComponent, Name: c.Name}
						break
					}
				}
			}
		} else {
			return ResolvedSelection{}, fmt.Errorf("invalid target kind %q in %s", req.Kind, req.String())
		}

		if !requestedSet[req] {
			requestedSet[req] = true
			dedupedRequested = append(dedupedRequested, req)
		}
	}

	// Adjacency resolution for explicit depends_on
	resolveDepTarget := func(depName string, source TargetID) (TargetID, error) {
		if strings.Contains(depName, ":") {
			parsed, err := ParseTargetID(depName)
			if err != nil {
				return TargetID{}, fmt.Errorf("target %s has invalid dependency %q: %w", source.String(), depName, err)
			}
			if parsed.Kind == TargetKindComponent {
				if _, ok := compMap[parsed.Name]; !ok {
					return TargetID{}, fmt.Errorf("target %s depends on unknown target %q", source.String(), depName)
				}
				return parsed, nil
			}
			if parsed.Kind == TargetKindService {
				s, ok := svcMap[parsed.Name]
				if !ok {
					return TargetID{}, fmt.Errorf("target %s depends on unknown target %q", source.String(), depName)
				}
				if isComponentServiceWrapper(s, model.Components) {
					for _, c := range model.Components {
						if isServiceWrappingComponent(s, c) {
							return TargetID{Kind: TargetKindComponent, Name: c.Name}, nil
						}
					}
				}
				return parsed, nil
			}
			return TargetID{}, fmt.Errorf("target %s depends on unknown target %q", source.String(), depName)
		}

		// Bare name in depends_on
		s, hasSvc := svcMap[depName]
		c, hasComp := compMap[depName]

		if hasSvc && hasComp && !isComponentServiceWrapper(s, model.Components) {
			return TargetID{}, fmt.Errorf("ambiguous dependency %q in target %s: matches both component and service", depName, source.String())
		}
		if hasSvc {
			if isComponentServiceWrapper(s, model.Components) {
				for _, comp := range model.Components {
					if isServiceWrappingComponent(s, comp) {
						return TargetID{Kind: TargetKindComponent, Name: comp.Name}, nil
					}
				}
			}
			return TargetID{Kind: TargetKindService, Name: s.Name}, nil
		}
		if hasComp {
			return TargetID{Kind: TargetKindComponent, Name: c.Name}, nil
		}
		return TargetID{}, fmt.Errorf("target %s depends on unknown target %q", source.String(), depName)
	}

	getDependencies := func(id TargetID) ([]TargetID, error) {
		var deps []TargetID
		if id.Kind == TargetKindComponent {
			comp := compMap[id.Name]
			for _, depName := range comp.DependsOn {
				depID, err := resolveDepTarget(depName, id)
				if err != nil {
					return nil, err
				}
				deps = append(deps, depID)
			}
		} else if id.Kind == TargetKindService {
			svc := svcMap[id.Name]
			for _, depName := range svc.DependsOn {
				depID, err := resolveDepTarget(depName, id)
				if err != nil {
					return nil, err
				}
				deps = append(deps, depID)
			}
		}
		return deps, nil
	}

	// Transitive closure and cycle detection via DFS
	visited := make(map[TargetID]bool)
	forcedDependencies := make(map[TargetID]string)
	visiting := make(map[TargetID]bool)

	var dfs func(curr TargetID, path []TargetID) error
	dfs = func(curr TargetID, path []TargetID) error {
		if visiting[curr] {
			// Cycle detected
			cycle := append([]TargetID(nil), path...)
			cycle = append(cycle, curr)
			cycleStrings := make([]string, len(cycle))
			for i, id := range cycle {
				cycleStrings[i] = id.String()
			}
			return fmt.Errorf("dependency cycle detected: %s", strings.Join(cycleStrings, " -> "))
		}
		if visited[curr] {
			return nil
		}

		visiting[curr] = true
		deps, err := getDependencies(curr)
		if err != nil {
			return err
		}

		for _, dep := range deps {
			if !requestedSet[dep] && forcedDependencies[dep] == "" {
				forcedDependencies[dep] = fmt.Sprintf("required by %s", curr.String())
			}
			if err := dfs(dep, append(path, curr)); err != nil {
				return err
			}
		}

		visiting[curr] = false
		visited[curr] = true
		return nil
	}

	for _, req := range dedupedRequested {
		if err := dfs(req, nil); err != nil {
			return ResolvedSelection{}, err
		}
	}

	// Partition visited into components and services
	var selectedComponents []Component
	for _, c := range model.Components {
		id := TargetID{Kind: TargetKindComponent, Name: c.Name}
		if visited[id] {
			selectedComponents = append(selectedComponents, c)
		}
	}
	sort.Slice(selectedComponents, func(i, j int) bool {
		return selectedComponents[i].Path < selectedComponents[j].Path
	})

	var selectedServices []Service
	for _, s := range model.Services {
		id := TargetID{Kind: TargetKindService, Name: s.Name}
		if visited[id] {
			// If this service is a build wrapper for a component that is already selected, skip starting it as a container
			if isComponentServiceWrapper(s, selectedComponents) {
				continue
			}
			selectedServices = append(selectedServices, s)
		}
	}
	sort.Slice(selectedServices, func(i, j int) bool {
		return selectedServices[i].Name < selectedServices[j].Name
	})

	selectedTargets := make([]TargetID, 0, len(selectedComponents)+len(selectedServices))
	for _, c := range selectedComponents {
		selectedTargets = append(selectedTargets, TargetID{Kind: TargetKindComponent, Name: c.Name})
	}
	for _, s := range selectedServices {
		selectedTargets = append(selectedTargets, TargetID{Kind: TargetKindService, Name: s.Name})
	}

	// Build FilteredModel
	filteredModel := model
	filteredModel.Components = selectedComponents
	filteredModel.Services = selectedServices
	filteredModel.Monorepo = len(selectedComponents) > 1 || (len(selectedComponents) == 1 && selectedComponents[0].Path != ".")

	// Filter CustomVerification
	selectedNames := make(map[string]bool)
	for _, c := range selectedComponents {
		selectedNames[c.Name] = true
	}
	for _, s := range selectedServices {
		selectedNames[s.Name] = true
	}
	var filteredChecks []VerificationCheck
	for _, check := range model.CustomVerification {
		if selectedNames[check.Component] {
			filteredChecks = append(filteredChecks, check)
		}
	}
	filteredModel.CustomVerification = filteredChecks

	filteredModel.Environment.Resolutions = ResolveEnvironmentBindings(filteredModel)

	return ResolvedSelection{
		RequestedTargets:   dedupedRequested,
		AllTargets:         allTargets,
		SelectedTargets:    selectedTargets,
		SelectedComponents: selectedComponents,
		SelectedServices:   selectedServices,
		ForcedDependencies: forcedDependencies,
		FilteredModel:      filteredModel,
	}, nil
}

func isServiceWrappingComponent(svc Service, comp Component) bool {
	if svc.Build == "" {
		return false
	}
	buildPath := filepath.Clean(svc.Build)
	compPath := filepath.Clean(comp.Path)
	return buildPath == compPath || (compPath == "." && (buildPath == "." || buildPath == ""))
}
