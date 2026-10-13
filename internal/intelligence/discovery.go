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
	comps, ev, _, found, err := discoverAllComponents(root, rootInfo, nil)
	return comps, ev, found, err
}

func discoverAllComponents(root string, rootInfo DetectedProject, services []Service) ([]Component, []Evidence, []string, bool, error) {
	var allComponents []Component
	var allEvidence []Evidence
	var allDiagnostics []string
	declaredPaths := make(map[string]struct{})

	// 1. Preserve declared Node, go.work and Cargo workspace behavior
	if hasNodeWorkspace(root) {
		comps, ev, found, err := discoverNodeWorkspaceComponents(root, rootInfo)
		if err == nil && found {
			allComponents = append(allComponents, comps...)
			allEvidence = append(allEvidence, ev...)
			for _, c := range comps {
				declaredPaths[filepath.Clean(c.Path)] = struct{}{}
			}
		}
	}
	if hasGoWork(root) {
		comps, ev, found, err := discoverGoWorkspaceComponents(root, rootInfo)
		if err == nil && found {
			allComponents = append(allComponents, comps...)
			allEvidence = append(allEvidence, ev...)
			for _, c := range comps {
				declaredPaths[filepath.Clean(c.Path)] = struct{}{}
			}
		}
	}
	if hasCargoWorkspace(root) {
		comps, ev, found, err := discoverCargoWorkspaceComponents(root, rootInfo)
		if err == nil && found {
			allComponents = append(allComponents, comps...)
			allEvidence = append(allEvidence, ev...)
			for _, c := range comps {
				declaredPaths[filepath.Clean(c.Path)] = struct{}{}
			}
		}
	}

	// 2. Deterministic fallback directory scan for recognized project manifests and executable entry points
	scannedComps, scannedEv, scanDiag, err := scanFallbackComponents(root, declaredPaths)
	if err != nil {
		return nil, nil, nil, false, err
	}
	allDiagnostics = append(allDiagnostics, scanDiag...)
	allComponents = append(allComponents, scannedComps...)
	allEvidence = append(allEvidence, scannedEv...)

	// Deduplicate overlapping declared/scanned paths
	deduped := make([]Component, 0, len(allComponents))
	seenPaths := make(map[string]struct{})
	for _, c := range allComponents {
		clean := filepath.Clean(c.Path)
		if _, seen := seenPaths[clean]; seen {
			continue
		}
		seenPaths[clean] = struct{}{}
		deduped = append(deduped, c)
	}

	// 3. Handle root component: keep a real runnable root app, but do not offer root workspace orchestration scripts
	if len(deduped) == 0 {
		rootComp := Component{
			Name:           rootInfo.Name,
			Path:           ".",
			Language:       rootInfo.Language,
			Framework:      detectFramework(root, rootInfo.Language),
			PackageManager: rootInfo.PackageManager,
			RunCommand:     rootInfo.RunCommand,
			Port:           rootInfo.Port,
		}
		return []Component{rootComp}, allEvidence, allDiagnostics, false, nil
	}

	if isRealRunnableRootApp(root, rootInfo, deduped) {
		rootComp := Component{
			Name:           rootInfo.Name,
			Path:           ".",
			Language:       rootInfo.Language,
			Framework:      detectFramework(root, rootInfo.Language),
			PackageManager: rootInfo.PackageManager,
			RunCommand:     rootInfo.RunCommand,
			Port:           rootInfo.Port,
		}
		deduped = append([]Component{rootComp}, deduped...)
	}

	// 4. Disambiguate duplicate names and ensure Component and Compose-service identities remain distinct
	deduped = disambiguateComponentNames(deduped, services)

	// 5. Deterministic sorting by repository-relative path
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Path < deduped[j].Path
	})

	isMonorepo := len(deduped) > 1 || (len(deduped) == 1 && deduped[0].Path != ".")
	return deduped, allEvidence, allDiagnostics, isMonorepo, nil
}

func hasNodeWorkspace(root string) bool {
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Workspaces interface{} `json:"workspaces"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Workspaces != nil {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pnpm-workspace.yaml")); err == nil {
		return true
	}
	return false
}

var (
	maxScanDepth = 6
	maxScanDirs  = 10000
)

func scanFallbackComponents(root string, declaredPaths map[string]struct{}) ([]Component, []Evidence, []string, error) {
	var scanned []Component
	var evidence []Evidence
	var diagnostics []string
	dirCount := 0
	seenPaths := make(map[string]struct{})
	rootHasManifest := hasRootProjectManifest(root)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if rel == "." {
			return nil
		}

		// Prune symlinks pointing outside the requested root
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
			if !isPathWithinRoot(root, target) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Do not follow directory symlinks inside root to avoid duplicate scans or symlink loops
			if fileInfo, err := os.Stat(target); err == nil && fileInfo.IsDir() {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if entry.IsDir() {
			name := entry.Name()
			if isPrunedDir(name, path) {
				return filepath.SkipDir
			}

			// Prune nested unrelated repositories
			if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
				return filepath.SkipDir
			}

			// Detect and report nested Compose layouts as not yet supported
			if hasComposeFile(path) {
				relCompose := filepath.ToSlash(filepath.Join(rel, findComposeFile(path)))
				diagnostics = append(diagnostics, fmt.Sprintf("nested Compose layout %q is not yet supported", relCompose))
			}

			// Resource bounds
			depth := strings.Count(rel, "/") + 1
			if depth > maxScanDepth {
				diagnostics = append(diagnostics, fmt.Sprintf("directory scan depth bound (%d) reached at %q", maxScanDepth, rel))
				return filepath.SkipDir
			}
			dirCount++
			if dirCount > maxScanDirs {
				diagnostics = append(diagnostics, fmt.Sprintf("directory scan limit (%d directories) reached", maxScanDirs))
				return filepath.SkipAll
			}

			if _, ok := declaredPaths[rel]; ok {
				return nil
			}
			if _, ok := seenPaths[rel]; ok {
				return nil
			}

			comp, ev, recognized, err := recognizeComponent(root, rel, path, rootHasManifest)
			if err != nil {
				return nil
			}
			if recognized {
				seenPaths[rel] = struct{}{}
				scanned = append(scanned, comp)
				if len(ev) > 0 {
					evidence = append(evidence, ev...)
				}
			}
		}

		return nil
	})

	names := make(map[string]struct{}, len(scanned))
	for _, c := range scanned {
		names[c.Name] = struct{}{}
	}
	for i := range scanned {
		var filtered []string
		for _, dep := range scanned[i].DependsOn {
			if _, ok := names[dep]; ok && dep != scanned[i].Name {
				filtered = append(filtered, dep)
			}
		}
		sort.Strings(filtered)
		scanned[i].DependsOn = filtered
	}

	return scanned, evidence, diagnostics, err
}

func hasRootProjectManifest(root string) bool {
	for _, f := range []string{
		"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"pom.xml", "build.gradle", "build.gradle.kts", "composer.json", "mix.exs",
		"Gemfile", "Package.swift", ".octo.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			if f == "package.json" && isStubPackageJSON(root) {
				continue
			}
			return true
		}
	}
	return false
}

func isSourceOrAuxiliaryDir(name string) bool {
	switch strings.ToLower(name) {
	case "src", "app", "cmd", "pkg", "internal", "lib", "docs", "examples", "scripts", "tools", "hack", "tests", "test", "e2e", "bin":
		return true
	default:
		return false
	}
}

func isPathWithinRoot(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		absTarget = target
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

func hasComposeFile(dir string) bool {
	return findComposeFile(dir) != ""
}

func isPrunedDir(name, fullPath string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "fixtures", "testdata":
		return true
	case ".venv", "venv", "env", ".env", "virtualenv":
		return true
	case ".cache", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache",
		".turbo", ".next", ".nuxt", ".svelte-kit", ".parcel-cache",
		"dist", "build", "out", "target", ".gradle", "bin", "obj",
		".idea", ".vscode", ".ds_store":
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	if _, err := os.Stat(filepath.Join(fullPath, "pyvenv.cfg")); err == nil {
		return true
	}
	return false
}

func recognizeComponent(root, rel, dir string, rootHasManifest bool) (Component, []Evidence, bool, error) {
	// 1. Node: package.json (non-stub)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		if !isStubPackageJSON(dir) {
			name, deps, _ := nodePackageMetadata(dir)
			if name == "" {
				name = filepath.Base(dir)
			}
			pm := detectNodePackageManager(dir)
			framework := detectFramework(dir, "Node")
			ev := []Evidence{{
				Kind:     EvidenceSignalFile,
				Path:     filepath.ToSlash(filepath.Join(rel, "package.json")),
				Detail:   "Component contains a package.json manifest.",
				Strength: 0.90,
			}}
			if f := lockfile(dir, pm); f != "" {
				ev = append(ev, Evidence{
					Kind:     EvidenceLockfile,
					Path:     filepath.ToSlash(filepath.Join(rel, f)),
					Detail:   "Component lockfile supports its package manager.",
					Strength: 0.95,
				})
			}
			return Component{
				Name:           name,
				Path:           rel,
				Language:       "Node",
				Framework:      framework,
				PackageManager: pm,
				DependsOn:      deps,
				Evidence:       ev,
			}, ev, true, nil
		}
	}

	// 2. Go: go.mod
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		name, deps, _ := goPackageMetadata(dir)
		if name == "" {
			name = filepath.Base(dir)
		}
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "go.mod")),
			Detail:   "Component contains a go.mod manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           name,
			Path:           rel,
			Language:       "Go",
			PackageManager: "go",
			DependsOn:      deps,
			Evidence:       ev,
		}, ev, true, nil
	}

	// 3. Rust: Cargo.toml
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		name, deps, runCmd, _ := cargoPackageMetadata(dir)
		if name == "" {
			name = filepath.Base(dir)
		}
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "Cargo.toml")),
			Detail:   "Component contains a Cargo.toml manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           name,
			Path:           rel,
			Language:       "Rust",
			PackageManager: "cargo",
			RunCommand:     runCmd,
			DependsOn:      deps,
			Evidence:       ev,
		}, ev, true, nil
	}

	// 4. Python: pyproject.toml, requirements.txt, Pipfile
	for _, f := range []string{"pyproject.toml", "requirements.txt", "Pipfile"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			info, _ := detectPythonProject(dir)
			name := info.Name
			if name == "" {
				name = filepath.Base(dir)
			}
			framework := detectFramework(dir, "Python")
			ev := []Evidence{{
				Kind:     EvidenceSignalFile,
				Path:     filepath.ToSlash(filepath.Join(rel, f)),
				Detail:   fmt.Sprintf("Component contains a Python %s manifest.", f),
				Strength: 0.90,
			}}
			return Component{
				Name:           name,
				Path:           rel,
				Language:       "Python",
				Framework:      framework,
				PackageManager: info.PackageManager,
				Evidence:       ev,
			}, ev, true, nil
		}
	}

	// 5. Java / Kotlin: pom.xml, build.gradle, build.gradle.kts
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			lang := "Java"
			pm := "maven"
			if strings.HasSuffix(f, ".gradle.kts") {
				lang = "Kotlin"
				pm = "gradle"
			} else if strings.HasSuffix(f, ".gradle") {
				pm = "gradle"
			}
			ev := []Evidence{{
				Kind:     EvidenceSignalFile,
				Path:     filepath.ToSlash(filepath.Join(rel, f)),
				Detail:   fmt.Sprintf("Component contains %s.", f),
				Strength: 0.90,
			}}
			return Component{
				Name:           filepath.Base(dir),
				Path:           rel,
				Language:       lang,
				PackageManager: pm,
				Evidence:       ev,
			}, ev, true, nil
		}
	}

	// 6. PHP: composer.json
	if _, err := os.Stat(filepath.Join(dir, "composer.json")); err == nil {
		info, _ := detectPHPProject(dir)
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "composer.json")),
			Detail:   "Component contains a composer.json manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           firstNonEmpty(info.Name, filepath.Base(dir)),
			Path:           rel,
			Language:       "PHP",
			PackageManager: "composer",
			RunCommand:     info.RunCommand,
			Port:           info.Port,
			Evidence:       ev,
		}, ev, true, nil
	}

	// 7. Elixir: mix.exs
	if _, err := os.Stat(filepath.Join(dir, "mix.exs")); err == nil {
		info, _ := detectElixirProject(dir)
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "mix.exs")),
			Detail:   "Component contains a mix.exs manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           firstNonEmpty(info.Name, filepath.Base(dir)),
			Path:           rel,
			Language:       "Elixir",
			PackageManager: "mix",
			RunCommand:     info.RunCommand,
			Port:           info.Port,
			Evidence:       ev,
		}, ev, true, nil
	}

	// 8. Swift: Package.swift
	if _, err := os.Stat(filepath.Join(dir, "Package.swift")); err == nil {
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "Package.swift")),
			Detail:   "Component contains a Package.swift manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           filepath.Base(dir),
			Path:           rel,
			Language:       "Swift",
			PackageManager: "swift",
			Evidence:       ev,
		}, ev, true, nil
	}

	// 9. Ruby: Gemfile
	if _, err := os.Stat(filepath.Join(dir, "Gemfile")); err == nil {
		ev := []Evidence{{
			Kind:     EvidenceSignalFile,
			Path:     filepath.ToSlash(filepath.Join(rel, "Gemfile")),
			Detail:   "Component contains a Gemfile manifest.",
			Strength: 0.90,
		}}
		return Component{
			Name:           filepath.Base(dir),
			Path:           rel,
			Language:       "Ruby",
			PackageManager: "bundle",
			Evidence:       ev,
		}, ev, true, nil
	}

	// 10. .NET / C#: *.csproj
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csproj") {
				name := strings.TrimSuffix(entry.Name(), ".csproj")
				ev := []Evidence{{
					Kind:     EvidenceSignalFile,
					Path:     filepath.ToSlash(filepath.Join(rel, entry.Name())),
					Detail:   fmt.Sprintf("Component contains C# project file %s.", entry.Name()),
					Strength: 0.90,
				}}
				return Component{
					Name:           name,
					Path:           rel,
					Language:       "C#",
					PackageManager: "dotnet",
					Evidence:       ev,
				}, ev, true, nil
			}
		}
	}

	// 11. Standalone entry points without manifest (supported when root has no manifest)
	if !rootHasManifest && !isSourceOrAuxiliaryDir(filepath.Base(dir)) && strings.Count(rel, "/") <= 1 {
		if _, err := os.Stat(filepath.Join(dir, "main.go")); err == nil {
			ev := []Evidence{{
				Kind:     EvidenceConfig,
				Path:     filepath.ToSlash(filepath.Join(rel, "main.go")),
				Detail:   "Component contains a Go main package entry file.",
				Strength: 0.85,
			}}
			return Component{
				Name:           filepath.Base(dir),
				Path:           rel,
				Language:       "Go",
				PackageManager: "go",
				RunCommand:     "go run main.go",
				Evidence:       ev,
			}, ev, true, nil
		}

		for _, f := range []string{"main.py", "app.py", "manage.py"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				ev := []Evidence{{
					Kind:     EvidenceConfig,
					Path:     filepath.ToSlash(filepath.Join(rel, f)),
					Detail:   fmt.Sprintf("Component contains Python entry file %s.", f),
					Strength: 0.85,
				}}
				return Component{
					Name:           filepath.Base(dir),
					Path:           rel,
					Language:       "Python",
					PackageManager: "python",
					RunCommand:     "python " + f,
					Evidence:       ev,
				}, ev, true, nil
			}
		}
	}

	return Component{}, nil, false, nil
}

func isRealRunnableRootApp(root string, rootInfo DetectedProject, childComponents []Component) bool {
	if len(childComponents) == 0 {
		return true
	}
	for _, f := range []string{"pnpm-workspace.yaml", "nx.json", "turbo.json", "lerna.json", "rush.json", "go.work"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			return hasIndependentRootEntryPoint(root)
		}
	}
	if cargoData, err := os.ReadFile(filepath.Join(root, "Cargo.toml")); err == nil {
		if strings.Contains(string(cargoData), "[workspace]") {
			return hasIndependentRootEntryPoint(root)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Workspaces interface{}       `json:"workspaces"`
			Scripts    map[string]string `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if pkg.Workspaces != nil {
				return hasIndependentRootEntryPoint(root)
			}
			if len(pkg.Scripts) > 0 {
				hasIndependentScript := false
				for sName, sCmd := range pkg.Scripts {
					if sName != "start" && sName != "dev" && sName != "serve" {
						continue
					}
					if isOrchestrationCommand(sCmd) {
						continue
					}
					hasIndependentScript = true
				}
				if !hasIndependentScript {
					return hasIndependentRootEntryPoint(root)
				}
			}
		}
	}

	return hasIndependentRootEntryPoint(root)
}

func isOrchestrationCommand(cmd string) bool {
	lower := strings.ToLower(cmd)
	for _, p := range []string{
		"turbo", "nx ", "lerna", "concurrently",
		"pnpm -r", "pnpm --recursive",
		"npm run", "npm -w", "npm --workspace",
		"yarn workspaces", "wsrun",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func hasIndependentRootEntryPoint(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "main.go")); err == nil {
		return true
	}
	for _, f := range []string{"main.py", "app.py", "manage.py", "wsgi.py"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			return true
		}
	}
	if _, err := os.Stat(filepath.Join(root, "src", "main.rs")); err == nil {
		return true
	}
	for _, f := range []string{"server.js", "server.ts", "src/server.js", "src/server.ts", "src/index.js", "src/index.ts", "index.js", "index.ts", "app.js", "app.ts"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			return true
		}
	}
	return false
}

func disambiguateComponentNames(components []Component, services []Service) []Component {
	originalNames := make([]string, len(components))
	for i := range components {
		originalNames[i] = components[i].Name
	}
	serviceNames := make(map[string]bool, len(services))
	for _, s := range services {
		serviceNames[s.Name] = true
	}

	nameCounts := make(map[string]int)
	for _, c := range components {
		nameCounts[c.Name]++
	}

	for i := range components {
		if nameCounts[components[i].Name] > 1 {
			if components[i].Path != "." {
				components[i].Name = filepath.ToSlash(components[i].Path)
			} else {
				components[i].Name = components[i].Name + "-root"
			}
		}
	}

	// Recheck nameCounts in case filepath.ToSlash(components[i].Path) had collisions
	nameCounts = make(map[string]int)
	for _, c := range components {
		nameCounts[c.Name]++
	}
	for i := range components {
		if nameCounts[components[i].Name] > 1 {
			if components[i].Path != "." {
				components[i].Name = filepath.ToSlash(components[i].Path) + "-app"
			}
		}
	}

	// Ensure Component and Compose-service identities remain distinct
	for i := range components {
		if serviceNames[components[i].Name] {
			if components[i].Path != "." {
				relPath := filepath.ToSlash(components[i].Path)
				if !serviceNames[relPath] {
					components[i].Name = relPath
				} else {
					components[i].Name = relPath + "-app"
				}
			} else {
				components[i].Name = components[i].Name + "-app"
			}
		}
	}

	return rewriteRenamedDependencies(components, originalNames)
}

// rewriteRenamedDependencies keeps component depends_on references valid when
// disambiguation renamed a component. Only an unambiguous rename is rewritten:
// the old name must have belonged to exactly one component.
func rewriteRenamedDependencies(components []Component, originalNames []string) []Component {
	oldCounts := make(map[string]int, len(originalNames))
	for _, name := range originalNames {
		oldCounts[name]++
	}
	renames := make(map[string]string)
	for i := range components {
		old := originalNames[i]
		if old != components[i].Name && oldCounts[old] == 1 {
			renames[old] = components[i].Name
		}
	}
	if len(renames) == 0 {
		return components
	}
	for i := range components {
		for j, dep := range components[i].DependsOn {
			if renamed, ok := renames[dep]; ok && renamed != components[i].Name {
				components[i].DependsOn[j] = renamed
			}
		}
	}
	return components
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
			if strings.HasPrefix(version, "workspace:") || version == "*" {
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
