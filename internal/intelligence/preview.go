package intelligence

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// MutationKind categorizes a machine-level side effect.
type MutationKind string

const (
	MutationFilesystem MutationKind = "filesystem"
	MutationNetwork    MutationKind = "network"
	MutationProcess    MutationKind = "process"
)

// MutationPreview describes an anticipated change to the host system.
type MutationPreview struct {
	Kind        MutationKind `json:"kind"`
	Target      string       `json:"target"`
	Action      string       `json:"action"`
	Explanation string       `json:"explanation"`
}

// NetworkPreview describes expected network port bindings.
type NetworkPreview struct {
	Component string `json:"component"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Action    string `json:"action"`
}

// ProcessPreview describes processes that will be spawned.
type ProcessPreview struct {
	Component   string `json:"component"`
	Command     string `json:"command"`
	Phase       string `json:"phase"`
	LongRunning bool   `json:"long_running"`
}

// MachinePreview gives AI agents and developers a comprehensive pre-execution impact assessment.
type MachinePreview struct {
	ProjectName   string                    `json:"project_name"`
	Root          string                    `json:"root"`
	Mutations     []MutationPreview         `json:"mutations"`
	Network       []NetworkPreview          `json:"network"`
	Processes     []ProcessPreview          `json:"processes"`
	Prerequisites []ProvisioningRequirement `json:"prerequisites"`
	RuntimeCheck  RuntimeCheck              `json:"runtime_check"`
	Environment   EnvironmentStatus         `json:"environment"`
}

// BuildMachinePreview inspects the model and plan to project all machine mutations
// before any command or side-effect is executed.
func BuildMachinePreview(ctx context.Context, model ProjectModel, plan ExecutionPlan) MachinePreview {
	preview := MachinePreview{
		ProjectName:   model.Name,
		Root:          model.Root,
		Prerequisites: plan.Provisioning,
		RuntimeCheck:  CheckRuntimeCompatibility(ctx, model),
		Environment:   InspectEnvironmentStatus(model.Root, model),
		Mutations:     make([]MutationPreview, 0),
		Network:       make([]NetworkPreview, 0),
		Processes:     make([]ProcessPreview, 0),
	}

	// 1. Filesystem mutations
	for _, step := range plan.Steps {
		if step.Phase == PhaseInstall {
			target := "dependencies"
			switch strings.ToLower(model.Language) {
			case "node", "javascript", "typescript":
				target = filepath.Join(step.WorkDir, "node_modules")
			case "python":
				target = filepath.Join(step.WorkDir, ".venv")
			case "rust":
				target = filepath.Join(step.WorkDir, "target")
			case "go":
				target = "Go module cache ($GOPATH/pkg/mod)"
			}
			preview.Mutations = append(preview.Mutations, MutationPreview{
				Kind:        MutationFilesystem,
				Target:      target,
				Action:      "populate / modify",
				Explanation: fmt.Sprintf("Step %s will execute %q to install dependencies.", step.ID, step.Command),
			})
		}
	}

	// Lockfile and execution state mutations
	preview.Mutations = append(preview.Mutations, MutationPreview{
		Kind:        MutationFilesystem,
		Target:      filepath.Join(model.Root, ".octo.lock"),
		Action:      "write / update",
		Explanation: "Verified candidate fingerprint and strategy will be recorded upon successful execution.",
	})

	// 2. Network port impact
	for _, port := range plan.Ports {
		preview.Network = append(preview.Network, NetworkPreview{
			Component: port.Component,
			Port:      port.Resolved,
			Protocol:  "tcp",
			Action:    "bind & verify listener",
		})
	}
	for _, svc := range model.Services {
		for _, p := range svc.Ports {
			preview.Network = append(preview.Network, NetworkPreview{
				Component: svc.Name,
				Port:      extractHostPort(p),
				Protocol:  "tcp",
				Action:    "container port mapping",
			})
		}
	}

	// 3. Process execution impact
	for _, step := range plan.Steps {
		if step.Command == "" {
			continue
		}
		preview.Processes = append(preview.Processes, ProcessPreview{
			Component:   step.Component,
			Command:     step.Command,
			Phase:       string(step.Phase),
			LongRunning: step.LongRunning,
		})
	}

	return preview
}

func extractHostPort(spec string) int {
	parts := strings.Split(spec, ":")
	if len(parts) >= 2 {
		var port int
		_, _ = fmt.Sscanf(parts[0], "%d", &port)
		return port
	}
	var port int
	_, _ = fmt.Sscanf(spec, "%d", &port)
	return port
}
