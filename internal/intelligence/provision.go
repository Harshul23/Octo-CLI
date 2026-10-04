package intelligence

// ProvisioningRequirement describes a prerequisite that must exist before
// repository execution can safely begin.
type ProvisioningRequirement struct {
	ID          string `json:"id" yaml:"id"`
	Kind        string `json:"kind" yaml:"kind"`
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version,omitempty" yaml:"version,omitempty"`
	Component  string `json:"component,omitempty" yaml:"component,omitempty"`
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
			ID: id,
			Kind: "command",
			Name: name,
			Component: component.Name,
			Explanation: "Required package manager detected from repository metadata.",
		})
	}
	return requirements
}
