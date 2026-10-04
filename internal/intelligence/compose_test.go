package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverComposeServices(t *testing.T) {
	root := t.TempDir()
	compose := `services:
  postgres:
    image: postgres:17
    ports:
      - "5432:5432"
  api:
    build:
      context: ./api
    depends_on:
      - postgres
    ports:
      - "8080:8080"
  worker:
    image: demo/worker:latest
    depends_on:
      postgres:
        condition: service_healthy
`
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(compose), 0644); err != nil {
		t.Fatal(err)
	}

	services, evidence, found, err := discoverComposeServices(root)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected compose topology")
	}
	if len(services) != 3 {
		t.Fatalf("services=%d, want 3", len(services))
	}
	if len(evidence) == 0 {
		t.Fatal("expected compose evidence")
	}

	api := findService(services, "api")
	if api == nil {
		t.Fatal("api service not found")
	}
	if api.Build != "./api" {
		t.Fatalf("api build=%q", api.Build)
	}
	if len(api.DependsOn) != 1 || api.DependsOn[0] != "postgres" {
		t.Fatalf("api dependencies=%v", api.DependsOn)
	}
	if len(api.Ports) != 1 || api.Ports[0] != "8080:8080" {
		t.Fatalf("api ports=%v", api.Ports)
	}

	worker := findService(services, "worker")
	if worker == nil || len(worker.DependsOn) != 1 || worker.DependsOn[0] != "postgres" {
		t.Fatalf("worker dependencies=%v", worker)
	}
}

func TestAnalyzeIncludesComposeServices(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte(`services:
  db:
    image: postgres:17
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# demo"), 0644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Services) != 1 || model.Services[0].Name != "db" {
		t.Fatalf("services=%+v", model.Services)
	}
}

func findService(services []Service, name string) *Service {
	for i := range services {
		if services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}
