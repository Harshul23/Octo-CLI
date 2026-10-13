package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Helper to write files in test temp directory
func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// 1. Unmarked React + Node server + root Postgres Compose
func TestFallbackDiscoveryUnmarkedReactNodeRootComposePostgres(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "frontend/package.json", `{
		"name": "frontend",
		"dependencies": {"react": "^18.2.0"},
		"scripts": {"start": "react-scripts start"}
	}`)
	writeTestFile(t, root, "server/package.json", `{
		"name": "server",
		"dependencies": {"express": "^4.18.2"},
		"scripts": {"start": "node index.js"}
	}`)
	writeTestFile(t, root, "docker-compose.yml", `version: '3.8'
services:
  postgres:
    image: postgres:15
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: testdb
`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if !model.Monorepo {
		t.Fatal("expected multi-component repository to be marked Monorepo")
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	frontend := findComponent(model.Components, "frontend")
	if frontend == nil {
		t.Fatal("frontend component not found")
	}
	if frontend.Framework != "React" {
		t.Fatalf("expected React framework, got %q", frontend.Framework)
	}
	if frontend.Role != "application" {
		t.Fatalf("expected application role, got %q", frontend.Role)
	}

	server := findComponent(model.Components, "server")
	if server == nil {
		t.Fatal("server component not found")
	}
	if server.Framework != "Express" {
		t.Fatalf("expected Express framework, got %q", server.Framework)
	}
	if server.Role != "application" {
		t.Fatalf("expected application role, got %q", server.Role)
	}

	if len(model.Services) != 1 || model.Services[0].Name != "postgres" {
		t.Fatalf("expected 1 service 'postgres', got %+v", model.Services)
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected execution steps in plan")
	}
}

// 2. React + Go mixed runtime siblings (manifest-less root)
func TestFallbackDiscoveryReactAndGoMixedRuntime(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "frontend/package.json", `{
		"name": "web",
		"dependencies": {"react": "^18.0.0"},
		"scripts": {"start": "react-scripts start"}
	}`)
	writeTestFile(t, root, "backend/go.mod", "module example.com/backend\n\ngo 1.24\n")
	writeTestFile(t, root, "backend/main.go", "package main\nfunc main() {}\n")

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if !model.Monorepo {
		t.Fatal("expected Monorepo = true")
	}

	if model.Language != "Mixed" {
		t.Fatalf("expected Language 'Mixed', got %q", model.Language)
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	web := findComponent(model.Components, "web")
	if web == nil || web.Language != "Node" {
		t.Fatalf("expected Node component 'web', got %+v", web)
	}

	backend := findComponent(model.Components, "backend")
	if backend == nil || backend.Language != "Go" {
		t.Fatalf("expected Go component 'backend', got %+v", backend)
	}
}

// 3. Manifest-less root does not create a fake root component
func TestFallbackDiscoveryManifestlessRoot(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "api/main.go", "package main\nfunc main() {}\n")
	writeTestFile(t, root, "web/package.json", `{
		"name": "web",
		"scripts": {"start": "vite"}
	}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected exactly 2 components (no root placeholder), got %d", len(model.Components))
	}

	for _, c := range model.Components {
		if c.Path == "." {
			t.Fatal("manifest-less root should not produce a root component when child components exist")
		}
	}
}

// 4. Declared workspace with shared library (classified as build/install dependency, not runnable)
func TestDeclaredWorkspaceWithSharedLibrary(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "package.json", `{
		"name": "monorepo",
		"private": true,
		"workspaces": ["apps/*", "packages/*"]
	}`)
	writeTestFile(t, root, "apps/web/package.json", `{
		"name": "web",
		"scripts": {"start": "node server.js"},
		"dependencies": {"@repo/shared": "workspace:*"}
	}`)
	writeTestFile(t, root, "apps/web/server.js", "console.log('web')")
	// Shared library with no executable scripts or entry point
	writeTestFile(t, root, "packages/shared/package.json", `{
		"name": "@repo/shared",
		"main": "index.js"
	}`)
	writeTestFile(t, root, "packages/shared/index.js", "module.exports = {}")

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	web := findComponent(model.Components, "web")
	if web == nil {
		t.Fatal("web component not found")
	}
	if web.Role != "application" || web.RunCommand == "" {
		t.Fatalf("expected web to be runnable application, got role=%q cmd=%q", web.Role, web.RunCommand)
	}

	shared := findComponent(model.Components, "@repo/shared")
	if shared == nil {
		t.Fatal("shared component not found")
	}
	if shared.Role != "library" {
		t.Fatalf("expected shared to be library, got role=%q", shared.Role)
	}
	if shared.NonRunnableReason == "" {
		t.Fatal("expected NonRunnableReason to be populated for library")
	}

	// Plan generation must succeed and skip start phase for the library
	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed for workspace with library: %v", err)
	}

	hasSharedPrepare := false
	hasSharedStart := false
	for _, step := range plan.Steps {
		if step.Component == "@repo/shared" {
			if step.Phase == PhasePrepare || step.Phase == PhaseInstall {
				hasSharedPrepare = true
			}
			if step.Phase == PhaseStart {
				hasSharedStart = true
			}
		}
	}
	if !hasSharedPrepare {
		t.Fatal("expected library to have preparation/install steps")
	}
	if hasSharedStart {
		t.Fatal("expected library NOT to have a start step")
	}
}

// 5. Duplicate package/basename discoveries receive deterministic unique identities
func TestDuplicateComponentNamesDisambiguation(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "apps/api/package.json", `{
		"name": "api",
		"scripts": {"start": "node server.js"}
	}`)
	writeTestFile(t, root, "services/api/go.mod", "module example.com/api\n\ngo 1.24\n")
	writeTestFile(t, root, "services/api/main.go", "package main\nfunc main() {}\n")

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(model.Components))
	}

	names := []string{model.Components[0].Name, model.Components[1].Name}
	if names[0] == names[1] {
		t.Fatalf("duplicate names were not disambiguated: %v", names)
	}

	// Deterministic unique identities based on repository-relative paths
	app1 := findComponent(model.Components, "apps/api")
	app2 := findComponent(model.Components, "services/api")
	if app1 == nil || app2 == nil {
		t.Fatalf("expected names 'apps/api' and 'services/api', got %v", names)
	}
}

// 6. Component and Compose-service identities remain distinct
func TestComponentAndComposeServiceIdentityDistinction(t *testing.T) {
	root := t.TempDir()

	// Component named redis
	writeTestFile(t, root, "services/redis/package.json", `{
		"name": "redis",
		"scripts": {"start": "node worker.js"}
	}`)
	// Compose service named redis
	writeTestFile(t, root, "docker-compose.yml", `version: '3.8'
services:
  redis:
    image: redis:7
`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Services) != 1 || model.Services[0].Name != "redis" {
		t.Fatalf("expected service 'redis', got %+v", model.Services)
	}

	if len(model.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(model.Components))
	}

	if model.Components[0].Name == "redis" {
		t.Fatalf("component name must be distinct from compose service 'redis', got %q", model.Components[0].Name)
	}

	// TopologyGraph build should succeed with distinct IDs
	graph, err := BuildTopologyGraph(model)
	if err != nil {
		t.Fatalf("BuildTopologyGraph failed: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 topology nodes, got %d", len(graph.Nodes))
	}
}

// 7. Runnable root plus children: keep real runnable root app
func TestRunnableRootPlusChildren(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "package.json", `{
		"name": "root-app",
		"scripts": {"start": "node server.js"}
	}`)
	writeTestFile(t, root, "server.js", "console.log('root')")
	writeTestFile(t, root, "frontend/package.json", `{
		"name": "frontend",
		"scripts": {"start": "vite"}
	}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Components) != 2 {
		t.Fatalf("expected 2 components (root + child), got %d: %+v", len(model.Components), model.Components)
	}

	hasRoot := false
	hasChild := false
	for _, c := range model.Components {
		if c.Path == "." {
			hasRoot = true
		}
		if c.Path == "frontend" {
			hasChild = true
		}
	}
	if !hasRoot || !hasChild {
		t.Fatalf("expected both root and child component, got %+v", model.Components)
	}
}

// 8. Ignored trees & pruning (.git, node_modules, vendor, venv, cache, nested git repos)
func TestIgnoredTreesAndPruning(t *testing.T) {
	root := t.TempDir()

	// Legitimate component
	writeTestFile(t, root, "apps/main/package.json", `{
		"name": "main-app",
		"scripts": {"start": "node index.js"}
	}`)

	// Ignored directories
	writeTestFile(t, root, ".git/hooks/package.json", `{"name": "git-hook"}`)
	writeTestFile(t, root, "node_modules/dep/package.json", `{"name": "dep"}`)
	writeTestFile(t, root, "vendor/lib/go.mod", "module example.com/vendor\n")
	writeTestFile(t, root, ".venv/lib/package.json", `{"name": "venv-pkg"}`)
	writeTestFile(t, root, "dist/bundle/package.json", `{"name": "dist-pkg"}`)
	writeTestFile(t, root, "build/out/package.json", `{"name": "build-pkg"}`)
	writeTestFile(t, root, "target/debug/package.json", `{"name": "target-pkg"}`)
	writeTestFile(t, root, ".cache/cache-dir/package.json", `{"name": "cache-pkg"}`)
	// Nested unrelated repository with its own .git
	writeTestFile(t, root, "nested-repo/.git/HEAD", "ref: refs/heads/main")
	writeTestFile(t, root, "nested-repo/package.json", `{"name": "nested-repo"}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(model.Components) != 1 {
		t.Fatalf("expected only 1 component (main-app), got %d: %+v", len(model.Components), model.Components)
	}
	if model.Components[0].Path != "apps/main" {
		t.Fatalf("expected component path 'apps/main', got %q", model.Components[0].Path)
	}
}

// 9. External symlinks not followed
func TestExternalSymlinksNotFollowed(t *testing.T) {
	root := t.TempDir()
	externalDir := t.TempDir()

	writeTestFile(t, externalDir, "package.json", `{
		"name": "external-app",
		"scripts": {"start": "node ext.js"}
	}`)

	writeTestFile(t, root, "local-app/package.json", `{
		"name": "local-app",
		"scripts": {"start": "node index.js"}
	}`)

	// Symlink pointing outside the requested root
	symlinkPath := filepath.Join(root, "external-link")
	if err := os.Symlink(externalDir, symlinkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	for _, c := range model.Components {
		if c.Name == "external-app" || c.Path == "external-link" {
			t.Fatalf("external symlink was followed: %+v", c)
		}
	}
	if len(model.Components) != 1 || model.Components[0].Path != "local-app" {
		t.Fatalf("expected only local-app, got %+v", model.Components)
	}
}

// 10. Ordinary single-repo regressions (preserve unambiguous identities and lock reuse)
func TestSingleRepoRegressions(t *testing.T) {
	// Node single repo
	nodeRoot := t.TempDir()
	writeTestFile(t, nodeRoot, "package.json", `{
		"name": "my-node-app",
		"scripts": {"start": "node server.js"}
	}`)
	mNode, err := Analyze(nodeRoot)
	if err != nil {
		t.Fatalf("Analyze node failed: %v", err)
	}
	if mNode.Monorepo {
		t.Fatal("expected single Node repo Monorepo = false")
	}
	if len(mNode.Components) != 1 || mNode.Components[0].Name != "my-node-app" || mNode.Components[0].Path != "." {
		t.Fatalf("unexpected Node component: %+v", mNode.Components)
	}

	// Go single repo
	goRoot := t.TempDir()
	writeTestFile(t, goRoot, "go.mod", "module example.com/my-go-app\n\ngo 1.24\n")
	writeTestFile(t, goRoot, "main.go", "package main\nfunc main() {}\n")
	mGo, err := Analyze(goRoot)
	if err != nil {
		t.Fatalf("Analyze Go failed: %v", err)
	}
	if mGo.Monorepo {
		t.Fatal("expected single Go repo Monorepo = false")
	}
	if len(mGo.Components) != 1 || mGo.Components[0].Name != "my-go-app" || mGo.Components[0].Path != "." {
		t.Fatalf("unexpected Go component: %+v", mGo.Components)
	}
}

// 11. Nested Compose layouts reported as not yet supported diagnostics
func TestNestedComposeLayoutReportedAsUnsupportedDiagnostic(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "app/package.json", `{
		"name": "app",
		"scripts": {"start": "node index.js"}
	}`)
	writeTestFile(t, root, "services/worker/docker-compose.yml", `version: '3.8'
services:
  worker:
    image: worker:latest
`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundDiagnostic := false
	for _, diag := range model.Diagnostics {
		if diag == `nested Compose layout "services/worker/docker-compose.yml" is not yet supported` {
			foundDiagnostic = true
			break
		}
	}
	if !foundDiagnostic {
		t.Fatalf("expected diagnostic about nested Compose layout, got: %v", model.Diagnostics)
	}
}

// 12. Resource bound truncation diagnostic
func TestScanResourceBoundTruncationDiagnostic(t *testing.T) {
	root := t.TempDir()

	// Temporarily reduce maxScanDepth to 2
	origDepth := maxScanDepth
	maxScanDepth = 2
	defer func() { maxScanDepth = origDepth }()

	// Create a component at depth 3: d1/d2/d3
	writeTestFile(t, root, "d1/d2/d3/app/package.json", `{
		"name": "deep-app",
		"scripts": {"start": "node index.js"}
	}`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundDepthDiagnostic := false
	for _, diag := range model.Diagnostics {
		if len(diag) > 0 && diag[:len("directory scan depth bound")] == "directory scan depth bound" {
			foundDepthDiagnostic = true
			break
		}
	}
	if !foundDepthDiagnostic {
		t.Fatalf("expected depth bound diagnostic, got: %v", model.Diagnostics)
	}
}
