package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeDiscoversDeclaredWorkspaceComponents(t *testing.T) {
	root := t.TempDir()

	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("package.json", `{
		"name": "stack",
		"private": true,
		"workspaces": ["apps/*"],
		"scripts": {"dev": "echo root"}
	}`)
	write("apps/api/package.json", `{
		"name": "@demo/api",
		"scripts": {"dev": "node server.js"}
	}`)
	write("apps/web/package.json", `{
		"name": "@demo/web",
		"scripts": {"dev": "next dev"},
		"dependencies": {"@demo/api": "workspace:*"}
	}`)
	write("apps/web/pnpm-lock.yaml", "lockfileVersion: '9.0'")

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}

	if !model.Monorepo {
		t.Fatal("expected monorepo")
	}
	if len(model.Components) != 2 {
		t.Fatalf("components=%d, want 2", len(model.Components))
	}

	web := findComponent(model.Components, "@demo/web")
	if web == nil {
		t.Fatal("web component not found")
	}
	if len(web.DependsOn) != 1 || web.DependsOn[0] != "@demo/api" {
		t.Fatalf("web dependencies=%v", web.DependsOn)
	}
	if web.Path != "apps/web" {
		t.Fatalf("web path=%q", web.Path)
	}
	if len(web.Evidence) == 0 {
		t.Fatal("expected component evidence")
	}
}

func TestPnpmWorkspaceDiscoveryUsesDeclaredPackages(t *testing.T) {
	root := t.TempDir()

	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("package.json", `{
		"name": "stack",
		"scripts": {"dev": "echo root"}
	}`)
	write("pnpm-workspace.yaml", "packages:\n  - 'services/*'\n")
	write("services/api/package.json", `{
		"name": "api",
		"scripts": {"dev": "node server.js"}
	}`)
	write("services/worker/package.json", `{
		"name": "worker",
		"scripts": {"dev": "node worker.js"}
	}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Components) != 2 {
		t.Fatalf("components=%d, want 2", len(model.Components))
	}
	if model.Components[0].Path != "services/api" || model.Components[1].Path != "services/worker" {
		t.Fatalf("components=%+v", model.Components)
	}
}

func findComponent(components []Component, name string) *Component {
	for i := range components {
		if components[i].Name == name {
			return &components[i]
		}
	}
	return nil
}
