package intelligence

import (
	"os"
	"strings"
	"testing"
)

func TestEnvironmentTemplateGeneration(t *testing.T) {
	model := ProjectModel{
		Name: "test-app",
		Port: 8080,
		Environment: EnvironmentModel{
			Variables: []EnvironmentVariable{
				{
					Name:     "DATABASE_URL",
					Required: true,
					Sources:  []string{"docker-compose.yml"},
				},
				{
					Name:     "PORT",
					Required: true,
					Sources:  []string{"server.js"},
				},
				{
					Name:     "OPTIONAL_FEATURE_FLAG",
					Required: false,
					Sources:  []string{"config.js"},
				},
			},
		},
	}

	tpl := GenerateEnvTemplate("/test", model)
	if !strings.Contains(tpl, "DATABASE_URL=postgresql://") {
		t.Fatalf("expected template to include DATABASE_URL with postgres placeholder: %s", tpl)
	}
	if !strings.Contains(tpl, "PORT=8080") {
		t.Fatalf("expected template to include PORT=8080: %s", tpl)
	}
	if !strings.Contains(tpl, "OPTIONAL_FEATURE_FLAG") {
		t.Fatalf("expected template to include OPTIONAL_FEATURE_FLAG: %s", tpl)
	}
}

func TestInspectEnvironmentStatus(t *testing.T) {
	tempDir := t.TempDir()
	model := ProjectModel{
		Name: "test-app",
		Environment: EnvironmentModel{
			Variables: []EnvironmentVariable{
				{
					Name:     "DEFINED_VAR",
					Required: true,
				},
				{
					Name:     "MISSING_REQ",
					Required: true,
				},
				{
					Name:     "MISSING_OPT",
					Required: false,
				},
			},
		},
	}

	// Write DEFINED_VAR to tempDir/.env
	envPath := tempDir + "/.env"
	if err := os.WriteFile(envPath, []byte("DEFINED_VAR=test_value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	status := InspectEnvironmentStatus(tempDir, model)
	if len(status.Present) != 1 || status.Present[0] != "DEFINED_VAR" {
		t.Fatalf("expected DEFINED_VAR in present: %#v", status.Present)
	}
	if len(status.MissingRequired) != 1 || status.MissingRequired[0] != "MISSING_REQ" {
		t.Fatalf("expected MISSING_REQ in missing required: %#v", status.MissingRequired)
	}
	if len(status.MissingOptional) != 1 || status.MissingOptional[0] != "MISSING_OPT" {
		t.Fatalf("expected MISSING_OPT in missing optional: %#v", status.MissingOptional)
	}
}
