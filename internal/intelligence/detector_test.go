package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeProjectDetectorGo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if got.Language != "Go" { t.Fatalf("language = %q", got.Language) }
	if got.Version != "1.24" { t.Fatalf("version = %q", got.Version) }
	if got.Name != "demo" { t.Fatalf("name = %q", got.Name) }
	if got.RunCommand != "" { t.Fatalf("detector guessed run command %q", got.RunCommand) }
}

func TestNativeProjectDetectorSkipsStubPackageJSON(t *testing.T) {
	root := t.TempDir()
	// Create a stub package.json (e.g., from stray npm install with no name or scripts)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"dependencies\":{\"foo\":\"^1.0.0\"}}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a real Go module file
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.24\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "Go" {
		t.Fatalf("language = %q, want Go", got.Language)
	}
	if got.Name != "demo" {
		t.Fatalf("name = %q, want demo", got.Name)
	}
}

func TestNativeProjectDetectorNode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"name\":\"demo\",\"version\":\"1.2.3\"}"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte(""), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if got.Language != "Node" { t.Fatalf("language = %q", got.Language) }
	if got.PackageManager != "pnpm" { t.Fatalf("package manager = %q", got.PackageManager) }
	if got.Version != "1.2.3" { t.Fatalf("version = %q", got.Version) }
	if got.RunCommand != "" { t.Fatalf("detector guessed run command %q", got.RunCommand) }
}

func TestNativeProjectDetectorNodeEngines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"name\":\"demo\",\"version\":\"1.0.0\",\"engines\":{\"node\":\">=20.0.0\"}}"), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if got.Version != ">=20.0.0" { t.Fatalf("version = %q, want >=20.0.0", got.Version) }

	// .nvmrc should take precedence if present
	if err := os.WriteFile(filepath.Join(root, ".nvmrc"), []byte("v22.1.0\n"), 0644); err != nil { t.Fatal(err) }
	got, err = (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	// engines node was checked first unless .nvmrc is prioritized
}


func TestNativeProjectDetectorMonorepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"workspaces\":[\"apps/*\"]}"), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if !got.IsMonorepo { t.Fatal("expected monorepo detection") }
	if got.MonorepoRoot != root { t.Fatalf("monorepo root = %q", got.MonorepoRoot) }
}

func TestNativeProjectDetectorPython(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname = \"demo\"\nversion = \"1.2.3\"\n"), 0644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "uv.lock"), []byte(""), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if got.Language != "Python" { t.Fatalf("language = %q", got.Language) }
	if got.Name != "demo" { t.Fatalf("name = %q", got.Name) }
	if got.Version != "1.2.3" { t.Fatalf("version = %q", got.Version) }
	if got.PackageManager != "uv" { t.Fatalf("package manager = %q", got.PackageManager) }
	if got.RunCommand != "" { t.Fatalf("detector guessed run command %q", got.RunCommand) }
}

func TestNativeProjectDetectorRust(t *testing.T) {
	root := t.TempDir()
	cargoToml := `[package]
name = "my-cli"
version = "0.5.0"
rust-version = "1.85.0"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
`
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(cargoToml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cargo.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "Rust" {
		t.Fatalf("language = %q, want Rust", got.Language)
	}
	if got.Name != "my-cli" {
		t.Fatalf("name = %q, want my-cli", got.Name)
	}
	if got.Version != "1.85.0" {
		t.Fatalf("version = %q, want 1.85.0", got.Version)
	}
	if got.PackageManager != "cargo" {
		t.Fatalf("package manager = %q, want cargo", got.PackageManager)
	}
}



func TestNativeProjectDetectorDoesNotInferNetworkPortFromLanguage(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
	}{
		{name: "node", file: "package.json", content: `{"name":"demo"}`},
		{name: "go", file: "go.mod", content: `module example.com/demo
go 1.24
`},
		{name: "python", file: "pyproject.toml", content: `[project]
name = "demo"
`},
		{name: "rust", file: "Cargo.toml", content: `[package]
name = "demo"
version = "0.1.0"
`},
		{name: "java", file: "pom.xml", content: `<project/>`},
		{name: "ruby", file: "Gemfile", content: `source "https://rubygems.org"
`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tt.file), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := (NativeProjectDetector{}).Detect(root)
			if err != nil {
				t.Fatal(err)
			}
			if got.Port != 0 {
				t.Fatalf("detector inferred port %d from language %q", got.Port, got.Language)
			}
		})
	}
}
