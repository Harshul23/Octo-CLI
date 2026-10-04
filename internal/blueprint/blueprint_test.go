package blueprint

import (
	"testing"

	"github.com/harshul/octo-cli/internal/intelligence"
)

func TestFromProjectModel(t *testing.T) {
	model := intelligence.ProjectModel{
		Name:           "demo",
		Root:           "/tmp/demo",
		Language:       "Node",
		RuntimeVersion: "22",
		PackageManager: "npm",
		RunCommand:     "npm run dev",
		SetupCommand:   "npm run build",
		Monorepo:       true,
	}

	bp := FromProjectModel(model)

	if bp.Name != model.Name || bp.Language != model.Language || bp.Version != model.RuntimeVersion {
		t.Fatalf("blueprint did not preserve project identity/runtime fields: %#v", bp)
	}
	if bp.SetupRequired != true || bp.IsMonorepo != true || bp.MonorepoRoot != model.Root {
		t.Fatalf("blueprint did not preserve native project state: %#v", bp)
	}
	if len(bp.Steps) != 3 {
		t.Fatalf("expected install, setup, and run steps; got %d", len(bp.Steps))
	}
	if bp.Steps[0].Command != "npm install" ||
		bp.Steps[1].Command != model.SetupCommand ||
		bp.Steps[2].Command != model.RunCommand {
		t.Fatalf("unexpected generated steps: %#v", bp.Steps)
	}
}
