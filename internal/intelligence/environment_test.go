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
	write(".env.example", "DATABASE_URL=postgres://example\nPUBLIC_API_URL=http://localhost:3000\n")

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
