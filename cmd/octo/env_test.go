package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvCommandJSON(t *testing.T) {
	tempDir := t.TempDir()
	packageJSON := `{
		"name": "env-test",
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
	rootCmd.SetArgs([]string{"env", tempDir, "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing env command: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"total_variables"`) {
		t.Fatalf("expected JSON output to contain 'total_variables', got: %s", out)
	}
}

func TestEnvTemplateCommand(t *testing.T) {
	tempDir := t.TempDir()
	packageJSON := `{
		"name": "template-test",
		"version": "1.0.0"
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"env", "template", tempDir})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing env template command: %v", err)
	}
}
