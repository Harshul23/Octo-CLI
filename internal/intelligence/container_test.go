package intelligence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveContainerImage(t *testing.T) {
	tests := []struct {
		language string
		version  string
		expected string
	}{
		{"Node", "", "node:20-alpine"},
		{"node", "18.17.0", "node:18-alpine"},
		{"TypeScript", "^22.0.0", "node:22-alpine"},
		{"Go", "", "golang:1.24-alpine"},
		{"go", "go1.23.1", "golang:1.23.1-alpine"},
		{"Python", "", "python:3.12-slim"},
		{"python", "3.11", "python:3.11-slim"},
		{"Rust", "", "rust:alpine"},
		{"Java", "", "eclipse-temurin:21-jdk-alpine"},
		{"java", "17", "eclipse-temurin:17-jdk-alpine"},
		{"Ruby", "", "ruby:3.3-alpine"},
		{"ruby", "3.2.2", "ruby:3-alpine"},
		{"Unknown", "", "alpine:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.language+"_"+tt.version, func(t *testing.T) {
			got := ResolveContainerImage(tt.language, tt.version)
			if got != tt.expected {
				t.Fatalf("ResolveContainerImage(%q, %q) = %q, want %q", tt.language, tt.version, got, tt.expected)
			}
		})
	}
}

func TestContainerAdapterBuildRunArgs(t *testing.T) {
	root := t.TempDir()
	adapter := NewContainerAdapter(root)

	step := ExecutionStep{
		ID:        "component.web.start",
		Component: "web",
		Phase:     PhaseStart,
		Command:   "npm start",
		WorkDir:   "apps/web",
		Environment: map[string]string{
			"PORT": "3000",
			"ENV":  "test",
		},
	}
	env := ResolvedEnvironment{
		Values: map[string]string{
			"DATABASE_URL": "postgres://localhost/db",
		},
	}

	args, err := adapter.BuildRunArgs(step, env, true, "octo-test-container")
	if err != nil {
		t.Fatal(err)
	}

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "run --rm -d --name octo-test-container") {
		t.Fatalf("expected run --rm -d --name in args, got: %s", argsStr)
	}

	absRoot, _ := filepath.Abs(root)
	if !strings.Contains(argsStr, "-v "+absRoot+":/app") {
		t.Fatalf("expected volume mount in args, got: %s", argsStr)
	}

	if !strings.Contains(argsStr, "-w /app/apps/web") {
		t.Fatalf("expected working directory in args, got: %s", argsStr)
	}

	if !strings.Contains(argsStr, "-p 3000:3000") {
		t.Fatalf("expected port mapping in args, got: %s", argsStr)
	}

	if !strings.Contains(argsStr, "-e PORT=3000") || !strings.Contains(argsStr, "-e ENV=test") || !strings.Contains(argsStr, "-e DATABASE_URL=postgres://localhost/db") {
		t.Fatalf("expected environment variables in args, got: %s", argsStr)
	}

	if !strings.Contains(argsStr, "sh -c npm start") {
		t.Fatalf("expected command in args, got: %s", argsStr)
	}
}

func TestRuntimeResolverSandboxMode(t *testing.T) {
	root := t.TempDir()

	// Default resolver (sandbox = false)
	defaultResolver := NewRuntimeResolver()
	step := ExecutionStep{ID: "component.web.start", Command: "npm start"}
	adapter, err := defaultResolver.Resolve(step)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "shell" {
		t.Fatalf("expected shell adapter by default, got %q", adapter.Name())
	}

	// Sandbox resolver (sandbox = true)
	sandboxResolver := NewRuntimeResolverWithOptions(RuntimeResolverOptions{
		Sandbox: true,
		Root:    root,
	})
	sandboxedAdapter, err := sandboxResolver.Resolve(step)
	if err != nil {
		t.Fatal(err)
	}
	if sandboxedAdapter.Name() != "container" {
		t.Fatalf("expected container adapter in sandbox mode, got %q", sandboxedAdapter.Name())
	}

	// Compose steps still resolve to ComposeAdapter even in sandbox mode
	composeStep := ExecutionStep{ID: "service.db.start", Command: "docker compose up -d db"}
	composeAdapter, err := sandboxResolver.Resolve(composeStep)
	if err != nil {
		t.Fatal(err)
	}
	if composeAdapter.Name() != "compose" {
		t.Fatalf("expected compose adapter for docker compose command, got %q", composeAdapter.Name())
	}
}

func TestExecutePlanReportWithSandboxOption(t *testing.T) {
	t.Run("detached long-running container process", func(t *testing.T) {
		mockAdapter := &mockSandboxedAdapter{}
		plan := ExecutionPlan{
			ProjectName: "demo",
			Steps: []ExecutionStep{{
				ID: "component.web.start", Component: "web", Phase: PhaseStart,
				Command: "run-server", LongRunning: true,
			}},
		}
		report := ExecutePlanReportWithOptions(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
			adapters: []RuntimeAdapter{mockAdapter},
		}, ResolvedEnvironment{Values: map[string]string{}}, ExecutionOptions{Sandbox: true, Detach: true})

		if !report.Success {
			t.Fatalf("expected success, got %v", report.FailureReason)
		}
		if !mockAdapter.executed {
			t.Fatal("expected mock sandboxed adapter to be invoked")
		}
	})

	t.Run("synchronous container step execution", func(t *testing.T) {
		mockAdapter := &mockSandboxedAdapter{}
		plan := ExecutionPlan{
			ProjectName: "demo",
			Steps: []ExecutionStep{{
				ID: "component.web.build", Component: "web", Phase: PhaseSetup,
				Command: "npm run build", LongRunning: false,
			}},
		}
		report := ExecutePlanReportWithOptions(context.Background(), ProjectModel{Name: "demo"}, plan, RuntimeResolver{
			adapters: []RuntimeAdapter{mockAdapter},
		}, ResolvedEnvironment{Values: map[string]string{}}, ExecutionOptions{Sandbox: true})

		if !report.Success {
			t.Fatalf("expected success, got %v", report.FailureReason)
		}
		if !mockAdapter.executed {
			t.Fatal("expected mock sandboxed adapter to be invoked")
		}
	})
}

type mockSandboxedAdapter struct {
	executed bool
}

func (m *mockSandboxedAdapter) Name() string { return "container" }
func (m *mockSandboxedAdapter) Supports(step ExecutionStep) bool { return step.Command != "" }
func (m *mockSandboxedAdapter) Execute(_ context.Context, _ ExecutionStep, _ ResolvedEnvironment) error {
	m.executed = true
	return nil
}
func (m *mockSandboxedAdapter) Start(_ context.Context, _ ExecutionStep, _ ResolvedEnvironment) (RunningProcess, error) {
	m.executed = true
	return &mockRunningProcess{pid: 999}, nil
}
