package intelligence

// EvidenceKind identifies where an inference came from.
type EvidenceKind string

const (
	EvidenceSignalFile EvidenceKind = "signal_file"
	EvidenceLockfile   EvidenceKind = "lockfile"
	EvidenceManifest   EvidenceKind = "manifest"
	EvidenceScript     EvidenceKind = "script"
	EvidenceConfig     EvidenceKind = "config"
)

// RelationshipKind identifies the semantics of a topology relationship.
// Execution engines MUST treat only DependsOn as an execution-order edge.
type RelationshipKind string

const (
	RelationshipDependsOn       RelationshipKind = "depends_on"
	RelationshipNetworkReference RelationshipKind = "network_reference"
)

// IsKnown reports whether the relationship kind is defined by the Octo v1 model.
func (k RelationshipKind) IsKnown() bool {
	switch k {
	case RelationshipDependsOn, RelationshipNetworkReference:
		return true
	default:
		return false
	}
}

// Evidence is a concrete fact used by Octo to explain an inference.
type Evidence struct {
	Kind EvidenceKind `json:"kind" yaml:"kind"`
	Path string `json:"path" yaml:"path"`
	Detail string `json:"detail" yaml:"detail"`
	Strength float64 `json:"strength" yaml:"strength"`
}

// Reference describes a runtime relationship discovered from repository evidence.
// References are intentionally separate from DependsOn because communication does
// not necessarily imply startup ordering.
type Reference struct {
	Target     string           `json:"target" yaml:"target"`
	Kind       RelationshipKind `json:"kind" yaml:"kind"`
	Variable   string           `json:"variable,omitempty" yaml:"variable,omitempty"`
	Confidence float64          `json:"confidence" yaml:"confidence"`
	Evidence   []Evidence       `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type HealthCheck struct {
	Command  string `json:"command,omitempty" yaml:"command,omitempty"`
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout  string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries  int    `json:"retries,omitempty" yaml:"retries,omitempty"`
}

type Service struct {
	Name        string         `json:"name" yaml:"name"`
	Image       string         `json:"image,omitempty" yaml:"image,omitempty"`
	Build       string         `json:"build,omitempty" yaml:"build,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	References  []Reference    `json:"references,omitempty" yaml:"references,omitempty"`
	Ports       []string       `json:"ports,omitempty" yaml:"ports,omitempty"`
	HealthCheck *HealthCheck   `json:"health_check,omitempty" yaml:"health_check,omitempty"`
	Confidence  float64        `json:"confidence" yaml:"confidence"`
	Evidence    []Evidence     `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type Component struct {
	Name          string      `json:"name" yaml:"name"`
	Path          string      `json:"path" yaml:"path"`
	Language      string      `json:"language,omitempty" yaml:"language,omitempty"`
	Framework     string      `json:"framework,omitempty" yaml:"framework,omitempty"`
	PackageManager string     `json:"package_manager,omitempty" yaml:"package_manager,omitempty"`
	RunCommand    string      `json:"run_command,omitempty" yaml:"run_command,omitempty"`
	DependsOn     []string    `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	References    []Reference `json:"references,omitempty" yaml:"references,omitempty"`
	Port          int         `json:"port,omitempty" yaml:"port,omitempty"`
	Confidence    float64     `json:"confidence" yaml:"confidence"`
	Evidence      []Evidence  `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// EnvironmentVariable describes an environment requirement without storing its value.
type EnvironmentVariable struct {
	Name    string   `json:"name" yaml:"name"`
	Required bool     `json:"required" yaml:"required"`
	Sources []string `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// EnvironmentResolution describes a secret-safe binding from an environment
// variable to a known topology target. It contains no resolved value.
type EnvironmentResolution struct {
	Variable   string           `json:"variable" yaml:"variable"`
	Source     string           `json:"source" yaml:"source"`
	Target     string           `json:"target" yaml:"target"`
	Kind       RelationshipKind `json:"kind" yaml:"kind"`
	Confidence float64          `json:"confidence" yaml:"confidence"`
	Evidence   []Evidence       `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type EnvironmentModel struct {
	Variables   []EnvironmentVariable   `json:"variables,omitempty" yaml:"variables,omitempty"`
	Resolutions []EnvironmentResolution `json:"resolutions,omitempty" yaml:"resolutions,omitempty"`
}

// ProjectModel is Octo's provider-neutral representation of a repository.
type ProjectModel struct {
	Name           string           `json:"name" yaml:"name"`
	Root           string           `json:"root" yaml:"root"`
	Language       string           `json:"language,omitempty" yaml:"language,omitempty"`
	RuntimeVersion string           `json:"runtime_version,omitempty" yaml:"runtime_version,omitempty"`
	Framework      string           `json:"framework,omitempty" yaml:"framework,omitempty"`
	PackageManager string           `json:"package_manager,omitempty" yaml:"package_manager,omitempty"`
	RunCommand     string           `json:"run_command,omitempty" yaml:"run_command,omitempty"`
	SetupCommand   string           `json:"setup_command,omitempty" yaml:"setup_command,omitempty"`
	Monorepo       bool             `json:"monorepo" yaml:"monorepo"`
	Port           int              `json:"port,omitempty" yaml:"port,omitempty"`
	Confidence     float64          `json:"confidence" yaml:"confidence"`
	Components     []Component      `json:"components,omitempty" yaml:"components,omitempty"`
	Services       []Service        `json:"services,omitempty" yaml:"services,omitempty"`
	Environment    EnvironmentModel `json:"environment,omitempty" yaml:"environment,omitempty"`
	Evidence       []Evidence       `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}
