package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverComposeHealthCheck(t *testing.T) {
	root := t.TempDir()
	compose := `services:
  postgres:
    image: postgres:17
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 5
`
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	services, _, found, err := discoverComposeServices(root)
	if err != nil {
		t.Fatal(err)
	}
	if !found || len(services) != 1 {
		t.Fatalf("found=%v services=%d", found, len(services))
	}
	check := services[0].HealthCheck
	if check == nil {
		t.Fatal("expected health check")
	}
	if check.Command != "pg_isready -U postgres" {
		t.Fatalf("command=%q", check.Command)
	}
	if check.Interval != "5s" || check.Timeout != "3s" || check.Retries != 5 {
		t.Fatalf("health check=%+v", check)
	}
}

func TestPlannerGatesDependentsOnHealth(t *testing.T) {
	model := ProjectModel{
		Name: "stack",
		Root: t.TempDir(),
		Components: []Component{{
			Name: "api", Path: "api", RunCommand: "echo api", DependsOn: []string{"postgres"},
		}},
		Services: []Service{{
			Name: "postgres",
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: "compose.yaml"}},
			HealthCheck: &HealthCheck{Command: "pg_isready -U postgres"},
		}},
	}
	plan, err := (DeterministicPlanner{}).Plan(nilContext{}, model)
	if err != nil {
		t.Fatal(err)
	}
	health := findExecutionStep(plan, "service.postgres.health")
	if health == nil {
		t.Fatal("missing postgres health step")
	}
	if health.Command != "docker compose -f compose.yaml exec -T postgres sh -c 'pg_isready -U postgres'" {
		t.Fatalf("health command=%q", health.Command)
	}
	api := findExecutionStep(plan, "component.api.start")
	if api == nil || !contains(api.DependsOn, "service.postgres.health") {
		t.Fatalf("api dependencies=%v", api)
	}
}

type nilContext struct{}
func (nilContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nilContext) Done() <-chan struct{} { return nil }
func (nilContext) Err() error { return nil }
func (nilContext) Value(any) any { return nil }
