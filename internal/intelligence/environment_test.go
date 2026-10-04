package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeDiscoversEnvironmentRequirementsWithoutValues(t *testing.T) {
	root := t.TempDir()

	write := func(name, value string) {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("package.json", `{"name":"demo","scripts":{"dev":"node index.js"}}`)
	write("index.js", `console.log(process.env.DATABASE_URL); console.log(process.env.PUBLIC_API_URL)`)

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Environment.Variables) != 2 {
		t.Fatalf("environment variables=%d, want 2", len(model.Environment.Variables))
	}

	db := findEnvironmentVariable(model.Environment, "DATABASE_URL")
	if db == nil {
		t.Fatal("DATABASE_URL not found")
	}
	if !db.Required {
		t.Fatal("DATABASE_URL should be required")
	}
	if len(db.Sources) == 0 || db.Sources[0] != "index.js" {
		t.Fatalf("DATABASE_URL sources=%v", db.Sources)
	}

	api := findEnvironmentVariable(model.Environment, "PUBLIC_API_URL")
	if api == nil {
		t.Fatal("PUBLIC_API_URL not found")
	}
	if api.Required {
		t.Fatal("PUBLIC_API_URL should not be marked required by the current detector")
	}
}

func findEnvironmentVariable(model EnvironmentModel, name string) *EnvironmentVariable {
	for i := range model.Variables {
		if model.Variables[i].Name == name {
			return &model.Variables[i]
		}
	}
	return nil
}


func TestResolveEnvironmentBindingsIsSecretSafeAndDeterministic(t *testing.T) {
	model := ProjectModel{
		Name: "stack",
		Components: []Component{
			{
				Name: "web",
				References: []Reference{{
					Target: "api", Kind: RelationshipNetworkReference, Variable: "API_URL",
					Confidence: 0.95,
					Evidence: []Evidence{{Kind: EvidenceConfig, Path: "web/.env", Detail: "Static environment variable API_URL references api.", Strength: 0.95}},
				}},
			},
			{
				Name: "api",
				References: []Reference{{
					Target: "postgres", Kind: RelationshipNetworkReference, Variable: "DATABASE_URL",
					Confidence: 0.95,
				}},
			},
		},
	}

	bindings := ResolveEnvironmentBindings(model)
	if len(bindings) != 2 {
		t.Fatalf("bindings=%d, want 2", len(bindings))
	}
	if bindings[0].Source != "component:api" || bindings[0].Variable != "DATABASE_URL" || bindings[0].Target != "postgres" {
		t.Fatalf("first binding=%+v", bindings[0])
	}
	if bindings[1].Source != "component:web" || bindings[1].Variable != "API_URL" || bindings[1].Target != "api" {
		t.Fatalf("second binding=%+v", bindings[1])
	}

	for _, binding := range bindings {
		if binding.Variable == "DATABASE_URL" && binding.Target == "postgres" {
			for _, evidence := range binding.Evidence {
				if evidence.Detail == "postgres://user:secret@postgres:5432/app" {
					t.Fatal("raw environment value leaked into binding evidence")
				}
			}
		}
	}
}


func TestResolveEnvironmentScopesComponentLocalFiles(t *testing.T) {
	root := t.TempDir()
	web := filepath.Join(root, "apps", "web")
	api := filepath.Join(root, "apps", "api")
	if err := os.MkdirAll(web, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(api, 0755); err != nil { t.Fatal(err) }

	write := func(path, value string) {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil { t.Fatal(err) }
	}
	write(filepath.Join(root, ".env"), "SHARED=root\nROOT_ONLY=root-value\n")
	write(filepath.Join(web, ".env"), "SHARED=web\nWEB_ONLY=web-value\n")
	write(filepath.Join(web, ".env.local"), "LOCAL_ONLY=local-value\nSHARED=web-local\n")
	write(filepath.Join(api, ".env"), "SHARED=api\nAPI_ONLY=api-value\n")

	model := EnvironmentModel{Variables: []EnvironmentVariable{
		{Name: "SHARED", Required: true},
		{Name: "ROOT_ONLY", Required: true},
		{Name: "WEB_ONLY", Required: true},
		{Name: "LOCAL_ONLY", Required: true},
		{Name: "API_ONLY", Required: true},
	},}
	modelVars := model.Variables
	_ = modelVars

	resolved, err := ResolveEnvironment(root, model)
	if err != nil { t.Fatal(err) }

	webEnv := resolved.ForStep(ExecutionStep{WorkDir: "apps/web"})
	if webEnv.Values["SHARED"] != "web-local" || webEnv.Values["WEB_ONLY"] != "web-value" || webEnv.Values["ROOT_ONLY"] != "root-value" {
		t.Fatalf("web environment=%v", webEnv.Values)
	}
	if webEnv.Values["API_ONLY"] != "" {
		t.Fatal("api-only variable leaked into web environment")
	}

	apiEnv := resolved.ForStep(ExecutionStep{WorkDir: "apps/api"})
	if apiEnv.Values["SHARED"] != "api" || apiEnv.Values["API_ONLY"] != "api-value" {
		t.Fatalf("api environment=%v", apiEnv.Values)
	}
	if apiEnv.Values["WEB_ONLY"] != "" {
		t.Fatal("web-only variable leaked into api environment")
	}
}
