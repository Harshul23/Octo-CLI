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
		project, err := signal.Detect(root)
		if err != nil { return DetectedProject{}, err }
		if project.Name == "" { project.Name = filepath.Base(root) }
		project.Language = signal.Language
		project.IsMonorepo, project.MonorepoRoot = detectMonorepo(root)
		return project, nil
	}
	return detectSimpleProject(root)
}

func detectNodeProject(root string) (DetectedProject, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil { return DetectedProject{}, err }
	var pkg struct {
		Name string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil { return DetectedProject{}, err }
	return DetectedProject{Name: firstNonEmpty(pkg.Name, filepath.Base(root)), Version: pkg.Version, PackageManager: detectNodePackageManager(root)}, nil
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
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "name") {
				if fields := strings.SplitN(trimmed, "=", 2); len(fields) == 2 {
					project.Name = strings.Trim(strings.TrimSpace(fields[1]), "\"'")
				}
			}
			if strings.HasPrefix(trimmed, "version") {
				if fields := strings.SplitN(trimmed, "=", 2); len(fields) == 2 {
					project.Version = strings.Trim(strings.TrimSpace(fields[1]), "\"'")
				}
			}
		}
	}
	project.PackageManager = detectPythonPackageManager(root)
	return project, nil
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
	project := DetectedProject{Name: filepath.Base(root)}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "name" && fields[1] == "=" { project.Name = strings.Trim(fields[2], "\""+"'") }
		if len(fields) >= 3 && fields[0] == "version" && fields[1] == "=" { project.Version = strings.Trim(fields[2], "\""+"'") }
	}
	return project, nil
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
