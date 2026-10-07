package intelligence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type DetectedProject struct {
	Name string
	Language string
	Version string
	RunCommand string
	Port int
	PackageManager string
	SetupCommand string
	SetupRequired bool
	IsMonorepo bool
	MonorepoRoot string
}

type ProjectDetector interface { Detect(path string) (DetectedProject, error) }

// SignalDefinition defines a project file marker and its detector function.
type SignalDefinition struct {
	File     string
	Language string
	Detect   func(root string) (DetectedProject, error)
}

// ProjectDetectorRegistry provides an extensible registry for project detectors and signals.
type ProjectDetectorRegistry struct {
	mu        sync.RWMutex
	detectors []ProjectDetector
	signals   []SignalDefinition
}

// NewProjectDetectorRegistry initializes a detector registry populated with built-in ecosystem signals.
func NewProjectDetectorRegistry() *ProjectDetectorRegistry {
	return &ProjectDetectorRegistry{
		signals: []SignalDefinition{
			{File: "package.json", Language: "Node", Detect: detectNodeProject},
			{File: "go.work", Language: "Go", Detect: detectGoWorkspaceProject},
			{File: "go.mod", Language: "Go", Detect: detectGoProject},
			{File: "pyproject.toml", Language: "Python", Detect: detectPythonProject},
			{File: "requirements.txt", Language: "Python", Detect: detectPythonProject},
			{File: "Cargo.toml", Language: "Rust", Detect: detectRustProject},
			{File: "pom.xml", Language: "Java", Detect: detectJavaProject},
			{File: "build.gradle", Language: "Java", Detect: detectJavaProject},
			{File: "Gemfile", Language: "Ruby", Detect: detectRubyProject},
			{File: "composer.json", Language: "PHP", Detect: detectPHPProject},
			{File: "mix.exs", Language: "Elixir", Detect: detectElixirProject},
		},
	}
}

var defaultDetectorRegistry = NewProjectDetectorRegistry()

// DefaultDetectorRegistry returns the shared global detector registry.
func DefaultDetectorRegistry() *ProjectDetectorRegistry {
	return defaultDetectorRegistry
}

// RegisterProjectDetector registers a custom ProjectDetector into the default registry.
func RegisterProjectDetector(d ProjectDetector) {
	defaultDetectorRegistry.RegisterDetector(d)
}

// RegisterProjectSignal registers a project signal definition into the default registry.
func RegisterProjectSignal(sig SignalDefinition) {
	defaultDetectorRegistry.RegisterSignal(sig)
}

// RegisterDetector appends a detector to the registry.
func (r *ProjectDetectorRegistry) RegisterDetector(d ProjectDetector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detectors = append(r.detectors, d)
}

// RegisterSignal appends a signal definition to the registry.
func (r *ProjectDetectorRegistry) RegisterSignal(sig SignalDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.signals = append(r.signals, sig)
}

func (r *ProjectDetectorRegistry) Detect(path string) (DetectedProject, error) {
	root, err := filepath.Abs(path)
	if err != nil { return DetectedProject{}, err }
	info, err := os.Stat(root)
	if err != nil { return DetectedProject{}, err }
	if !info.IsDir() { return DetectedProject{}, errors.New("project path is not a directory") }

	r.mu.RLock()
	customDetectors := make([]ProjectDetector, len(r.detectors))
	copy(customDetectors, r.detectors)
	signals := make([]SignalDefinition, len(r.signals))
	copy(signals, r.signals)
	r.mu.RUnlock()

	for _, d := range customDetectors {
		if proj, err := d.Detect(root); err == nil && proj.Language != "" {
			if proj.Name == "" { proj.Name = filepath.Base(root) }
			proj.IsMonorepo, proj.MonorepoRoot = detectMonorepo(root)
			return proj, nil
		}
	}

	for _, signal := range signals {
		if _, err := os.Stat(filepath.Join(root, signal.File)); err != nil { continue }
		if signal.File == "package.json" && isStubPackageJSON(root) && hasOtherProjectSignals(root) {
			continue
		}
		project, err := signal.Detect(root)
		if err != nil { return DetectedProject{}, err }
		if project.Name == "" { project.Name = filepath.Base(root) }
		project.Language = signal.Language
		project.IsMonorepo, project.MonorepoRoot = detectMonorepo(root)
		return project, nil
	}
	return detectSimpleProject(root)
}

type NativeProjectDetector struct{}

func (NativeProjectDetector) Detect(path string) (DetectedProject, error) {
	return defaultDetectorRegistry.Detect(path)
}

func isStubPackageJSON(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil { return false }
	var pkg struct {
		Name       string            `json:"name"`
		Main       string            `json:"main"`
		Scripts    map[string]string `json:"scripts"`
		Workspaces interface{}       `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil { return false }
	return pkg.Name == "" && pkg.Main == "" && len(pkg.Scripts) == 0 && pkg.Workspaces == nil
}

func hasOtherProjectSignals(root string) bool {
	defaultDetectorRegistry.mu.RLock()
	signals := defaultDetectorRegistry.signals
	defaultDetectorRegistry.mu.RUnlock()

	for _, sig := range signals {
		if sig.File == "package.json" { continue }
		if _, err := os.Stat(filepath.Join(root, sig.File)); err == nil {
			return true
		}
	}
	return false
}

func detectPHPProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "composer.json"))
	if err != nil {
		return DetectedProject{}, err
	}
	var comp struct {
		Name    string            `json:"name"`
		Scripts map[string]string `json:"scripts"`
		Require map[string]string `json:"require"`
	}
	_ = json.Unmarshal(data, &comp)

	name := filepath.Base(root)
	if comp.Name != "" {
		parts := strings.Split(comp.Name, "/")
		if len(parts) > 1 {
			name = parts[1]
		} else {
			name = comp.Name
		}
	}

	phpVer := ""
	if v, ok := comp.Require["php"]; ok {
		phpVer = v
	}

	runCmd := "php -S 127.0.0.1:8000"
	port := 8000
	if _, ok := comp.Scripts["dev"]; ok {
		runCmd = "composer run dev"
	} else if _, ok := comp.Scripts["start"]; ok {
		runCmd = "composer start"
	} else if _, ok := comp.Scripts["serve"]; ok {
		runCmd = "composer serve"
	} else if _, err := os.Stat(filepath.Join(root, "artisan")); err == nil {
		runCmd = "php artisan serve"
	}

	return DetectedProject{
		Name:           name,
		Language:       "PHP",
		Version:        phpVer,
		RunCommand:     runCmd,
		Port:           port,
		PackageManager: "composer",
		SetupCommand:   "composer install",
		SetupRequired:  true,
	}, nil
}

func detectElixirProject(root string) (DetectedProject, error) {
	name := filepath.Base(root)
	mixPath := filepath.Join(root, "mix.exs")
	data, _ := os.ReadFile(mixPath)
	content := string(data)

	runCmd := "mix run --no-halt"
	port := 4000
	if strings.Contains(content, ":phoenix") || strings.Contains(content, "phoenix") {
		runCmd = "mix phx.server"
	}

	return DetectedProject{
		Name:           name,
		Language:       "Elixir",
		RunCommand:     runCmd,
		Port:           port,
		PackageManager: "mix",
		SetupCommand:   "mix deps.get",
		SetupRequired:  true,
	}, nil
}

func detectNodeProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil { return DetectedProject{}, err }
	var pkg struct {
		Name    string            `json:"name"`
		Version string            `json:"version"`
		Engines map[string]string `json:"engines"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil { return DetectedProject{}, err }
	version := pkg.Version
	if nodeVer := detectNodeRuntimeVersion(root, pkg.Engines); nodeVer != "" {
		version = nodeVer
	}
	return DetectedProject{Name: firstNonEmpty(pkg.Name, filepath.Base(root)), Version: version, PackageManager: detectNodePackageManager(root)}, nil
}

func detectNodeRuntimeVersion(root string, engines map[string]string) string {
	if engines != nil {
		if nodeVer, ok := engines["node"]; ok && strings.TrimSpace(nodeVer) != "" {
			return strings.TrimSpace(nodeVer)
		}
	}
	for _, file := range []string{".nvmrc", ".node-version"} {
		if data, err := os.ReadFile(filepath.Join(root, file)); err == nil {
			trimmed := strings.TrimSpace(string(data))
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func detectGoWorkspaceProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		return DetectedProject{}, err
	}
	project := DetectedProject{Name: filepath.Base(root), PackageManager: "go"}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			project.Version = fields[1]
			break
		}
	}
	return project, nil
}

func detectGoProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil { return DetectedProject{}, err }
	project := DetectedProject{Name: filepath.Base(root)}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" { project.Version = fields[1] }
		if len(fields) >= 2 && fields[0] == "module" {
			parts := strings.Split(fields[1], "/")
			if len(parts) > 0 && parts[len(parts)-1] != "" { project.Name = parts[len(parts)-1] }
		}
	}
	return project, nil
}

func detectPythonProject(root string) (DetectedProject, error) {
	project := DetectedProject{Name: filepath.Base(root)}
	if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
		inProjectSection := false
		inDepsSection := false
		pkgVersion := ""
		pythonVersion := ""
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				sec := strings.Trim(trimmed, "[]")
				inProjectSection = sec == "project" || sec == "tool.poetry"
				inDepsSection = sec == "tool.poetry.dependencies" || sec == "project.dependencies"
				continue
			}
			if inProjectSection {
				if strings.HasPrefix(trimmed, "name") {
					if fields := strings.SplitN(trimmed, "=", 2); len(fields) == 2 {
						project.Name = strings.Trim(strings.TrimSpace(fields[1]), "\"'")
					}
				}
				if strings.HasPrefix(trimmed, "version") {
					if fields := strings.SplitN(trimmed, "=", 2); len(fields) == 2 {
						pkgVersion = strings.Trim(strings.TrimSpace(fields[1]), "\"'")
					}
				}
				if strings.HasPrefix(trimmed, "requires-python") {
					if fields := strings.SplitN(trimmed, "=", 2); len(fields) == 2 {
						pythonVersion = strings.Trim(strings.TrimSpace(fields[1]), "\"'")
					}
				}
			}
			if inDepsSection {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == "python" {
					pythonVersion = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				}
			}
		}
		if pythonVersion != "" {
			project.Version = pythonVersion
		} else if pkgVersion != "" {
			project.Version = pkgVersion
		}
	}
	if pythonVer := detectPythonRuntimeVersion(root); pythonVer != "" {
		project.Version = pythonVer
	}
	project.PackageManager = detectPythonPackageManager(root)
	return project, nil
}

func detectPythonRuntimeVersion(root string) string {
	for _, f := range []string{".python-version", ".runtime"} {
		if data, err := os.ReadFile(filepath.Join(root, f)); err == nil {
			trimmed := strings.TrimSpace(string(data))
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func detectPythonPackageManager(root string) string {
	for _, candidate := range []struct{ file, manager string }{
		{"uv.lock", "uv"},
		{"poetry.lock", "poetry"},
		{"Pipfile.lock", "pipenv"},
		{"Pipfile", "pipenv"},
		{"requirements.txt", "pip"},
	} {
		if _, err := os.Stat(filepath.Join(root, candidate.file)); err == nil {
			return candidate.manager
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		return "pip"
	}
	return ""
}
func detectJavaProject(root string) (DetectedProject, error) { return DetectedProject{Name: filepath.Base(root)}, nil }
func detectRubyProject(root string) (DetectedProject, error) { return DetectedProject{Name: filepath.Base(root)}, nil }

func detectRustProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil { return DetectedProject{}, err }
	project := DetectedProject{Name: filepath.Base(root), PackageManager: "cargo"}
	inPackage := false
	pkgVersion := ""
	rustVersion := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inPackage = trimmed == "[package]"
			continue
		}
		if !inPackage {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch key {
		case "name":
			project.Name = val
		case "version":
			pkgVersion = val
		case "rust-version":
			rustVersion = val
		}
	}
	if rustVer := detectRustRuntimeVersion(root); rustVer != "" {
		project.Version = rustVer
	} else if rustVersion != "" {
		project.Version = rustVersion
	} else if pkgVersion != "" {
		project.Version = pkgVersion
	}
	return project, nil
}

func detectRustRuntimeVersion(root string) string {
	for _, f := range []string{".rust-version", "rust-toolchain"} {
		if data, err := os.ReadFile(filepath.Join(root, f)); err == nil {
			trimmed := strings.TrimSpace(string(data))
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func detectSimpleProject(root string) (DetectedProject, error) {
	entries, err := os.ReadDir(root)
	if err != nil { return DetectedProject{}, err }
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".html") || strings.HasSuffix(entry.Name(), ".htm")) {
			project := DetectedProject{Name: filepath.Base(root), Language: "HTML"}
			project.IsMonorepo, project.MonorepoRoot = detectMonorepo(root)
			return project, nil
		}
	}
	project := DetectedProject{Name: filepath.Base(root), Language: "Unknown"}
	project.IsMonorepo, project.MonorepoRoot = detectMonorepo(root)
	return project, nil
}

func detectNodePackageManager(root string) string {
	for _, candidate := range []struct{ file, manager string }{
		{"bun.lockb", "bun"}, {"bun.lock", "bun"}, {"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"},
	} {
		if _, err := os.Stat(filepath.Join(root, candidate.file)); err == nil { return candidate.manager }
	}
	return "npm"
}

func detectMonorepo(root string) (bool, string) {
	for _, file := range []string{"pnpm-workspace.yaml", "nx.json", "turbo.json", "lerna.json", "rush.json", "go.work"} {
		if _, err := os.Stat(filepath.Join(root, file)); err == nil { return true, root }
	}
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err == nil {
		var pkg struct { Workspaces any `json:"workspaces"` }
		if json.Unmarshal(data, &pkg) == nil && pkg.Workspaces != nil { return true, root }
	}
	if cargoData, err := os.ReadFile(filepath.Join(root, "Cargo.toml")); err == nil {
		if strings.Contains(string(cargoData), "[workspace]") {
			return true, root
		}
	}
	return false, ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values { if value != "" { return value } }
	return ""
}

func detectProject(path string) (DetectedProject, error) { return NativeProjectDetector{}.Detect(path) }
