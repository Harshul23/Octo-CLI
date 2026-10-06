package intelligence

import (
	"strings"
)

// ProvisioningRequirement describes a prerequisite that must exist before
// repository execution can safely begin.
type ProvisioningRequirement struct {
	ID          string `json:"id" yaml:"id"`
	Kind        string `json:"kind" yaml:"kind"`
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version,omitempty" yaml:"version,omitempty"`
	Component   string `json:"component,omitempty" yaml:"component,omitempty"`
	Explanation string `json:"explanation,omitempty" yaml:"explanation,omitempty"`
}

// BuildProvisioningRequirements derives machine prerequisites from repository facts.
// It does not mutate the machine.
func BuildProvisioningRequirements(model ProjectModel) []ProvisioningRequirement {
	seen := make(map[string]struct{})
	requirements := make([]ProvisioningRequirement, 0, len(model.Components))

	for _, component := range model.Components {
		name := component.PackageManager
		if name == "" {
			continue
		}
		id := "command." + name
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		requirements = append(requirements, ProvisioningRequirement{
			ID:          id,
			Kind:        "command",
			Name:        name,
			Component:   component.Name,
			Explanation: packageManagerExplanation(name),
		})
	}

	// Service provisioning requirements (e.g. Docker for Compose infrastructure services)
	if len(model.Services) > 0 {
		id := "service.docker"
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			requirements = append(requirements, ProvisioningRequirement{
				ID:          id,
				Kind:        "service",
				Name:        "docker",
				Explanation: "Docker is required to provision Compose infrastructure services.",
			})
		}
	}

	return requirements
}

func packageManagerExplanation(name string) string {
	switch strings.ToLower(name) {
	case "pnpm":
		return "pnpm package manager is required (install via 'corepack enable' or 'npm install -g pnpm')."
	case "yarn":
		return "Yarn package manager is required (install via 'corepack enable' or 'npm install -g yarn')."
	case "poetry":
		return "Poetry package manager is required (install via 'pipx install poetry' or https://python-poetry.org)."
	case "uv":
		return "uv package manager is required (install via https://astral.sh/uv)."
	case "cargo":
		return "Cargo package manager is required (install via https://rustup.rs)."
	case "go":
		return "Go toolchain is required (install via https://go.dev/dl)."
	default:
		return "Required package manager detected from repository metadata."
	}
}
