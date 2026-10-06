package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ExecutionCandidate struct {
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	ID string `json:"id" yaml:"id"`
	Command string `json:"command" yaml:"command"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
	Evidence []Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type CandidateProvider interface {
	Name() string
	Supports(component Component) bool
	Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error)
}

type ExecutionCandidateProviders struct { providers []CandidateProvider }

func NewExecutionCandidateProviders() ExecutionCandidateProviders {
	return ExecutionCandidateProviders{providers: []CandidateProvider{
		GoExecutionCandidateProvider{}, NodeExecutionCandidateProvider{},
		PythonExecutionCandidateProvider{}, JavaExecutionCandidateProvider{},
		RubyExecutionCandidateProvider{}, RustExecutionCandidateProvider{},
	}}
}

func (p ExecutionCandidateProviders) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	var candidates []ExecutionCandidate
	for _, provider := range p.providers {
		if !provider.Supports(component) { continue }
		found, err := provider.Candidates(ctx, root, component)
		if err != nil { return nil, fmt.Errorf("%s candidate provider: %w", provider.Name(), err) }
		candidates = append(candidates, found...)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence { return candidates[i].Confidence > candidates[j].Confidence }
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, nil
}

func SelectExecutionCandidate(ctx context.Context, candidates []ExecutionCandidate) (ExecutionCandidate, error) {
	return SelectExecutionCandidateWithProvider(ctx, candidates, OptionalDecisionProvider())
}

func SelectExecutionCandidateWithProvider(ctx context.Context, candidates []ExecutionCandidate, provider DecisionProvider) (ExecutionCandidate, error) {
	decisionCandidates := primaryExecutionCandidates(candidates)
	if provider != nil {
		if p, ok := provider.(interface{ IncludeNonPrimaryCandidates() bool }); ok && p.IncludeNonPrimaryCandidates() {
			decisionCandidates = candidates
		}
	}
	options := make([]DecisionOption, 0, len(decisionCandidates))
	for _, candidate := range decisionCandidates {
		if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.Command) == "" { continue }
		options = append(options, DecisionOption{Kind: candidate.Kind, ID: candidate.ID, Value: candidate.Command, Confidence: candidate.Confidence, Evidence: candidate.Evidence})
	}
	if len(options) == 0 { return ExecutionCandidate{}, fmt.Errorf("no primary application candidates") }
	if provider == nil { provider = DeterministicDecisionProvider{} }
	result, err := provider.Decide(ctx, DecisionRequest{Name: "run_command", Options: options})
	if err != nil { return ExecutionCandidate{}, err }
	for _, candidate := range candidates {
		if candidate.ID == result.OptionID { return candidate, nil }
	}
	return ExecutionCandidate{}, fmt.Errorf("selected candidate %q was not found", result.OptionID)
}

type GoExecutionCandidateProvider struct{}

func (GoExecutionCandidateProvider) Name() string { return "go" }
func (GoExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Go" }

func (GoExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	entries, err := os.ReadDir(componentRoot)
	if err != nil { return nil, err }

	hasMain := false
	goFiles := 0
	for _, entry := range entries {
		if entry.IsDir() { continue }
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") { continue }
		goFiles++
		hasMain = hasMain || name == "main.go"
	}

	candidates := make([]ExecutionCandidate, 0, 8)
	if hasMain {
		candidates = append(candidates, ExecutionCandidate{
			Kind: "application", ID: "go.main-file", Command: "go run main.go", Confidence: 0.78,
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, "main.go")), Detail: "A Go main package entry file exists.", Strength: 0.78}},
		})
	}
	if hasMain && goFiles > 1 {
		candidates = append(candidates, ExecutionCandidate{
			Kind: "application", ID: "go.package", Command: "go run .", Confidence: 0.96,
			Evidence: []Evidence{
				{Kind: EvidenceConfig, Path: filepath.ToSlash(component.Path), Detail: "The component contains multiple non-test Go files; run the complete package so sibling files are compiled together.", Strength: 0.96},
				{Kind: EvidenceSignalFile, Path: filepath.ToSlash(filepath.Join(component.Path, "go.mod")), Detail: "The component is a Go module.", Strength: 0.95},
			},
		})
	}

	nested, err := discoverNestedGoEntryPoints(ctx, componentRoot, component.Path)
	if err != nil { return nil, err }
	candidates = append(candidates, nested...)

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence { return candidates[i].Confidence > candidates[j].Confidence }
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, nil
}

func discoverNestedGoEntryPoints(ctx context.Context, componentRoot, componentPath string) ([]ExecutionCandidate, error) {
	candidates := make([]ExecutionCandidate, 0)
	const maxDepth = 3

	err := filepath.WalkDir(componentRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil { return err }
		if err := ctx.Err(); err != nil { return err }
		if path == componentRoot { return nil }

		rel, err := filepath.Rel(componentRoot, path)
		if err != nil { return err }
		depth := strings.Count(filepath.ToSlash(rel), "/") + 1

		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if depth > maxDepth { return filepath.SkipDir }
			return nil
		}
		if depth > maxDepth || entry.Name() != "main.go" { return nil }

		data, err := os.ReadFile(path)
		if err != nil { return err }
		if !isGoMainPackage(data) { return nil }

		dir := filepath.Dir(rel)
		slashDir := filepath.ToSlash(dir)
		if slashDir == "." { return nil }

		confidence := 0.90
		kind := "application"
		id := "go.package." + strings.ReplaceAll(slashDir, "/", ".")
		if strings.HasPrefix(slashDir, "examples/") {
			kind = "example"
			confidence = 0.84
			id = "go.example." + strings.ReplaceAll(strings.TrimPrefix(slashDir, "examples/"), "/", ".")
		} else if isGoAuxiliaryEntryPoint(slashDir) {
			kind = "tool"
			confidence = 0.80
			id = "go.tool." + strings.ReplaceAll(slashDir, "/", ".")
		}
		candidatePath := filepath.ToSlash(filepath.Join(componentPath, dir, "main.go"))
		candidates = append(candidates, ExecutionCandidate{
			Kind: kind, ID: id, Command: "go run ./" + slashDir, Confidence: confidence,
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: candidatePath, Detail: "A nested Go package contains a package-main entry point.", Strength: confidence}},
		})
		return nil
	})
	if err != nil { return nil, err }
	return candidates, nil
}

func isGoMainPackage(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "package main" || strings.HasPrefix(line, "package main ") { return true }
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "/*") { break }
	}
	return false
}

type PythonExecutionCandidateProvider struct{}
func (PythonExecutionCandidateProvider) Name() string { return "python" }
func (PythonExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Python" }
func (PythonExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	candidates := make([]ExecutionCandidate, 0, 8)

	runPrefix := ""
	switch component.PackageManager {
	case "uv":
		runPrefix = "uv run "
	case "poetry":
		runPrefix = "poetry run "
	case "pipenv":
		runPrefix = "pipenv run "
	}

	// 1. Django: check for manage.py
	if _, err := os.Stat(filepath.Join(componentRoot, "manage.py")); err == nil {
		cmd := runPrefix + "python manage.py runserver"
		candidates = append(candidates, ExecutionCandidate{
			ID:         "python.django",
			Command:    cmd,
			Confidence: 0.98,
			Evidence: []Evidence{{
				Kind:     EvidenceConfig,
				Path:     filepath.ToSlash(filepath.Join(component.Path, "manage.py")),
				Detail:   "Django manage.py entry script exists.",
				Strength: 0.98,
			}},
		})
	}

	// 2. FastAPI: check for FastAPI application entry points
	if component.Framework == "FastAPI" || hasFastAPISignal(componentRoot) {
		fastapiEntries := []struct {
			relPath string
			module  string
		}{
			{"app/main.py", "app.main:app"},
			{"main.py", "main:app"},
			{"app.py", "app:app"},
			{"app/app.py", "app.app:app"},
			{"src/main.py", "src.main:app"},
			{"src/app/main.py", "src.app.main:app"},
		}
		for _, entry := range fastapiEntries {
			if _, err := os.Stat(filepath.Join(componentRoot, entry.relPath)); err == nil {
				uvicornCmd := runPrefix + "uvicorn " + entry.module
				candidates = append(candidates, ExecutionCandidate{
					ID:         "python.fastapi." + strings.ReplaceAll(filepath.Dir(entry.relPath), "/", "."),
					Command:    uvicornCmd,
					Confidence: 0.96,
					Evidence: []Evidence{{
						Kind:     EvidenceConfig,
						Path:     filepath.ToSlash(filepath.Join(component.Path, entry.relPath)),
						Detail:   "FastAPI application entry point found at " + entry.relPath + ".",
						Strength: 0.96,
					}},
				})
				break
			}
		}
	}

	// 3. Flask: check for Flask application entry points
	if component.Framework == "Flask" {
		flaskEntries := []string{"app.py", "wsgi.py", "main.py"}
		for _, file := range flaskEntries {
			if _, err := os.Stat(filepath.Join(componentRoot, file)); err == nil {
				candidates = append(candidates, ExecutionCandidate{
					ID:         "python.flask",
					Command:    runPrefix + "flask run",
					Confidence: 0.95,
					Evidence: []Evidence{{
						Kind:     EvidenceConfig,
						Path:     filepath.ToSlash(filepath.Join(component.Path, file)),
						Detail:   "Flask application entry point found at " + file + ".",
						Strength: 0.95,
					}},
				})
				break
			}
		}
	}

	// 4. Conventional Python entry files
	entryFiles := []struct {
		file       string
		confidence float64
	}{
		{"main.py", 0.90},
		{"app.py", 0.85},
		{"app/main.py", 0.88},
		{"app/app.py", 0.82},
		{"src/main.py", 0.85},
		{"src/app.py", 0.80},
		{"wsgi.py", 0.78},
	}
	for _, entry := range entryFiles {
		if _, err := os.Stat(filepath.Join(componentRoot, entry.file)); err != nil {
			continue
		}
		id := "python." + strings.ReplaceAll(strings.TrimSuffix(entry.file, ".py"), "/", ".")
		cmd := runPrefix + "python " + entry.file
		candidates = append(candidates, ExecutionCandidate{
			ID:         id,
			Command:    cmd,
			Confidence: entry.confidence,
			Evidence: []Evidence{{
				Kind:     EvidenceConfig,
				Path:     filepath.ToSlash(filepath.Join(component.Path, entry.file)),
				Detail:   "A conventional Python application entry file exists.",
				Strength: entry.confidence,
			}},
		})
	}

	// 5. Package modules exposing __main__.py
	entries, err := os.ReadDir(componentRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "venv" || entry.Name() == "tests" {
				continue
			}
			if _, err := os.Stat(filepath.Join(componentRoot, entry.Name(), "__main__.py")); err == nil {
				candidates = append(candidates, ExecutionCandidate{
					ID:         "python.module." + entry.Name(),
					Command:    runPrefix + "python -m " + entry.Name(),
					Confidence: 0.94,
					Evidence: []Evidence{{
						Kind:     EvidenceConfig,
						Path:     filepath.ToSlash(filepath.Join(component.Path, entry.Name(), "__main__.py")),
						Detail:   "A Python package exposes an executable __main__.py module.",
						Strength: 0.94,
					}},
				})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, nil
}

func hasFastAPISignal(root string) bool {
	for _, f := range []string{"pyproject.toml", "requirements.txt", "Pipfile"} {
		if data, err := os.ReadFile(filepath.Join(root, f)); err == nil {
			if strings.Contains(strings.ToLower(string(data)), "fastapi") {
				return true
			}
		}
	}
	return false
}

type NodeExecutionCandidateProvider struct{}
func (NodeExecutionCandidateProvider) Name() string { return "node" }
func (NodeExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Node" }
func (NodeExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	data, err := os.ReadFile(filepath.Join(componentRoot, "package.json"))
	if err != nil { return nil, nil }
	var pkg struct { Scripts map[string]string `json:"scripts"` }
	if err := json.Unmarshal(data, &pkg); err != nil { return nil, err }
	candidates := make([]ExecutionCandidate, 0, 2)
	for _, script := range []struct{ name string; confidence float64 }{{"start", 0.94}, {"dev", 0.82}} {
		if _, ok := pkg.Scripts[script.name]; !ok { continue }
		pm := component.PackageManager; if pm == "" { pm = "npm" }
		command := pm + " " + script.name
		if pm == "npm" && script.name != "start" && script.name != "test" {
			command = "npm run " + script.name
		}
		candidates = append(candidates, ExecutionCandidate{ID: "node.script."+script.name, Command: command, Confidence: script.confidence, Evidence: []Evidence{{Kind: EvidenceScript, Path: filepath.ToSlash(filepath.Join(component.Path, "package.json")), Detail: "package.json defines the "+script.name+" script.", Strength: script.confidence}}})
	}
	return candidates, nil
}

type JavaExecutionCandidateProvider struct{}
func (JavaExecutionCandidateProvider) Name() string { return "java" }
func (JavaExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Java" }
func (JavaExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	candidates := make([]ExecutionCandidate, 0, 2)
	if _, err := os.Stat(filepath.Join(componentRoot, "pom.xml")); err == nil {
		data, err := os.ReadFile(filepath.Join(componentRoot, "pom.xml")); if err != nil { return nil, err }
		if strings.Contains(string(data), "spring-boot-maven-plugin") {
			command := "mvn spring-boot:run"; if _, err := os.Stat(filepath.Join(componentRoot, "mvnw")); err == nil { command = "./mvnw spring-boot:run" }
			candidates = append(candidates, ExecutionCandidate{ID: "java.maven.spring-boot", Command: command, Confidence: 0.96, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, "pom.xml")), Detail: "Maven configuration declares the Spring Boot Maven plugin.", Strength: 0.96}}})
		}
	}
	for _, buildFile := range []string{"build.gradle", "build.gradle.kts"} {
		path := filepath.Join(componentRoot, buildFile); data, err := os.ReadFile(path)
		if err != nil { if os.IsNotExist(err) { continue }; return nil, err }
		text := string(data); command := "./gradlew run"; if _, err := os.Stat(filepath.Join(componentRoot, "gradlew")); err != nil { command = "gradle run" }
		if strings.Contains(text, "org.springframework.boot") {
			command = "gradle bootRun"; if _, err := os.Stat(filepath.Join(componentRoot, "gradlew")); err == nil { command = "./gradlew bootRun" }
			candidates = append(candidates, ExecutionCandidate{ID: "java.gradle.spring-boot", Command: command, Confidence: 0.96, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, buildFile)), Detail: "Gradle configuration declares the Spring Boot plugin.", Strength: 0.96}}}); continue
		}
		if strings.Contains(text, "application") {
			candidates = append(candidates, ExecutionCandidate{ID: "java.gradle.application", Command: command, Confidence: 0.90, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, buildFile)), Detail: "Gradle configuration declares the application plugin, which provides the run task.", Strength: 0.90}}})
		}
	}
	return candidates, nil
}

type RubyExecutionCandidateProvider struct{}
func (RubyExecutionCandidateProvider) Name() string { return "ruby" }
func (RubyExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Ruby" }
func (RubyExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	if _, err := os.Stat(filepath.Join(componentRoot, "Gemfile")); err != nil { if os.IsNotExist(err) { return nil, nil }; return nil, err }
	candidates := make([]ExecutionCandidate, 0, 2)
	if _, err := os.Stat(filepath.Join(componentRoot, "bin", "rails")); err == nil {
		candidates = append(candidates, ExecutionCandidate{ID: "ruby.rails", Command: "bin/rails server", Confidence: 0.96, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, "bin", "rails")), Detail: "The repository exposes the Rails executable under bin/rails.", Strength: 0.96}}})
	}
	for _, file := range []string{"main.rb", "app.rb"} {
		if _, err := os.Stat(filepath.Join(componentRoot, file)); err != nil { continue }
		candidates = append(candidates, ExecutionCandidate{ID: "ruby."+strings.TrimSuffix(file, ".rb"), Command: "bundle exec ruby "+file, Confidence: 0.88, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, file)), Detail: "A conventional Ruby application entry file exists.", Strength: 0.88}}})
	}
	return candidates, nil
}

type RustExecutionCandidateProvider struct{}
func (RustExecutionCandidateProvider) Name() string { return "rust" }
func (RustExecutionCandidateProvider) Supports(component Component) bool { return component.Language == "Rust" }
func (RustExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	componentRoot := root
	if component.Path != "" && component.Path != "." { componentRoot = filepath.Join(root, filepath.FromSlash(component.Path)) }
	candidates := make([]ExecutionCandidate, 0, 4)
	mainPath := filepath.Join(componentRoot, "src", "main.rs")
	if _, err := os.Stat(mainPath); err == nil {
		runCmd := "cargo run"
		if component.Path != "" && component.Path != "." {
			runCmd = "cargo run -p " + component.Name
		}
		candidates = append(candidates, ExecutionCandidate{ID: "rust.cargo-run", Command: runCmd, Confidence: 0.96, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, "src", "main.rs")), Detail: "Cargo project contains the conventional binary entry point at src/main.rs.", Strength: 0.96}}})
	}
	binDir := filepath.Join(componentRoot, "src", "bin")
	entries, err := os.ReadDir(binDir); if err != nil && !os.IsNotExist(err) { return nil, err }
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".rs" { continue }
		name := strings.TrimSuffix(entry.Name(), ".rs"); if name == "" { continue }
		binCmd := "cargo run --bin " + name
		if component.Path != "" && component.Path != "." {
			binCmd = "cargo run -p " + component.Name + " --bin " + name
		}
		candidates = append(candidates, ExecutionCandidate{ID: "rust.bin."+name, Command: binCmd, Confidence: 0.94, Evidence: []Evidence{{Kind: EvidenceConfig, Path: filepath.ToSlash(filepath.Join(component.Path, "src", "bin", entry.Name())), Detail: "Cargo project declares an executable binary under src/bin.", Strength: 0.94}}})
	}
	return candidates, nil
}


func primaryExecutionCandidates(candidates []ExecutionCandidate) []ExecutionCandidate {
	primary := make([]ExecutionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Kind == "" || candidate.Kind == "application" {
			primary = append(primary, candidate)
		}
	}
	return primary
}

func isGoAuxiliaryEntryPoint(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "docs", "scripts", "tools", "hack":
		return true
	default:
		return false
	}
}
