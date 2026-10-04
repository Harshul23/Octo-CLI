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

func TestNativeProjectDetectorMonorepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"workspaces\":[\"apps/*\"]}"), 0644); err != nil { t.Fatal(err) }

	got, err := (NativeProjectDetector{}).Detect(root)
	if err != nil { t.Fatal(err) }
	if !got.IsMonorepo { t.Fatal("expected monorepo detection") }
	if got.MonorepoRoot != root { t.Fatalf("monorepo root = %q", got.MonorepoRoot) }
}
