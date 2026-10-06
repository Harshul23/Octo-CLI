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

func TestGoWorkspaceDiscovery(t *testing.T) {
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

	write("go.work", `go 1.24
use (
	./services/api
	./services/web
)
`)
	write("services/api/go.mod", "module example.com/api\n\ngo 1.24\n")
	write("services/api/main.go", "package main\nfunc main() {}\n")

	write("services/web/go.mod", "module example.com/web\n\ngo 1.24\nrequire example.com/api v0.0.0\n")
	write("services/web/main.go", "package main\nfunc main() {}\n")
	write("services/web/web.go", "package main\n")

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}

	if !model.Monorepo {
		t.Fatal("expected Go workspace to be detected as monorepo")
	}
	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	web := findComponent(model.Components, "web")
	if web == nil {
		t.Fatal("web component not found")
	}
	if len(web.DependsOn) != 1 || web.DependsOn[0] != "api" {
		t.Fatalf("expected web to depend on api: %v", web.DependsOn)
	}
	if web.RunCommand != "go run ." {
		t.Fatalf("expected run command 'go run .', got %q", web.RunCommand)
	}
}

func TestCargoWorkspaceDiscovery(t *testing.T) {
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

	write("Cargo.toml", `[workspace]
members = [
	"crates/cli",
	"crates/core",
]
`)
	write("crates/core/Cargo.toml", `[package]
name = "core"
version = "0.1.0"
`)
	write("crates/core/src/lib.rs", "pub fn hello() {}")

	write("crates/cli/Cargo.toml", `[package]
name = "cli"
version = "0.1.0"

[dependencies]
core = { path = "../core" }
`)
	write("crates/cli/src/main.rs", "fn main() {}")

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}

	if !model.Monorepo {
		t.Fatal("expected Cargo workspace to be detected as monorepo")
	}
	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	cli := findComponent(model.Components, "cli")
	if cli == nil {
		t.Fatal("cli component not found")
	}
	if len(cli.DependsOn) != 1 || cli.DependsOn[0] != "core" {
		t.Fatalf("expected cli to depend on core: %v", cli.DependsOn)
	}
	if cli.RunCommand != "cargo run -p cli" {
		t.Fatalf("expected run command 'cargo run -p cli', got %q", cli.RunCommand)
	}
}
