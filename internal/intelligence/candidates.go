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
	ID         string     `json:"id" yaml:"id"`
	Command    string     `json:"command" yaml:"command"`
	Confidence float64    `json:"confidence" yaml:"confidence"`
	Evidence   []Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

type CandidateProvider interface {
	Name() string
	Supports(component Component) bool
	Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error)
}

type ExecutionCandidateProviders struct {
	providers []CandidateProvider
}

func NewExecutionCandidateProviders() ExecutionCandidateProviders {
	return ExecutionCandidateProviders{
		providers: []CandidateProvider{
			GoExecutionCandidateProvider{},
			NodeExecutionCandidateProvider{},
			PythonExecutionCandidateProvider{},
			RustExecutionCandidateProvider{},
		},
	}
}

func (p ExecutionCandidateProviders) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	var candidates []ExecutionCandidate
	for _, provider := range p.providers {
		if !provider.Supports(component) {
			continue
		}
		found, err := provider.Candidates(ctx, root, component)
		if err != nil {
			return nil, fmt.Errorf("%s candidate provider: %w", provider.Name(), err)
		}
		candidates = append(candidates, found...)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, nil
}

func SelectExecutionCandidate(ctx context.Context, candidates []ExecutionCandidate) (ExecutionCandidate, error) {
	return SelectExecutionCandidateWithProvider(ctx, candidates, OptionalDecisionProvider())
}

func SelectExecutionCandidateWithProvider(ctx context.Context, candidates []ExecutionCandidate, provider DecisionProvider) (ExecutionCandidate, error) {
	options := make([]DecisionOption, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.Command) == "" {
			continue
		}
		options = append(options, DecisionOption{
			ID: candidate.ID, Value: candidate.Command,
			Confidence: candidate.Confidence, Evidence: candidate.Evidence,
		})
	}
	if len(options) == 0 {
		return ExecutionCandidate{}, fmt.Errorf("no executable candidates")
	}
	if provider == nil {
		provider = DeterministicDecisionProvider{}
	}
	result, err := provider.Decide(ctx, DecisionRequest{
		Name: "run_command", Options: options,
	})
	if err != nil {
		return ExecutionCandidate{}, err
	}
	for _, candidate := range candidates {
		if candidate.ID == result.OptionID {
			return candidate, nil
		}
	}
	return ExecutionCandidate{}, fmt.Errorf("selected candidate %q was not found", result.OptionID)
}

type GoExecutionCandidateProvider struct{}

func (GoExecutionCandidateProvider) Name() string { return "go" }

func (GoExecutionCandidateProvider) Supports(component Component) bool {
	return component.Language == "Go"
}

func (GoExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}
	entries, err := os.ReadDir(componentRoot)
	if err != nil {
		return nil, err
	}
	hasMain := false
	goFiles := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		goFiles++
		hasMain = hasMain || name == "main.go"
	}

	candidates := make([]ExecutionCandidate, 0, 2)
	if hasMain {
		candidates = append(candidates, ExecutionCandidate{
			ID: "go.main-file", Command: "go run main.go", Confidence: 0.78,
			Evidence: []Evidence{{
				Kind: EvidenceConfig,
				Path: filepath.ToSlash(filepath.Join(component.Path, "main.go")),
				Detail: "A Go main package entry file exists.", Strength: 0.78,
			}},
		})
	}
	if hasMain && goFiles > 1 {
		candidates = append(candidates, ExecutionCandidate{
			ID: "go.package", Command: "go run .", Confidence: 0.96,
			Evidence: []Evidence{
				{
					Kind: EvidenceConfig, Path: filepath.ToSlash(component.Path),
					Detail: "The component contains multiple non-test Go files; run the complete package so sibling files are compiled together.", Strength: 0.96,
				},
				{
					Kind: EvidenceSignalFile, Path: filepath.ToSlash(filepath.Join(component.Path, "go.mod")),
					Detail: "The component is a Go module.", Strength: 0.95,
				},
			},
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates, nil
}

type PythonExecutionCandidateProvider struct{}

func (PythonExecutionCandidateProvider) Name() string { return "python" }

func (PythonExecutionCandidateProvider) Supports(component Component) bool {
	return component.Language == "Python"
}

func (PythonExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}

	candidates := make([]ExecutionCandidate, 0, 3)
	pythonCommand := "python"
	if component.PackageManager == "uv" {
		pythonCommand = "uv run python"
	} else if component.PackageManager == "poetry" {
		pythonCommand = "poetry run python"
	}
	for _, entry := range []struct {
		file string
		confidence float64
	}{
		{file: "main.py", confidence: 0.90},
		{file: "app.py", confidence: 0.82},
	} {
		if _, err := os.Stat(filepath.Join(componentRoot, entry.file)); err != nil {
			continue
		}
		candidates = append(candidates, ExecutionCandidate{
			ID: "python." + strings.TrimSuffix(entry.file, ".py"),
			Command: pythonCommand + " " + entry.file,
			Confidence: entry.confidence,
			Evidence: []Evidence{{
				Kind: EvidenceConfig,
				Path: filepath.ToSlash(filepath.Join(component.Path, entry.file)),
				Detail: "A conventional Python application entry file exists.",
				Strength: entry.confidence,
			}},
		})
	}

	entries, err := os.ReadDir(componentRoot)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(componentRoot, entry.Name(), "__main__.py")); err != nil {
			continue
		}
		candidates = append(candidates, ExecutionCandidate{
			ID: "python.module." + entry.Name(),
			Command: pythonCommand + " -m " + entry.Name(),
			Confidence: 0.94,
			Evidence: []Evidence{{
				Kind: EvidenceConfig,
				Path: filepath.ToSlash(filepath.Join(component.Path, entry.Name(), "__main__.py")),
				Detail: "A Python package exposes an executable __main__.py module.",
				Strength: 0.94,
			}},
		})
	}
	return candidates, nil
}

type NodeExecutionCandidateProvider struct{}

func (NodeExecutionCandidateProvider) Name() string { return "node" }

func (NodeExecutionCandidateProvider) Supports(component Component) bool {
	return component.Language == "Node"
}

func (NodeExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}
	data, err := os.ReadFile(filepath.Join(componentRoot, "package.json"))
	if err != nil {
		return nil, nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	candidates := make([]ExecutionCandidate, 0, 2)
	for _, script := range []struct {
		name string
		confidence float64
	}{
		{name: "start", confidence: 0.94},
		{name: "dev", confidence: 0.82},
	} {
		if _, ok := pkg.Scripts[script.name]; !ok {
			continue
		}
		pm := component.PackageManager
		if pm == "" {
			pm = "npm"
		}
		candidates = append(candidates, ExecutionCandidate{
			ID: "node.script." + script.name, Command: pm + " " + script.name,
			Confidence: script.confidence,
			Evidence: []Evidence{{
				Kind: EvidenceScript,
				Path: filepath.ToSlash(filepath.Join(component.Path, "package.json")),
				Detail: "package.json defines the " + script.name + " script.", Strength: script.confidence,
			}},
		})
	}
	return candidates, nil
}


type RustExecutionCandidateProvider struct{}

func (RustExecutionCandidateProvider) Name() string { return "rust" }

func (RustExecutionCandidateProvider) Supports(component Component) bool {
	return component.Language == "Rust"
}

func (RustExecutionCandidateProvider) Candidates(ctx context.Context, root string, component Component) ([]ExecutionCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}

	candidates := make([]ExecutionCandidate, 0, 4)
	mainPath := filepath.Join(componentRoot, "src", "main.rs")
	if _, err := os.Stat(mainPath); err == nil {
		candidates = append(candidates, ExecutionCandidate{
			ID: "rust.cargo-run", Command: "cargo run", Confidence: 0.96,
			Evidence: []Evidence{{
				Kind: EvidenceConfig,
				Path: filepath.ToSlash(filepath.Join(component.Path, "src", "main.rs")),
				Detail: "Cargo project contains the conventional binary entry point at src/main.rs.",
				Strength: 0.96,
			}},
		})
	}

	binDir := filepath.Join(componentRoot, "src", "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".rs" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".rs")
		if name == "" {
			continue
		}
		candidates = append(candidates, ExecutionCandidate{
			ID: "rust.bin." + name,
			Command: "cargo run --bin " + name,
			Confidence: 0.94,
			Evidence: []Evidence{{
				Kind: EvidenceConfig,
				Path: filepath.ToSlash(filepath.Join(component.Path, "src", "bin", entry.Name())),
				Detail: "Cargo project declares an executable binary under src/bin.",
				Strength: 0.94,
			}},
		})
	}

	return candidates, nil
}
