package intelligence

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// RepositoryOverride represents explicit developer/maintainer strategy overrides from .octo.yaml.
type RepositoryOverride struct {
	Spec         string                       `json:"spec,omitempty" yaml:"spec,omitempty"`
	Project      OverrideProject              `json:"project,omitempty" yaml:"project,omitempty"`
	Components   map[string]OverrideComponent `json:"components,omitempty" yaml:"components,omitempty"`
	Services     map[string]OverrideService   `json:"services,omitempty" yaml:"services,omitempty"`
	Verification []OverrideVerificationCheck  `json:"verification,omitempty" yaml:"verification,omitempty"`
}

type OverrideProject struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type OverrideComponent struct {
	Command     OverrideCommand   `json:"command,omitempty" yaml:"command,omitempty"`
	RunCommand  string            `json:"run_command,omitempty" yaml:"run_command,omitempty"`
	Port        int               `json:"port,omitempty" yaml:"port,omitempty"`
	Ports       []int             `json:"ports,omitempty" yaml:"ports,omitempty"`
	Environment map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

type OverrideCommand struct {
	Dev   string `json:"dev,omitempty" yaml:"dev,omitempty"`
	Start string `json:"start,omitempty" yaml:"start,omitempty"`
}

type OverrideService struct {
	Image     string   `json:"image,omitempty" yaml:"image,omitempty"`
	Ports     []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

type OverrideVerificationCheck struct {
	Component      string `json:"component" yaml:"component"`
	Kind           string `json:"kind" yaml:"kind"` // "http", "port", "health"
	Path           string `json:"path,omitempty" yaml:"path,omitempty"`
	Port           int    `json:"port,omitempty" yaml:"port,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty" yaml:"expected_status,omitempty"`
	Command        string `json:"command,omitempty" yaml:"command,omitempty"`
}

var overrideFileCandidates = []string{
	".octo.yaml",
	".octo.yml",
	filepath.Join(".octo", "config.yaml"),
	filepath.Join(".octo", "config.yml"),
}

// FindOverrideFile checks for repository override configuration in root.
func FindOverrideFile(root string) string {
	for _, candidate := range overrideFileCandidates {
		target := filepath.Join(root, candidate)
		if _, err := os.Stat(target); err == nil {
			return target
		}
	}
	return ""
}

// LoadRepositoryOverride parses repository-level strategy overrides if present.
func LoadRepositoryOverride(root string) (*RepositoryOverride, string, error) {
	file := FindOverrideFile(root)
	if file == "" {
		return nil, "", nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, file, fmt.Errorf("read override config %q: %w", file, err)
	}
	var override RepositoryOverride
	if err := yaml.Unmarshal(data, &override); err != nil {
		return nil, file, fmt.Errorf("parse override config %q: %w", file, err)
	}
	return &override, file, nil
}

// ApplyRepositoryOverrides incorporates repository overrides into the ProjectModel with high confidence evidence.
func ApplyRepositoryOverrides(model *ProjectModel, override *RepositoryOverride, configPath string) {
	if override == nil || model == nil {
		return
	}

	relConfigPath, err := filepath.Rel(model.Root, configPath)
	if err != nil || relConfigPath == "" {
		relConfigPath = filepath.Base(configPath)
	}

	if override.Project.Name != "" {
		model.Name = override.Project.Name
	}

	// 1. Component Overrides
	for compName, compOverride := range override.Components {
		cmd := compOverride.RunCommand
		if cmd == "" {
			if compOverride.Command.Dev != "" {
				cmd = compOverride.Command.Dev
			} else if compOverride.Command.Start != "" {
				cmd = compOverride.Command.Start
			}
		}

		port := compOverride.Port
		if port == 0 && len(compOverride.Ports) > 0 {
			port = compOverride.Ports[0]
		}

		// Find existing component or append
		foundIdx := -1
		for i := range model.Components {
			if model.Components[i].Name == compName {
				foundIdx = i
				break
			}
		}

		overrideEvidence := Evidence{
			Kind:     EvidenceConfig,
			Path:     relConfigPath,
			Detail:   fmt.Sprintf("Explicit component override defined in %s", relConfigPath),
			Strength: 1.0,
		}

		if foundIdx >= 0 {
			comp := &model.Components[foundIdx]
			if cmd != "" {
				candidate := ExecutionCandidate{
					ID:         fmt.Sprintf("override.%s.run", compName),
					Command:    cmd,
					Confidence: 1.0,
					Evidence:   []Evidence{overrideEvidence},
				}
				comp.ExecutionCandidates = append([]ExecutionCandidate{candidate}, comp.ExecutionCandidates...)
				comp.RunCommand = cmd
				comp.Confidence = 1.0
			}
			if port > 0 {
				comp.Port = port
				comp.PortStrict = true
			}
			if len(compOverride.DependsOn) > 0 {
				comp.DependsOn = compOverride.DependsOn
			}
			comp.Evidence = append(comp.Evidence, overrideEvidence)
		} else {
			// New declared component from override
			newComp := Component{
				Name:        compName,
				Path:        ".",
				RunCommand:  cmd,
				Port:        port,
				PortStrict:  port > 0,
				DependsOn:   compOverride.DependsOn,
				Confidence:  1.0,
				Evidence:    []Evidence{overrideEvidence},
			}
			if cmd != "" {
				newComp.ExecutionCandidates = []ExecutionCandidate{{
					ID:         fmt.Sprintf("override.%s.run", compName),
					Command:    cmd,
					Confidence: 1.0,
					Evidence:   []Evidence{overrideEvidence},
				}}
			}
			model.Components = append(model.Components, newComp)
		}
	}

	// 2. Service Overrides
	for svcName, svcOverride := range override.Services {
		found := false
		for i := range model.Services {
			if model.Services[i].Name == svcName {
				found = true
				if svcOverride.Image != "" {
					model.Services[i].Image = svcOverride.Image
				}
				if len(svcOverride.Ports) > 0 {
					model.Services[i].Ports = svcOverride.Ports
				}
				if len(svcOverride.DependsOn) > 0 {
					model.Services[i].DependsOn = svcOverride.DependsOn
				}
				break
			}
		}
		if !found {
			model.Services = append(model.Services, Service{
				Name:      svcName,
				Image:     svcOverride.Image,
				Ports:     svcOverride.Ports,
				DependsOn: svcOverride.DependsOn,
				Evidence: []Evidence{{
					Kind:     EvidenceConfig,
					Path:     relConfigPath,
					Detail:   fmt.Sprintf("Service defined in %s", relConfigPath),
					Strength: 1.0,
				}},
			})
		}
	}

	// 3. Custom Verification Overrides
	for i, v := range override.Verification {
		kind := VerificationKind(v.Kind)
		if kind == "" {
			if v.Path != "" {
				kind = VerificationHTTP
			} else {
				kind = VerificationPort
			}
		}
		checkID := fmt.Sprintf("custom.%s.%d", v.Component, i+1)
		targetPort := v.Port
		if targetPort == 0 {
			for _, comp := range model.Components {
				if comp.Name == v.Component && comp.Port > 0 {
					targetPort = comp.Port
					break
				}
			}
		}
		model.CustomVerification = append(model.CustomVerification, VerificationCheck{
			ID:             checkID,
			Kind:           kind,
			Component:      v.Component,
			Host:           "127.0.0.1",
			Port:           targetPort,
			Path:           v.Path,
			ExpectedStatus: v.ExpectedStatus,
			Command:        v.Command,
			Timeout:        5 * time.Second,
		})
	}
}
