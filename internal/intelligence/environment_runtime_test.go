package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveEnvironmentPrecedence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("API_KEY=file\nDATABASE_URL=db-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.local"), []byte("API_KEY=local\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("API_KEY", "shell")

	env, err := ResolveEnvironment(root, EnvironmentModel{Variables: []EnvironmentVariable{
		{Name: "API_KEY", Required: true},
		{Name: "DATABASE_URL", Required: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if env.Values["API_KEY"] != "shell" {
		t.Fatalf("API_KEY=%q, want shell precedence", env.Values["API_KEY"])
	}
	if env.Values["DATABASE_URL"] != "db-file" {
		t.Fatalf("DATABASE_URL=%q, want .env value", env.Values["DATABASE_URL"])
	}
}

func TestResolveEnvironmentReportsMissingRequiredVariables(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveEnvironment(root, EnvironmentModel{Variables: []EnvironmentVariable{
		{Name: "DATABASE_URL", Required: true},
	}})
	if err == nil {
		t.Fatal("expected missing environment variable error")
	}
}

func TestResolveEnvironmentIgnoresUndeclaredValues(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("DECLARED=yes\nUNDECLARED=secret\n"), 0600); err != nil {
		t.Fatal(err)
	}

	env, err := ResolveEnvironment(root, EnvironmentModel{Variables: []EnvironmentVariable{
		{Name: "DECLARED", Required: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Values["UNDECLARED"]; ok {
		t.Fatal("resolver must not expose undeclared environment values")
	}
}
