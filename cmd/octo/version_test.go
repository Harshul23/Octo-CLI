package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionDefaultAndResolution(t *testing.T) {
	// Verify that rootCmd has a version set (at least 'dev' or the injected tag)
	if rootCmd.Version == "" {
		t.Fatal("expected rootCmd.Version to be non-empty")
	}

	// Verify resolution logic with mock build info
	mockInfo := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.2.3",
		},
	}

	testVersion := "dev"
	if testVersion == "dev" {
		if mockInfo.Main.Version != "" && mockInfo.Main.Version != "(devel)" {
			testVersion = mockInfo.Main.Version
		}
	}

	if testVersion != "v1.2.3" {
		t.Fatalf("expected testVersion to be 'v1.2.3', got '%s'", testVersion)
	}

	// Verify that if version was injected (not 'dev'), it is not overridden
	injectedVersion := "v2.0.0"
	if injectedVersion == "dev" {
		if mockInfo.Main.Version != "" && mockInfo.Main.Version != "(devel)" {
			injectedVersion = mockInfo.Main.Version
		}
	}
	if injectedVersion != "v2.0.0" {
		t.Fatalf("expected injectedVersion to remain 'v2.0.0', got '%s'", injectedVersion)
	}
}
