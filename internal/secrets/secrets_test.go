package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanForEnvVarsIgnoresCIDirectoriesAndTestFiles(t *testing.T) {
	root := t.TempDir()

	// 1. Production code with genuine runtime env vars
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(`
package main
import "os"
func main() {
    port := os.Getenv("PORT")
    token := os.Getenv("APP_API_TOKEN")
    _ = port
    _ = token
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Unit test file with mock env vars
	if err := os.WriteFile(filepath.Join(root, "main_test.go"), []byte(`
package main
import "os"
func TestMain() {
    mock := os.Getenv("TEST_MOCK_SECRET")
    _ = mock
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. GitHub Actions workflow with CI runner env vars
	ghDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(ghDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ghDir, "ci.yml"), []byte(`
name: CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo $CI_RUNNER_TOKEN
`), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Scan
	vars, err := ScanForEnvVars(root, "go")
	if err != nil {
		t.Fatalf("ScanForEnvVars failed: %v", err)
	}

	varMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v.Name] = true
	}

	// APP_API_TOKEN should be detected
	if !varMap["APP_API_TOKEN"] {
		t.Errorf("expected APP_API_TOKEN from main.go to be detected, found: %v", vars)
	}

	// TEST_MOCK_SECRET from main_test.go must NOT be detected
	if varMap["TEST_MOCK_SECRET"] {
		t.Errorf("expected TEST_MOCK_SECRET from main_test.go to be ignored, but was detected")
	}

	// CI_RUNNER_TOKEN from .github/workflows must NOT be detected
	if varMap["CI_RUNNER_TOKEN"] {
		t.Errorf("expected CI_RUNNER_TOKEN from .github/workflows to be ignored, but was detected")
	}
}

func TestIsTestFile(t *testing.T) {
	testCases := []struct {
		path   string
		isTest bool
	}{
		{"main.go", false},
		{"main_test.go", true},
		{"server.js", false},
		{"server.test.js", true},
		{"server.spec.ts", true},
		{"models.py", false},
		{"test_models.py", true},
		{"models_test.py", true},
		{"app.rb", false},
		{"app_spec.rb", true},
		{"Service.java", false},
		{"ServiceTest.java", true},
		{"Controller.kt", false},
		{"ControllerTests.kt", true},
	}

	for _, tc := range testCases {
		if got := isTestFile(tc.path); got != tc.isTest {
			t.Errorf("isTestFile(%q) = %v, want %v", tc.path, got, tc.isTest)
		}
	}
}
