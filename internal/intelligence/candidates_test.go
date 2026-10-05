package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGoExecutionCandidateProviderPrefersWholePackage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/gum\n\ngo 1.26\n"), 0o644); err != nil { t.Fatal(err) }
	for _, name := range []string{"main.go", "gum.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package main\n"), 0o644); err != nil { t.Fatal(err) }
	}
	candidates, err := (GoExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name:"gum", Language:"Go", Path:"."})
	if err != nil { t.Fatal(err) }
	if len(candidates) != 2 { t.Fatalf("candidates=%d, want 2", len(candidates)) }
	selected, err := SelectExecutionCandidate(context.Background(), candidates)
	if err != nil { t.Fatal(err) }
	if selected.Command != "go run ." { t.Fatalf("selected=%q, want go run .", selected.Command) }
}

func TestNodeExecutionCandidateProviderDoesNotInventStartScript(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"octo","scripts":{}}`), 0o644); err != nil { t.Fatal(err) }
	candidates, err := (NodeExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name:"octo", Language:"Node", Path:".", PackageManager:"npm"})
	if err != nil { t.Fatal(err) }
	if len(candidates) != 0 { t.Fatalf("candidates=%d, want 0", len(candidates)) }
}

func TestSelectExecutionCandidateUsesDecisionProvider(t *testing.T) {
	selected, err := SelectExecutionCandidate(context.Background(), []ExecutionCandidate{
		{ID:"low", Command:"go run main.go", Confidence:0.7},
		{ID:"high", Command:"go run .", Confidence:0.9},
	})
	if err != nil { t.Fatal(err) }
	if selected.ID != "high" { t.Fatalf("selected=%q, want high", selected.ID) }
}

func TestPythonExecutionCandidateProviderUsesExplicitEntryEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("print('hello')\n"), 0o644); err != nil { t.Fatal(err) }

	candidates, err := (PythonExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name:"demo", Language:"Python", Path:"."})
	if err != nil { t.Fatal(err) }
	if len(candidates) != 1 { t.Fatalf("candidates=%d, want 1", len(candidates)) }
	if candidates[0].Command != "python main.py" { t.Fatalf("command=%q, want python main.py", candidates[0].Command) }
}

func TestPythonExecutionCandidateProviderSupportsPackageMain(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "demo")
	if err := os.Mkdir(pkg, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(pkg, "__main__.py"), []byte("print('hello')\n"), 0o644); err != nil { t.Fatal(err) }

	candidates, err := (PythonExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name:"demo", Language:"Python", Path:"."})
	if err != nil { t.Fatal(err) }
	if len(candidates) != 1 { t.Fatalf("candidates=%d, want 1", len(candidates)) }
	if candidates[0].Command != "python -m demo" { t.Fatalf("command=%q, want python -m demo", candidates[0].Command) }
}
