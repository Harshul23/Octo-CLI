package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewCommandJSON(t *testing.T) {
	tempDir := t.TempDir()
	packageJSON := `{
		"name": "preview-test",
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
	rootCmd.SetArgs([]string{"preview", tempDir, "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing preview command: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"project_name": "preview-test"`) {
		t.Fatalf("expected JSON output to contain 'preview-test', got: %s", out)
	}
	if !strings.Contains(out, `"mutations"`) {
		t.Fatalf("expected JSON output to contain 'mutations', got: %s", out)
	}
}
