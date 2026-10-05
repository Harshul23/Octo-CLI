package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGoExecutionCandidateProviderDiscoversCmdEntryPoint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "cmd", "server")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, err := (GoExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name: "demo", Language: "Go", Path: "."})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates=%+v, want one nested candidate", candidates)
	}
	if candidates[0].Kind != "application" || candidates[0].ID != "go.package.cmd.server" || candidates[0].Command != "go run ./cmd/server" {
		t.Fatalf("candidate=%+v", candidates[0])
	}
}

func TestGoExecutionCandidateProviderDiscoversExampleEntryPoint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "examples", "hello")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidates, err := (GoExecutionCandidateProvider{}).Candidates(context.Background(), root, Component{Name: "demo", Language: "Go", Path: "."})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates=%+v, want one nested candidate", candidates)
	}
	if candidates[0].Kind != "example" || candidates[0].ID != "go.example.hello" || candidates[0].Command != "go run ./examples/hello" {
		t.Fatalf("candidate=%+v", candidates[0])
	}
}
