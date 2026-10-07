package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepositoryOverride(t *testing.T) {
	root := t.TempDir()
	configYAML := `
spec: octo.dev/v1
project:
  name: overridden-app
components:
  web:
    run_command: "pnpm run start:custom"
    port: 8080
    env:
      NODE_ENV: production
    depends_on:
      - api
services:
  redis:
    image: "redis:7-alpine"
    ports:
      - "6379:6379"
verification:
  - component: web
    kind: http
    path: /healthz
    expected_status: 200
`
	if err := os.WriteFile(filepath.Join(root, ".octo.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	override, path, err := LoadRepositoryOverride(root)
	if err != nil {
		t.Fatalf("LoadRepositoryOverride failed: %v", err)
	}
	if override == nil {
		t.Fatal("expected override to be found")
	}
	if filepath.Base(path) != ".octo.yaml" {
		t.Fatalf("expected .octo.yaml, got %s", path)
	}
	if override.Project.Name != "overridden-app" {
		t.Fatalf("project name = %q, want 'overridden-app'", override.Project.Name)
	}
	web, ok := override.Components["web"]
	if !ok || web.RunCommand != "pnpm run start:custom" {
		t.Fatalf("web run_command = %q", web.RunCommand)
	}
	if web.Port != 8080 {
		t.Fatalf("web port = %d, want 8080", web.Port)
	}
	if len(override.Verification) != 1 || override.Verification[0].Path != "/healthz" {
		t.Fatalf("unexpected verification: %+v", override.Verification)
	}
}

func TestApplyRepositoryOverrides(t *testing.T) {
	root := t.TempDir()
	model := ProjectModel{
		Name: "original",
		Root: root,
		Components: []Component{
			{
				Name:       "web",
				Path:       "apps/web",
				RunCommand: "npm start",
				Port:       3000,
				Confidence: 0.8,
			},
		},
	}

	override := &RepositoryOverride{
		Project: OverrideProject{Name: "custom-name"},
		Components: map[string]OverrideComponent{
			"web": {
				RunCommand: "npm run custom",
				Port:       4000,
				DependsOn:  []string{"db"},
			},
			"worker": {
				RunCommand: "python worker.py",
			},
		},
		Verification: []OverrideVerificationCheck{
			{
				Component:      "web",
				Kind:           "http",
				Path:           "/ready",
				ExpectedStatus: 200,
			},
		},
	}

	ApplyRepositoryOverrides(&model, override, filepath.Join(root, ".octo.yaml"))

	if model.Name != "custom-name" {
		t.Fatalf("model.Name = %q, want custom-name", model.Name)
	}

	var webComp *Component
	var workerComp *Component
	for i := range model.Components {
		if model.Components[i].Name == "web" {
			webComp = &model.Components[i]
		}
		if model.Components[i].Name == "worker" {
			workerComp = &model.Components[i]
		}
	}

	if webComp == nil || webComp.RunCommand != "npm run custom" {
		t.Fatalf("web run_command = %q, want 'npm run custom'", webComp.RunCommand)
	}
	if webComp.Port != 4000 || !webComp.PortStrict {
		t.Fatalf("web port = %d (strict=%v), want 4000 strict", webComp.Port, webComp.PortStrict)
	}
	if len(webComp.ExecutionCandidates) == 0 || webComp.ExecutionCandidates[0].Confidence != 1.0 {
		t.Fatal("expected override candidate with confidence 1.0")
	}

	if workerComp == nil || workerComp.RunCommand != "python worker.py" {
		t.Fatalf("worker component not added properly: %+v", workerComp)
	}

	if len(model.CustomVerification) != 1 {
		t.Fatalf("expected 1 custom verification check, got %d", len(model.CustomVerification))
	}
	if model.CustomVerification[0].Kind != VerificationHTTP || model.CustomVerification[0].Path != "/ready" {
		t.Fatalf("unexpected custom verification: %+v", model.CustomVerification[0])
	}
}

func TestAnalyzeAppliesRepositoryOverride(t *testing.T) {
	root := t.TempDir()
	packageJSON := `{
  "name": "my-demo",
  "scripts": {
    "dev": "next dev",
    "start": "next start"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	overrideYAML := `
spec: octo.dev/v1
components:
  my-demo:
    run_command: "next dev --turbo"
    port: 3001
`
	if err := os.WriteFile(filepath.Join(root, ".octo.yaml"), []byte(overrideYAML), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.RunCommand != "next dev --turbo" {
		t.Fatalf("model.RunCommand = %q, want 'next dev --turbo'", model.RunCommand)
	}
	if model.Port != 3001 {
		t.Fatalf("model.Port = %d, want 3001", model.Port)
	}
}
