package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverComponentPortFromCommand(t *testing.T) {
	component := Component{Name: "web", Path: ".", Language: "Node"}
	port, evidence, err := discoverComponentPort("/tmp/project", component, "npm run dev -- --port 4173")
	if err != nil {
		t.Fatal(err)
	}
	if port != 4173 {
		t.Fatalf("port=%d, want 4173", port)
	}
	if len(evidence) != 1 || evidence[0].Kind != EvidenceScript {
		t.Fatalf("unexpected evidence: %#v", evidence)
	}
}

func TestAnalyzeNodeScriptPort(t *testing.T) {
	root := t.TempDir()
	writeFile := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("package.json", `{"name":"web","scripts":{"start":"vite --host 0.0.0.0 --port 4173"},"dependencies":{"vite":"7.0.0"}}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if model.Port != 4173 {
		t.Fatalf("port=%d, want 4173", model.Port)
	}
	if len(model.Components) != 1 || model.Components[0].Port != 4173 {
		t.Fatalf("component port=%d, want 4173", model.Components[0].Port)
	}
}

func TestAnalyzeJavaApplicationPort(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "main", "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main", "resources", "application.properties"), []byte("server.port=8081\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if model.Port != 8081 {
		t.Fatalf("port=%d, want 8081", model.Port)
	}
}

func TestAnalyzeDoesNotInferPortFromLanguage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if model.Port != 0 {
		t.Fatalf("port=%d, want 0", model.Port)
	}
}
