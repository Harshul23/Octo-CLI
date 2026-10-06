package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionCompatibilityRules(t *testing.T) {
	tests := []struct {
		required string
		current  string
		expected bool
	}{
		// Caret matches
		{"^18.0.0", "18.12.0", true},
		{"^18.0.0", "19.0.0", false},
		{"^18", "18.2.1", true},
		{"^3.9", "3.11.2", true},
		{"^3.9", "4.0.0", false},
		// Tilde matches
		{"~18.2.0", "18.2.5", true},
		{"~18.2.0", "18.3.0", false},
		// Gte matches
		{">=18.0.0", "20.10.0", true},
		{">=18.0.0", "16.14.0", false},
		// Wildcard
		{"20.x", "20.11.0", true},
		{"20.x", "21.0.0", false},
		// Exact / prefix
		{"20", "20.11.0", true},
		{"18", "20.11.0", false},
	}

	for _, tt := range tests {
		got := IsVersionCompatible(tt.required, tt.current)
		if got != tt.expected {
			t.Errorf("IsVersionCompatible(%q, %q) = %v; want %v", tt.required, tt.current, got, tt.expected)
		}
	}
}

func TestGoVersionCompatibility(t *testing.T) {
	if !IsGoVersionCompatible("1.22", "1.24.0") {
		t.Errorf("expected 1.24.0 to be compatible with go 1.22")
	}
	if IsGoVersionCompatible("1.25", "1.24.0") {
		t.Errorf("expected 1.24.0 to NOT be compatible with go 1.25")
	}
}

func TestDetectVirtualEnv(t *testing.T) {
	tempDir := t.TempDir()

	// No venv initially
	if got := DetectVirtualEnv(tempDir); got != "" {
		t.Errorf("expected empty venv, got %q", got)
	}

	// Create .venv with bin/python
	venvDir := filepath.Join(tempDir, ".venv", "bin")
	if err := os.MkdirAll(venvDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(venvDir, "python"), []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	got := DetectVirtualEnv(tempDir)
	if got != filepath.Join(tempDir, ".venv") {
		t.Errorf("expected %q, got %q", filepath.Join(tempDir, ".venv"), got)
	}
}

func TestCheckRuntimeCompatibilityCurrentHost(t *testing.T) {
	model := ProjectModel{
		Language:       "Go",
		RuntimeVersion: "1.20",
	}
	check := CheckRuntimeCompatibility(context.Background(), model)
	if !check.Compatible {
		t.Errorf("expected Go on current host to be compatible with 1.20: %v", check.Message)
	}
	if check.DetectedVersion == "" {
		t.Errorf("expected detected Go version to be non-empty")
	}
}
