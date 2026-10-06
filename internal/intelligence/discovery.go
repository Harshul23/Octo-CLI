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
	info         DetectedProject
	dependencies []string
}

func discoverWorkspaceComponents(root string, rootInfo DetectedProject) ([]Component, []Evidence, bool, error) {
	if !rootInfo.IsMonorepo {
		return nil, nil, false, nil
	}

	if rootInfo.Language == "Node" {
		return discoverNodeWorkspaceComponents(root, rootInfo)
	}
	if rootInfo.Language == "Go" || hasGoWork(root) {
		return discoverGoWorkspaceComponents(root, rootInfo)
	}
	if rootInfo.Language == "Rust" || hasCargoWorkspace(root) {
		return discoverCargoWorkspaceComponents(root, rootInfo)
	}

	return nil, nil, false, nil
}

func discoverNodeWorkspaceComponents(root string, rootInfo DetectedProject) ([]Component, []Evidence, bool, error) {
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
		Kind:     EvidenceConfig,
		Path:     marker,
		Detail:   fmt.Sprintf("Discovered %d workspace components from the declared workspace configuration.", len(packages)),
		Strength: 0.98,
	}}

	for _, pkg := range packages {
		componentRoot := filepath.Join(root, filepath.FromSlash(pkg.path))
		framework := detectFramework(componentRoot, pkg.info.Language)
		componentEvidence := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(pkg.path, signalPath(pkg.info.Language))),
			Detail:   "Workspace component contains a project signal file.",
			Strength: 0.90,
		}}

		if f := lockfile(componentRoot, pkg.info.PackageManager); f != "" {
			componentEvidence = append(componentEvidence, Evidence{
				Kind:     EvidenceLockfile,
				Path:     filepath.ToSlash(filepath.Join(pkg.path, f)),
				Detail:   "Workspace component lockfile supports its package manager.",
				Strength: 0.95,
			})
		}
		if pkg.info.RunCommand != "" {
			componentEvidence = append(componentEvidence, Evidence{
				Kind:     EvidenceScript,
				Path:     filepath.ToSlash(filepath.Join(pkg.path, "package.json")),
				Detail:   "Selected run command: " + pkg.info.RunCommand,
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
		candidates := make([]ExecutionCandidate, 0)
		if pkg.info.RunCommand != "" {
			candidates = append(candidates, ExecutionCandidate{
				ID:         fmt.Sprintf("%s.script.run", strings.ToLower(pkg.info.Language)),
				Command:    pkg.info.RunCommand,
				Confidence: 0.90,
				Evidence:   componentEvidence,
			})
		}

		components = append(components, Component{
			Name:                pkg.name,
			Path:                pkg.path,
			Language:            pkg.info.Language,
			Framework:           framework,
			PackageManager:      pkg.info.PackageManager,
			RunCommand:          pkg.info.RunCommand,
			DependsOn:           dependsOn,
			Port:                pkg.info.Port,
			Confidence:          conf,
			Evidence:            componentEvidence,
			ExecutionCandidates: candidates,
		})
	}

	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return components, evidence, true, nil
}

func hasGoWork(root string) bool {
	_, err := os.Stat(filepath.Join(root, "go.work"))
	return err == nil
}

func discoverGoWorkspaceComponents(root string, rootInfo DetectedProject) ([]Component, []Evidence, bool, error) {
	workPath := filepath.Join(root, "go.work")
	data, err := os.ReadFile(workPath)
	if err != nil {
		return nil, nil, false, nil
	}

	paths, err := goWorkspacePaths(root, string(data))
	if err != nil || len(paths) == 0 {
		return nil, nil, false, err
	}

	packages := make([]workspacePackage, 0, len(paths))
	for _, path := range paths {
		info, err := detectProject(path)
		if err != nil {
			return nil, nil, false, fmt.Errorf("analyze go workspace component %q: %w", path, err)
		}
		name, deps, err := goPackageMetadata(path)
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
		Kind:     EvidenceConfig,
		Path:     "go.work",
		Detail:   fmt.Sprintf("Discovered %d Go workspace components from go.work.", len(packages)),
		Strength: 0.98,
	}}

	for _, pkg := range packages {
		componentEvidence := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(pkg.path, "go.mod")),
			Detail:   "Workspace component contains a go.mod manifest.",
			Strength: 0.90,
		}}

		dependsOn := make([]string, 0)
		for _, dep := range pkg.dependencies {
			if _, ok := names[dep]; ok && dep != pkg.name {
				dependsOn = append(dependsOn, dep)
			}
		}
		sort.Strings(dependsOn)

		runCmd := pkg.info.RunCommand
		if runCmd == "" {
			if isGoExecutable(filepath.Join(root, filepath.FromSlash(pkg.path))) {
				runCmd = "go run ."
			}
		}

		candidates := make([]ExecutionCandidate, 0)
		if runCmd != "" {
			candidates = append(candidates, ExecutionCandidate{
				ID:         "go.package",
				Command:    runCmd,
				Confidence: 0.90,
				Evidence:   componentEvidence,
			})
		}

		conf := confidence(componentEvidence, runCmd != "")
		components = append(components, Component{
			Name:                pkg.name,
			Path:                pkg.path,
			Language:            "Go",
			PackageManager:      "go",
			RunCommand:          runCmd,
			DependsOn:           dependsOn,
			Port:                pkg.info.Port,
			Confidence:          conf,
			Evidence:            componentEvidence,
			ExecutionCandidates: candidates,
		})
	}

	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return components, evidence, true, nil
}

func goWorkspacePaths(root, content string) ([]string, error) {
	var paths []string
	inUseBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}
		if trimmed == "use (" {
			inUseBlock = true
			continue
		}
		if inUseBlock {
			if trimmed == ")" {
				inUseBlock = false
				continue
			}
			p := strings.Trim(trimmed, "\"'")
			abs := filepath.Join(root, filepath.FromSlash(p))
			if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
				paths = append(paths, abs)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "use ") {
			p := strings.TrimSpace(strings.TrimPrefix(trimmed, "use "))
			p = strings.Trim(p, "\"'")
			abs := filepath.Join(root, filepath.FromSlash(p))
			if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
				paths = append(paths, abs)
			}
		}
	}
	return paths, nil
}

func goPackageMetadata(path string) (string, []string, error) {
	data, err := os.ReadFile(filepath.Join(path, "go.mod"))
	if err != nil {
		return "", nil, err
	}
	name := filepath.Base(path)
	var deps []string

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			parts := strings.Split(fields[1], "/")
			if len(parts) > 0 && parts[len(parts)-1] != "" {
				name = parts[len(parts)-1]
			}
		}
		if len(fields) >= 2 && fields[0] == "require" {
			parts := strings.Split(fields[1], "/")
			if len(parts) > 0 {
				deps = append(deps, parts[len(parts)-1])
			}
		}
	}
	return name, deps, nil
}

func isGoExecutable(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err == nil {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err == nil && strings.HasPrefix(strings.TrimSpace(string(data)), "package main") {
				return true
			}
		}
	}
	return false
}

func hasCargoWorkspace(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "[workspace]")
}

func discoverCargoWorkspaceComponents(root string, rootInfo DetectedProject) ([]Component, []Evidence, bool, error) {
	cargoPath := filepath.Join(root, "Cargo.toml")
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, nil, false, nil
	}

	paths, err := cargoWorkspacePaths(root, string(data))
	if err != nil || len(paths) == 0 {
		return nil, nil, false, err
	}

	packages := make([]workspacePackage, 0, len(paths))
	for _, path := range paths {
		info, err := detectProject(path)
		if err != nil {
			return nil, nil, false, fmt.Errorf("analyze cargo workspace component %q: %w", path, err)
		}
		name, deps, runCmd, err := cargoPackageMetadata(path)
		if err != nil {
			return nil, nil, false, err
		}
		if name == "" {
			name = filepath.Base(path)
		}
		info.RunCommand = runCmd
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
		Kind:     EvidenceConfig,
		Path:     "Cargo.toml",
		Detail:   fmt.Sprintf("Discovered %d Cargo workspace members from Cargo.toml.", len(packages)),
		Strength: 0.98,
	}}

	for _, pkg := range packages {
		componentEvidence := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(pkg.path, "Cargo.toml")),
			Detail:   "Workspace member contains a Cargo.toml manifest.",
			Strength: 0.90,
		}}

		dependsOn := make([]string, 0)
		for _, dep := range pkg.dependencies {
			if _, ok := names[dep]; ok && dep != pkg.name {
				dependsOn = append(dependsOn, dep)
			}
		}
		sort.Strings(dependsOn)

		candidates := make([]ExecutionCandidate, 0)
		if pkg.info.RunCommand != "" {
			candidates = append(candidates, ExecutionCandidate{
				ID:         "rust.cargo.run",
				Command:    pkg.info.RunCommand,
				Confidence: 0.90,
				Evidence:   componentEvidence,
			})
		}

		conf := confidence(componentEvidence, pkg.info.RunCommand != "")
		components = append(components, Component{
			Name:                pkg.name,
			Path:                pkg.path,
			Language:            "Rust",
			PackageManager:      "cargo",
			RunCommand:          pkg.info.RunCommand,
			DependsOn:           dependsOn,
			Port:                pkg.info.Port,
			Confidence:          conf,
			Evidence:            componentEvidence,
			ExecutionCandidates: candidates,
		})
	}

	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	return components, evidence, true, nil
}

func cargoWorkspacePaths(root, content string) ([]string, error) {
	var patterns []string
	inWorkspace := false
	inMembers := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, "#"); idx != -1 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inMembers = false
			inWorkspace = (trimmed == "[workspace]")
			continue
		}
		if !inWorkspace {
			continue
		}
		if strings.HasPrefix(trimmed, "members") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if strings.HasPrefix(val, "[") {
					val = strings.TrimPrefix(val, "[")
					if strings.Contains(val, "]") {
						val = strings.Split(val, "]")[0]
						for _, item := range strings.Split(val, ",") {
							item = strings.TrimSpace(strings.Trim(strings.TrimSpace(item), "\"'"))
							if item != "" {
								patterns = append(patterns, item)
							}
						}
					} else {
						inMembers = true
						for _, item := range strings.Split(val, ",") {
							item = strings.TrimSpace(strings.Trim(strings.TrimSpace(item), "\"'"))
							if item != "" {
								patterns = append(patterns, item)
							}
						}
					}
				}
			}
			continue
		}
		if inMembers {
			if strings.Contains(trimmed, "]") {
				inMembers = false
				val := strings.Split(trimmed, "]")[0]
				for _, item := range strings.Split(val, ",") {
					item = strings.TrimSpace(strings.Trim(strings.TrimSpace(item), "\"'"))
					if item != "" {
						patterns = append(patterns, item)
					}
				}
				continue
			}
			for _, item := range strings.Split(trimmed, ",") {
				item = strings.TrimSpace(strings.Trim(strings.TrimSpace(item), "\"'"))
				if item != "" {
					patterns = append(patterns, item)
				}
			}
		}
	}

	seen := make(map[string]struct{})
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if _, err := os.Stat(filepath.Join(match, "Cargo.toml")); err == nil {
				abs, _ := filepath.Abs(match)
				if _, ok := seen[abs]; !ok {
					seen[abs] = struct{}{}
					paths = append(paths, abs)
				}
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func cargoPackageMetadata(path string) (string, []string, string, error) {
	data, err := os.ReadFile(filepath.Join(path, "Cargo.toml"))
	if err != nil {
		return "", nil, "", err
	}
	name := filepath.Base(path)
	var deps []string
	isBin := false
	if _, err := os.Stat(filepath.Join(path, "src", "main.rs")); err == nil {
		isBin = true
	}
	if _, err := os.Stat(filepath.Join(path, "main.rs")); err == nil {
		isBin = true
	}

	lines := strings.Split(string(data), "\n")
	inDeps := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[[bin]]") {
			isBin = true
		}
		if strings.HasPrefix(trimmed, "[package]") {
			inDeps = false
			continue
		}
		if strings.HasPrefix(trimmed, "[dependencies") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inDeps = false
			continue
		}
		if !inDeps && strings.HasPrefix(trimmed, "name") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				name = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
		if inDeps {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) >= 1 && parts[0] != "" {
				depName := strings.TrimSpace(parts[0])
				deps = append(deps, depName)
			}
		}
	}

	runCmd := ""
	if isBin {
		runCmd = fmt.Sprintf("cargo run -p %s", name)
	}
	return name, deps, runCmd, nil
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
