package blueprint

import (
	"errors"
	"os"

	"github.com/harshul/octo-cli/internal/analyzer"
	"gopkg.in/yaml.v3"
)

// Blueprint is a configuration derived from project analysis.
type Blueprint struct {
	Name       string `yaml:"name"`
	Language   string `yaml:"language,omitempty"`
	Version    string `yaml:"version,omitempty"`
	RunCommand string `yaml:"run,omitempty"`
}

// FromAnalysis converts an analysis result into a basic blueprint.
func FromAnalysis(a analyzer.Analysis) Blueprint {
	return Blueprint{Name: a.Name}
}

// FromProjectInfo converts a ProjectInfo result into a full blueprint.
func FromProjectInfo(p analyzer.ProjectInfo) Blueprint {
	return Blueprint{
		Name:       p.Name,
		Language:   p.Language,
		Version:    p.Version,
		RunCommand: p.RunCommand,
	}
}

// Write writes the blueprint as a YAML file.
func Write(path string, bp Blueprint) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Marshal the blueprint to YAML
	data, err := yaml.Marshal(&bp)
	if err != nil {
		return err
	}

	// Write the YAML content
	_, err = f.Write(data)
	return err
}

// Read reads a YAML-like file and extracts the blueprint fields.
func Read(path string) (Blueprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Blueprint{}, err
	}

	var bp Blueprint
	if err := yaml.Unmarshal(data, &bp); err != nil {
		return Blueprint{}, err
	}

	if bp.Name == "" {
		return Blueprint{}, errors.New("invalid configuration: missing name")
	}

	return bp, nil
}