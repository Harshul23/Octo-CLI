package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegression_NextJSAppRouter(t *testing.T) {
	root := t.TempDir()
	packageJSON := `{
  "name": "nextjs-app",
  "version": "0.1.0",
  "scripts": {
    "dev": "next dev -p 3000",
    "build": "next build",
    "start": "next start -p 3000"
  },
  "dependencies": {
    "next": "14.2.0",
    "react": "^18",
    "react-dom": "^18"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lockfileVersion: '6.0'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "next.config.mjs"), []byte("export default {};\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Node" {
		t.Fatalf("Language = %q, want 'Node'", model.Language)
	}
	if model.PackageManager != "pnpm" {
		t.Fatalf("PackageManager = %q, want 'pnpm'", model.PackageManager)
	}
	if model.Framework != "Next.js" {
		t.Fatalf("Framework = %q, want 'Next.js'", model.Framework)
	}
	if model.RunCommand != "pnpm start" && model.RunCommand != "next dev" && model.RunCommand != "pnpm dev" {
		t.Fatalf("RunCommand = %q, want 'pnpm start' or 'next dev'", model.RunCommand)
	}
	if model.Port != 3000 {
		t.Fatalf("Port = %d, want 3000", model.Port)
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected non-empty plan steps")
	}

	traces := BuildDecisionTrace(model, plan)
	if len(traces) == 0 {
		t.Fatal("expected decision traces")
	}
}

func TestRegression_FastAPIPoetry(t *testing.T) {
	root := t.TempDir()
	pyproject := `[tool.poetry]
name = "fastapi-service"
version = "0.1.0"
description = "FastAPI Service"
authors = ["Acme <dev@acme.org>"]

[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.110.0"
uvicorn = "^0.28.0"
`
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "poetry.lock"), []byte("# poetry lock\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "app"), 0755)
	if err := os.WriteFile(filepath.Join(root, "app", "main.py"), []byte("from fastapi import FastAPI\napp = FastAPI()\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Python" {
		t.Fatalf("Language = %q, want 'Python'", model.Language)
	}
	if model.PackageManager != "poetry" {
		t.Fatalf("PackageManager = %q, want 'poetry'", model.PackageManager)
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected plan steps")
	}
}

func TestRegression_GoWorkspaceMicroservicesWithCompose(t *testing.T) {
	root := t.TempDir()
	goWork := `go 1.24
use (
	./services/api
	./services/worker
)
`
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(goWork), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. API service
	_ = os.MkdirAll(filepath.Join(root, "services", "api"), 0755)
	if err := os.WriteFile(filepath.Join(root, "services", "api", "go.mod"), []byte("module example.com/api\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "services", "api", "main.go"), []byte("package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8080\", nil) }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Worker service
	_ = os.MkdirAll(filepath.Join(root, "services", "worker"), 0755)
	if err := os.WriteFile(filepath.Join(root, "services", "worker", "go.mod"), []byte("module example.com/worker\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "services", "worker", "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Compose file
	composeYAML := `services:
  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
`
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte(composeYAML), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if !model.Monorepo {
		t.Fatal("expected model to be recognized as monorepo")
	}
	if len(model.Components) < 2 {
		t.Fatalf("expected at least 2 components, got %d", len(model.Components))
	}
	if len(model.Services) != 2 {
		t.Fatalf("expected 2 compose services, got %d", len(model.Services))
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// Verify compose services are provisioned and dependencies ordered
	hasServiceStep := false
	for _, step := range plan.Steps {
		if step.Phase == PhaseProvision {
			hasServiceStep = true
			break
		}
	}
	if !hasServiceStep {
		t.Fatal("expected PhaseProvision step for compose infrastructure services")
	}
}

func TestRegression_RustAxumCargoWorkspace(t *testing.T) {
	root := t.TempDir()
	cargoWorkspace := `[workspace]
members = [
    "crates/server",
    "crates/worker",
]
`
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(cargoWorkspace), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.lock"), []byte("# lock\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Crates
	_ = os.MkdirAll(filepath.Join(root, "crates", "server", "src"), 0755)
	if err := os.WriteFile(filepath.Join(root, "crates", "server", "Cargo.toml"), []byte("[package]\nname = \"server\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "crates", "server", "src", "main.rs"), []byte("fn main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_ = os.MkdirAll(filepath.Join(root, "crates", "worker", "src"), 0755)
	if err := os.WriteFile(filepath.Join(root, "crates", "worker", "Cargo.toml"), []byte("[package]\nname = \"worker\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "crates", "worker", "src", "main.rs"), []byte("fn main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Rust" {
		t.Fatalf("Language = %q, want 'Rust'", model.Language)
	}
	if model.PackageManager != "cargo" {
		t.Fatalf("PackageManager = %q, want 'cargo'", model.PackageManager)
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected plan steps")
	}
}

func TestRegression_SpringBootGradle(t *testing.T) {
	root := t.TempDir()
	buildGradle := `plugins {
    id 'org.springframework.boot' version '3.2.0'
    id 'io.spring.dependency-management' version '1.1.4'
    id 'java'
}
group = 'com.example'
version = '0.0.1-SNAPSHOT'
`
	if err := os.WriteFile(filepath.Join(root, "build.gradle"), []byte(buildGradle), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "src", "main", "resources"), 0755)
	if err := os.WriteFile(filepath.Join(root, "src", "main", "resources", "application.properties"), []byte("server.port=8080\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "src", "main", "java", "com", "example", "demo"), 0755)
	if err := os.WriteFile(filepath.Join(root, "src", "main", "java", "com", "example", "demo", "DemoApp.java"), []byte("package com.example.demo;\npublic class DemoApp {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Java" {
		t.Fatalf("Language = %q, want 'Java'", model.Language)
	}
	if model.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", model.Port)
	}
}

func TestRegression_RailsRuby(t *testing.T) {
	root := t.TempDir()
	gemfile := `source 'https://rubygems.org'
gem 'rails', '~> 7.1'
gem 'puma', '>= 5.0'
`
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte(gemfile), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(root, "bin"), 0755)
	if err := os.WriteFile(filepath.Join(root, "bin", "rails"), []byte("#!/usr/bin/env ruby\n"), 0755); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Ruby" {
		t.Fatalf("Language = %q, want 'Ruby'", model.Language)
	}
	if model.RunCommand != "bin/rails server" {
		t.Fatalf("RunCommand = %q, want 'bin/rails server'", model.RunCommand)
	}
}

func TestRegression_PhoenixElixir(t *testing.T) {
	root := t.TempDir()
	mixContent := `defmodule MyApp.MixProject do
  use Mix.Project
  def project do
    [app: :my_app, version: "0.1.0"]
  end
  defp deps do
    [{:phoenix, "~> 1.7"}]
  end
end
`
	if err := os.WriteFile(filepath.Join(root, "mix.exs"), []byte(mixContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mix.lock"), []byte("%{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "Elixir" {
		t.Fatalf("Language = %q, want 'Elixir'", model.Language)
	}
	if model.RunCommand != "mix phx.server" {
		t.Fatalf("RunCommand = %q, want 'mix phx.server'", model.RunCommand)
	}
	if model.Port != 4000 {
		t.Fatalf("Port = %d, want 4000 (Phoenix default)", model.Port)
	}
}

func TestRegression_LaravelPHP(t *testing.T) {
	root := t.TempDir()
	composerJSON := `{
  "name": "laravel/laravel",
  "require": {
    "php": "^8.2",
    "laravel/framework": "^11.0"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "composer.json"), []byte(composerJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artisan"), []byte("#!/usr/bin/env php\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "composer.lock"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.Language != "PHP" {
		t.Fatalf("Language = %q, want 'PHP'", model.Language)
	}
	if model.RunCommand != "php artisan serve" {
		t.Fatalf("RunCommand = %q, want 'php artisan serve'", model.RunCommand)
	}
}

func TestRegression_RepoWithStrategyOverrides(t *testing.T) {
	root := t.TempDir()
	packageJSON := `{
  "name": "custom-service",
  "scripts": {
    "start": "node index.js"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	octoYAML := `spec: octo.dev/v1
components:
  custom-service:
    run_command: "node custom_entrypoint.js --port 8088"
    port: 8088
verification:
  - component: custom-service
    kind: http
    path: /health
    expected_status: 200
`
	if err := os.WriteFile(filepath.Join(root, ".octo.yaml"), []byte(octoYAML), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if model.RunCommand != "node custom_entrypoint.js --port 8088" {
		t.Fatalf("RunCommand = %q, want custom override", model.RunCommand)
	}
	if model.Port != 8088 {
		t.Fatalf("Port = %d, want 8088", model.Port)
	}
	if len(model.CustomVerification) != 1 || model.CustomVerification[0].Path != "/health" {
		t.Fatalf("unexpected custom verification: %+v", model.CustomVerification)
	}

	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	checks := BuildVerificationChecks(model, plan)

	hasHTTPCheck := false
	for _, c := range checks {
		if c.Kind == VerificationHTTP && c.Path == "/health" {
			hasHTTPCheck = true
			break
		}
	}
	if !hasHTTPCheck {
		t.Fatal("expected VerificationHTTP check in plan checks")
	}
}
