package intelligence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type workspacePackage struct {
	name         string
	path         string
	info DetectedProject
	dependencies []string
}

func discoverWorkspaceComponents(root string, rootInfo DetectedProject) ([]Component, []Evidence, bool, error) {
	if rootInfo.Language != "Node" || !rootInfo.IsMonorepo {
		return nil, nil, false, nil
	}

	paths, marker, err := nodeWorkspacePaths(root)
	if err != nil {
		return nil, nil, false, err
	}
	if len(paths) == 0 {
		return nil, nil, false, nil
	}

	packages := make([]workspacePackage, 0, len(paths))
	for _, path := range paths {
		info, err := detectProject(path)
		if err != nil {
			return nil, nil, false, fmt.Errorf("analyze workspace component %q: %w", path, err)
		}
		name, deps, err := nodePackageMetadata(path)
		if err != nil {
			return nil, nil, false, err
		}
		if name == "" {
			name = filepath.Base(path)
		}
		packages = append(packages, workspacePackage{
			name: name, path: relativePath(root, path), info: info, dependencies: deps,
		})
	}

	names := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		names[pkg.name] = struct{}{}
	}

	components := make([]Component, 0, len(packages))
	evidence := []Evidence{{
		Kind: EvidenceConfig,
		Path: marker,
		Detail: fmt.Sprintf("Discovered %d workspace components from the declared workspace configuration.", len(packages)),
		Strength: 0.98,
	}}

	for _, pkg := range packages {
		componentRoot := filepath.Join(root, filepath.FromSlash(pkg.path))
		framework := detectFramework(componentRoot, pkg.info.Language)
		componentEvidence := []Evidence{{
			Kind: EvidenceSignalFile,
			Path: filepath.ToSlash(filepath.Join(pkg.path, signalPath(pkg.info.Language))),
			Detail: "Workspace component contains a project signal file.",
			Strength: 0.90,
		}}

		if f := lockfile(componentRoot, pkg.info.PackageManager); f != "" {
			componentEvidence = append(componentEvidence, Evidence{
				Kind: EvidenceLockfile,
				Path: filepath.ToSlash(filepath.Join(pkg.path, f)),
				Detail: "Workspace component lockfile supports its package manager.",
				Strength: 0.95,
			})
		}
		if pkg.info.RunCommand != "" {
			componentEvidence = append(componentEvidence, Evidence{
				Kind: EvidenceScript,
				Path: filepath.ToSlash(filepath.Join(pkg.path, "package.json")),
				Detail: "Selected run command: " + pkg.info.RunCommand,
				Strength: 0.80,
			})
		}

		dependsOn := make([]string, 0)
		for _, dep := range pkg.dependencies {
			if _, ok := names[dep]; ok && dep != pkg.name {
				dependsOn = append(dependsOn, dep)
			}
		}
		sort.Strings(dependsOn)

		conf := confidence(componentEvidence, pkg.info.RunCommand != "")
		components = append(components, Component{
			Name: pkg.name, Path: pkg.path,
			Language: pkg.info.Language, Framework: framework,
			PackageManager: pkg.info.PackageManager,
			RunCommand: pkg.info.RunCommand, DependsOn: dependsOn,
			Port: pkg.info.Port, Confidence: conf,
			Evidence: componentEvidence,
		})
	}

	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return components, evidence, true, nil
}

func nodeWorkspacePaths(root string) ([]string, string, error) {
	packagePath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(packagePath); err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(data, &pkg); err == nil {
			if raw, ok := pkg["workspaces"]; ok {
				patterns := workspacePatterns(raw)
				paths, err := expandWorkspacePatterns(root, patterns)
				return paths, "package.json", err
			}
		}
	}

	pnpmPath := filepath.Join(root, "pnpm-workspace.yaml")
	if data, err := os.ReadFile(pnpmPath); err == nil {
		var config map[string]interface{}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, "", fmt.Errorf("parse pnpm-workspace.yaml: %w", err)
		}
		patterns := workspacePatterns(config["packages"])
		paths, err := expandWorkspacePatterns(root, patterns)
		return paths, "pnpm-workspace.yaml", err
	}

	return nil, "", nil
}

func workspacePatterns(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		patterns := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				patterns = append(patterns, s)
			}
		}
		return patterns
	case []string:
		return v
	default:
		return nil
	}
}

func expandWorkspacePatterns(root string, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	paths := make([]string, 0)

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, fmt.Errorf("expand workspace pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(match, "package.json")); err != nil {
				continue
			}
			abs, err := filepath.Abs(match)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[abs]; ok {
				continue
			}
			seen[abs] = struct{}{}
			paths = append(paths, abs)
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func nodePackageMetadata(path string) (string, []string, error) {
	data, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return "", nil, fmt.Errorf("read %s/package.json: %w", path, err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", nil, fmt.Errorf("parse %s/package.json: %w", path, err)
	}

	name, _ := pkg["name"].(string)
	deps := make([]string, 0)
	for _, field := range []string{"dependencies", "devDependencies", "optionalDependencies"} {
		values, _ := pkg[field].(map[string]interface{})
		for dep, rawVersion := range values {
			version, _ := rawVersion.(string)
			if strings.HasPrefix(version, "workspace:") {
				deps = append(deps, dep)
			}
		}
	}
	sort.Strings(deps)
	return name, deps, nil
}

func signalPath(language string) string {
	switch language {
	case "Node":
		return "package.json"
	case "Java":
		return "pom.xml"
	case "Python":
		return "pyproject.toml"
	case "Go":
		return "go.mod"
	case "Rust":
		return "Cargo.toml"
	case "Ruby":
		return "Gemfile"
	default:
		return ""
	}
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
