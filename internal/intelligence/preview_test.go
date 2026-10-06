package intelligence

import (
	"context"
	"testing"
)

func TestBuildMachinePreview(t *testing.T) {
	model := ProjectModel{
		Name:     "web-api",
		Root:     "/mock/repo",
		Language: "Node",
		Port:     3000,
		Services: []Service{
			{
				Name:  "db",
				Ports: []string{"5432:5432"},
			},
		},
	}

	plan := ExecutionPlan{
		ProjectName: "web-api",
		Root:        "/mock/repo",
		Steps: []ExecutionStep{
			{
				ID:        "component.web-api.install",
				Phase:     PhaseInstall,
				Command:   "npm install",
				WorkDir:   "/mock/repo",
				Component: "web-api",
			},
			{
				ID:          "component.web-api.start",
				Phase:       PhaseStart,
				Command:     "npm start",
				WorkDir:     "/mock/repo",
				Component:   "web-api",
				LongRunning: true,
			},
		},
		Ports: []PortAssignment{
			{
				Component: "web-api",
				Resolved:  3000,
			},
		},
	}

	preview := BuildMachinePreview(context.Background(), model, plan)

	if preview.ProjectName != "web-api" {
		t.Fatalf("expected project name web-api, got %q", preview.ProjectName)
	}

	// Verify filesystem mutations captured install & lockfile
	if len(preview.Mutations) < 2 {
		t.Fatalf("expected at least 2 mutation previews, got %d", len(preview.Mutations))
	}

	// Verify network impact captured app port and db port
	if len(preview.Network) < 2 {
		t.Fatalf("expected at least 2 network previews, got %d", len(preview.Network))
	}

	// Verify processes captured install and start
	if len(preview.Processes) != 2 {
		t.Fatalf("expected 2 process previews, got %d", len(preview.Processes))
	}
}
