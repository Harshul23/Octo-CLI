package intelligence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

type signalDefinition struct {
	File string
	Language string
	Detect func(root string) (DetectedProject, error)
}

type NativeProjectDetector struct{}

var projectSignals = []signalDefinition{
	{File: "package.json", Language: "Node", Detect: detectNodeProject},
	{File: "go.mod", Language: "Go", Detect: detectGoProject},
	{File: "pyproject.toml", Language: "Python", Detect: detectPythonProject},
	{File: "requirements.txt", Language: "Python", Detect: detectPythonProject},
	{File: "Cargo.toml", Language: "Rust", Detect: detectRustProject},
	{File: "pom.xml", Language: "Java", Detect: detectJavaProject},
	{File: "build.gradle", Language: "Java", Detect: detectJavaProject},
	{File: "Gemfile", Language: "Ruby", Detect: detectRubyProject},
}

func (NativeProjectDetector) Detect(path string) (DetectedProject, error) {
	root, err := filepath.Abs(path)
	if err != nil { return DetectedProject{}, err }
	info, err := os.Stat(root)
	if err != nil { return DetectedProject{}, err }
	if !info.IsDir() { return DetectedProject{}, errors.New("project path is not a directory") }

	for _, signal := range projectSignals {
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
	for _, sig := range projectSignals {
		if sig.File == "package.json" { continue }
		if _, err := os.Stat(filepath.Join(root, sig.File)); err == nil {
			return true
		}
	}
	return false
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
			return DetectedProject{Name: filepath.Base(root), Language: "HTML"}, nil
		}
	}
	return DetectedProject{Name: filepath.Base(root), Language: "Unknown"}, nil
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
	for _, file := range []string{"pnpm-workspace.yaml", "nx.json", "turbo.json", "lerna.json", "rush.json"} {
		if _, err := os.Stat(filepath.Join(root, file)); err == nil { return true, root }
	}
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err == nil {
		var pkg struct { Workspaces any `json:"workspaces"` }
		if json.Unmarshal(data, &pkg) == nil && pkg.Workspaces != nil { return true, root }
	}
	return false, ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values { if value != "" { return value } }
	return ""
}

func detectProject(path string) (DetectedProject, error) { return NativeProjectDetector{}.Detect(path) }
