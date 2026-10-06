package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCommandJSON(t *testing.T) {
	tempDir := t.TempDir()
	packageJSON := `{
		"name": "verify-test",
		"version": "1.0.0",
		"scripts": {
			"start": "node index.js"
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"verify", tempDir, "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing verify: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"project_name": "verify-test"`) {
		t.Fatalf("expected JSON output to contain project name 'verify-test', got: %s", out)
	}
}
