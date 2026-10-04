package intelligence

import (
	"context"
	"testing"
)

func TestBuildProvisioningRequirementsDeduplicatesManagers(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", PackageManager: "pnpm"},
			{Name: "admin", PackageManager: "pnpm"},
			{Name: "api", PackageManager: "npm"},
		},
	}
	requirements := BuildProvisioningRequirements(model)
	if len(requirements) != 2 {
		t.Fatalf("requirements=%+v", requirements)
	}
}

func TestPlannerAddsProvisioningBeforeInstall(t *testing.T) {
	model := ProjectModel{
		Name: "demo",
		Root: t.TempDir(),
		Components: []Component{{
			Name: "demo", Path: ".", PackageManager: "pnpm", RunCommand: "pnpm dev",
		}},
	}
	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	var provision, install *ExecutionStep
	for i := range plan.Steps {
		switch plan.Steps[i].Phase {
		case PhaseProvision:
			provision = &plan.Steps[i]
		case PhaseInstall:
			install = &plan.Steps[i]
		}
	}
	if provision == nil || provision.Command != "command -v pnpm" {
		t.Fatalf("provision=%+v", provision)
	}
	if install == nil || len(install.DependsOn) != 1 || install.DependsOn[0] != provision.ID {
		t.Fatalf("install=%+v provision=%+v", install, provision)
	}
}
