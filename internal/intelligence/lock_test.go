package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionFingerprintChangesForSourceEdit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.go")
	if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	component := Component{Name: "app", Path: ".", Language: "Go"}
	candidate := ExecutionCandidate{ID: "go.package", Command: "go run .", Evidence: []Evidence{{Kind: EvidenceConfig, Path: ".", Detail: "package", Strength: 0.9}}}
	before, err := ExecutionFingerprint(root, component, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("package main\nfunc main() { println(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := ExecutionFingerprint(root, component, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("expected source edit to invalidate fingerprint")
	}
}

func TestExecutionFingerprintIgnoresReadmeEdit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	component := Component{Name: "app", Path: ".", Language: "Go"}
	candidate := ExecutionCandidate{ID: "go.package", Command: "go run ."}
	before, err := ExecutionFingerprint(root, component, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := ExecutionFingerprint(root, component, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("README edit should not invalidate execution fingerprint")
	}
}

func TestApplyVerifiedStrategiesRequiresMatchingFingerprint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := ExecutionCandidate{ID: "go.package", Command: "go run .", Confidence: 0.96}
	component := Component{Name: "app", Path: ".", Language: "Go", RunCommand: "go run .", ExecutionCandidates: []ExecutionCandidate{candidate}}
	fingerprint, err := ExecutionFingerprint(root, component, candidate)
	if err != nil {
		t.Fatal(err)
	}
	model := ProjectModel{Name: "app", Components: []Component{component}}
	lock := OctoLock{Project: "app", Strategies: []VerifiedStrategy{{Component: "app", Candidate: "go.package", Command: "go run .", Fingerprint: fingerprint}}}
	status, err := ApplyVerifiedStrategies(root, &model, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Reused) != 1 || status.Reused[0].Candidate != "go.package" {
		t.Fatalf("status=%+v", status)
	}
	if len(status.Invalidated) != 0 {
		t.Fatalf("unexpected invalidated strategies=%+v", status.Invalidated)
	}
	if model.Components[0].RunCommand != "go run ." {
		t.Fatalf("run command=%q", model.Components[0].RunCommand)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() { println(2) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model.Components[0].RunCommand = ""
	model.Components[0].Confidence = 0
status, err = ApplyVerifiedStrategies(root, &model, lock)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Reused) != 0 || len(status.Invalidated) != 1 {
		t.Fatalf("status=%+v", status)
	}
	if model.Components[0].RunCommand != "" || model.Components[0].Confidence != 0 {
		t.Fatal("stale lock must not be applied")
	}
}

func TestSaveAndLoadOctoLock(t *testing.T) {
	root := t.TempDir()
	lock := OctoLock{Project: "demo", Strategies: []VerifiedStrategy{{Component: "demo", Candidate: "go.package", Command: "go run .", Fingerprint: "sha256:test"}}}
	if err := SaveOctoLock(root, lock); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOctoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec != OctoLockSpec || len(got.Strategies) != 1 || got.Strategies[0].Candidate != "go.package" {
		t.Fatalf("lock=%+v", got)
	}
}
