package intelligence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const OctoLockSpec = "octo.dev/lock/v1"

type VerifiedStrategy struct {
	Component  string    `yaml:"component" json:"component"`
	Candidate  string    `yaml:"candidate" json:"candidate"`
	Command    string    `yaml:"command" json:"command"`
	Fingerprint string   `yaml:"fingerprint" json:"fingerprint"`
	VerifiedAt time.Time `yaml:"verified_at" json:"verified_at"`
}

type OctoLock struct {
	Spec       string             `yaml:"spec" json:"spec"`
	Project    string             `yaml:"project" json:"project"`
	Strategies []VerifiedStrategy `yaml:"strategies,omitempty" json:"strategies,omitempty"`
}

type VerifiedStrategyApplication struct {
	Reused       []VerifiedStrategy
	Invalidated  []VerifiedStrategy
}

func LoadOctoLock(root string) (OctoLock, error) {
	path := filepath.Join(root, ".octo.lock")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return OctoLock{Spec: OctoLockSpec}, nil
	}
	if err != nil {
		return OctoLock{}, err
	}
	var lock OctoLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return OctoLock{}, err
	}
	if lock.Spec == "" {
		lock.Spec = OctoLockSpec
	}
	return lock, nil
}

func SaveOctoLock(root string, lock OctoLock) error {
	lock.Spec = OctoLockSpec
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ".octo.lock"), data, 0o644)
}

// ApplyVerifiedStrategies reuses only strategies whose candidate still exists
// and whose execution-relevant repository fingerprint is unchanged.
func ApplyVerifiedStrategies(root string, model *ProjectModel, lock OctoLock) (VerifiedStrategyApplication, error) {
	var result VerifiedStrategyApplication

	for i := range model.Components {
		component := &model.Components[i]
		matchedLock := false
		reused := false

		for _, strategy := range lock.Strategies {
			if strategy.Component != component.Name {
				continue
			}
			matchedLock = true

			for _, candidate := range component.ExecutionCandidates {
				if candidate.ID != strategy.Candidate || candidate.Command != strategy.Command {
					continue
				}

				fingerprint, err := ExecutionFingerprint(root, *component, candidate)
				if err != nil {
					return result, err
				}
				if fingerprint != strategy.Fingerprint {
					continue
				}

				component.RunCommand = candidate.Command
				component.Confidence = candidate.Confidence
				if component.Path == "." && i == 0 {
					model.RunCommand = candidate.Command
					model.Confidence = candidate.Confidence
				}
				result.Reused = append(result.Reused, strategy)
				reused = true
				break
			}

			if reused {
				break
			}
		}

		if matchedLock && !reused {
			for _, strategy := range lock.Strategies {
				if strategy.Component == component.Name {
					result.Invalidated = append(result.Invalidated, strategy)
					break
				}
			}
		}
	}

	return result, nil
}

// RecordVerifiedStrategies persists successful execution candidates after the
// complete execution plan and deterministic verification have passed.
func RecordVerifiedStrategies(root string, model ProjectModel, plan ExecutionPlan, report ExecutionReport, lock OctoLock) error {
	if !report.Success {
		return nil
	}
	lock.Spec = OctoLockSpec
	lock.Project = model.Name

	byStep := make(map[string]ExecutionStepResult)
	for _, result := range report.Steps {
		if result.Status == StepSucceeded && result.CandidateID != "" {
			byStep[result.ID] = result
		}
	}

	for _, step := range plan.Steps {
		result, ok := byStep[step.ID]
		if !ok {
			continue
		}
		if step.Component == "" || result.CandidateID == "" {
			continue
		}
		var candidate *ExecutionCandidate
		for i := range step.Candidates {
			if step.Candidates[i].ID == result.CandidateID {
				candidate = &step.Candidates[i]
				break
			}
		}
		if candidate == nil {
			continue
		}
		component := findVerifiedStrategyComponent(model.Components, step.Component)
		if component == nil {
			continue
		}
		fingerprint, err := ExecutionFingerprint(root, *component, *candidate)
		if err != nil {
			return err
		}
		entry := VerifiedStrategy{
			Component: component.Name,
			Candidate: candidate.ID,
			Command: candidate.Command,
			Fingerprint: fingerprint,
			VerifiedAt: time.Now().UTC(),
		}
		for _, existing := range lock.Strategies {
			if existing.Component == entry.Component && existing.Candidate == entry.Candidate && existing.Command == entry.Command && existing.Fingerprint == entry.Fingerprint {
				entry.VerifiedAt = existing.VerifiedAt
				break
			}
		}
		replaced := false
		for i := range lock.Strategies {
			if lock.Strategies[i].Component == entry.Component {
				lock.Strategies[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			lock.Strategies = append(lock.Strategies, entry)
		}
	}
	sort.Slice(lock.Strategies, func(i, j int) bool {
		return lock.Strategies[i].Component < lock.Strategies[j].Component
	})
	return SaveOctoLock(root, lock)
}

func findVerifiedStrategyComponent(components []Component, name string) *Component {
	for i := range components {
		if components[i].Name == name {
			return &components[i]
		}
	}
	return nil
}

// ExecutionFingerprint hashes execution-relevant files and the candidate's
// evidence. Documentation and generated/dependency trees are deliberately
// excluded, so unrelated edits do not invalidate verified strategies.
func ExecutionFingerprint(root string, component Component, candidate ExecutionCandidate) (string, error) {
	h := sha256.New()
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}

	files := make(map[string]struct{})
	err := filepath.WalkDir(componentRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(componentRoot, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel != "." && ignoredExecutionDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isExecutionRelevantFile(rel, component.Language) {
			files[filepath.ToSlash(rel)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	for _, evidence := range candidate.Evidence {
		p := filepath.FromSlash(evidence.Path)
		if evidence.Path == "" {
			continue
		}
		if p == "." {
			continue
		}
		full := filepath.Join(root, p)
		if _, err := os.Stat(full); err == nil {
			rel, err := filepath.Rel(componentRoot, full)
			if err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
				files[filepath.ToSlash(rel)] = struct{}{}
			}
		}
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	h.Write([]byte("component=" + component.Name + "\n"))
	h.Write([]byte("language=" + component.Language + "\n"))
	h.Write([]byte("candidate=" + candidate.ID + "\n"))
	h.Write([]byte("command=" + candidate.Command + "\n"))
	for _, rel := range paths {
		full := filepath.Join(componentRoot, filepath.FromSlash(rel))
		data, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func ignoredExecutionDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "target", "dist", "build", ".next",
		"coverage", "__pycache__", ".venv", "venv", ".tox", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func isExecutionRelevantFile(path, language string) bool {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))
	if base == ".octo.lock" || strings.HasPrefix(base, ".env") {
		return false
	}
	switch base {
	case "Dockerfile", "Makefile", "Taskfile", "Taskfile.yml", "Taskfile.yaml",
		"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock",
		"go.mod", "go.sum", "go.work", "Cargo.toml", "Cargo.lock", "pom.xml", "build.gradle",
		"build.gradle.kts", "requirements.txt", "pyproject.toml", "poetry.lock", "uv.lock",
		"Pipfile", "Pipfile.lock", "Gemfile", "Gemfile.lock",
		"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
		"pnpm-workspace.yaml", "nx.json", "turbo.json", "lerna.json", "rush.json":
		return true
	}
	switch language {
	case "Go":
		return ext == ".go"
	case "Node":
		return ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" || ext == ".mjs" || ext == ".cjs"
	case "Python":
		return ext == ".py"
	case "Rust":
		return ext == ".rs"
	case "Java":
		return ext == ".java" || ext == ".kt" || ext == ".kts"
	case "Ruby":
		return ext == ".rb"
	default:
		return false
	}
}
