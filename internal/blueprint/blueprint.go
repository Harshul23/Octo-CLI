package blueprint

import (
	"errors"
	"os"

	"github.com/harshul/octo-cli/internal/analyzer"
	"gopkg.in/yaml.v3"
)

// ThermalConfig holds thermal and resource management settings
type ThermalConfig struct {
	// Concurrency is the maximum number of concurrent operations (0 = auto-detect)
	Concurrency int `yaml:"concurrency,omitempty"`
	// BatchSize is the number of projects to process in each batch (0 = auto-detect)
	BatchSize int `yaml:"batch_size,omitempty"`
	// CoolDownMs is the delay between batches in milliseconds (0 = use default)
	CoolDownMs int `yaml:"cool_down_ms,omitempty"`
	// Mode is the thermal mode ("auto", "cool", "performance")
	// - "auto": Automatically detect and adjust based on hardware
	// - "cool": Prioritize low temperatures over speed
	// - "performance": Use maximum resources regardless of thermals
	Mode string `yaml:"mode,omitempty"`
}

// Blueprint is a configuration derived from project analysis.
type Blueprint struct {
	Name           string        `yaml:"name"`
	Language       string        `yaml:"language,omitempty"`
	Version        string        `yaml:"version,omitempty"`
	RunCommand     string        `yaml:"run,omitempty"`
	SetupCommand   string        `yaml:"setup,omitempty"`
	SetupRequired  bool          `yaml:"setup_required,omitempty"`
	PackageManager string        `yaml:"package_manager,omitempty"`
	IsMonorepo     bool          `yaml:"is_monorepo,omitempty"`
	MonorepoRoot   string        `yaml:"monorepo_root,omitempty"`
	EnvVars        []EnvVar      `yaml:"env_vars,omitempty"`
	Thermal        ThermalConfig `yaml:"thermal,omitempty"`
}

// EnvVar represents a required environment variable
type EnvVar struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

// FromAnalysis converts an analysis result into a basic blueprint.
func FromAnalysis(a analyzer.Analysis) Blueprint {
	return Blueprint{Name: a.Name}
}

// FromProjectInfo converts a ProjectInfo result into a full blueprint.
func FromProjectInfo(p analyzer.ProjectInfo) Blueprint {
	return Blueprint{
		Name:           p.Name,
		Language:       p.Language,
		Version:        p.Version,
		RunCommand:     p.RunCommand,
		SetupCommand:   p.SetupCommand,
		SetupRequired:  p.SetupRequired,
		PackageManager: p.PackageManager,
		IsMonorepo:     p.IsMonorepo,
		MonorepoRoot:   p.MonorepoRoot,
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