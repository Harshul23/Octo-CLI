package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAndCandidateDotnet(t *testing.T) {
	tmp := t.TempDir()
	csproj := filepath.Join(tmp, "MyWeb.csproj")
	if err := os.WriteFile(csproj, []byte("<Project Sdk=\"Microsoft.NET.Sdk.Web\"></Project>"), 0644); err != nil {
		t.Fatal(err)
	}

	detected, err := detectProject(tmp)
	if err != nil {
		t.Fatalf("detectProject failed: %v", err)
	}
	if detected.Language != "C#" {
		t.Fatalf("expected language C#, got %q", detected.Language)
	}

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if model.Language != "C#" {
		t.Fatalf("model language = %q, want C#", model.Language)
	}

	planner := DeterministicPlanner{DecisionProvider: DeterministicDecisionProvider{}}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	foundRun := false
	for _, step := range plan.Steps {
		if step.Phase == PhaseStart && (step.Command == "dotnet run --project MyWeb.csproj" || step.Command == "dotnet run") {
			foundRun = true
			break
		}
	}
	if !foundRun {
		t.Fatalf("expected dotnet run step in plan, got: %+v", plan.Steps)
	}
}

func TestDetectAndCandidateSwift(t *testing.T) {
	tmp := t.TempDir()
	pkgSwift := filepath.Join(tmp, "Package.swift")
	if err := os.WriteFile(pkgSwift, []byte("// swift-tools-version:5.9\nimport PackageDescription\nlet package = Package(name: \"MyCli\")"), 0644); err != nil {
		t.Fatal(err)
	}

	detected, err := detectProject(tmp)
	if err != nil {
		t.Fatalf("detectProject failed: %v", err)
	}
	if detected.Language != "Swift" {
		t.Fatalf("expected language Swift, got %q", detected.Language)
	}

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if model.Language != "Swift" {
		t.Fatalf("model language = %q, want Swift", model.Language)
	}

	planner := DeterministicPlanner{DecisionProvider: DeterministicDecisionProvider{}}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	foundRun := false
	for _, step := range plan.Steps {
		if step.Phase == PhaseStart && step.Command == "swift run" {
			foundRun = true
			break
		}
	}
	if !foundRun {
		t.Fatalf("expected swift run step in plan, got: %+v", plan.Steps)
	}
}

func TestDetectAndCandidateKotlin(t *testing.T) {
	tmp := t.TempDir()
	buildGradle := filepath.Join(tmp, "build.gradle.kts")
	content := `plugins {
    id("org.springframework.boot") version "3.2.0"
    kotlin("jvm") version "1.9.20"
}`
	if err := os.WriteFile(buildGradle, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// fake gradlew wrapper
	if err := os.WriteFile(filepath.Join(tmp, "gradlew"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	detected, err := detectProject(tmp)
	if err != nil {
		t.Fatalf("detectProject failed: %v", err)
	}
	if detected.Language != "Kotlin" {
		t.Fatalf("expected language Kotlin, got %q", detected.Language)
	}

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if model.Language != "Kotlin" {
		t.Fatalf("model language = %q, want Kotlin", model.Language)
	}

	planner := DeterministicPlanner{DecisionProvider: DeterministicDecisionProvider{}}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	foundRun := false
	for _, step := range plan.Steps {
		if step.Phase == PhaseStart && (step.Command == "./gradlew bootRun" || step.Command == "./gradlew run") {
			foundRun = true
			break
		}
	}
	if !foundRun {
		t.Fatalf("expected ./gradlew bootRun in plan, got: %+v", plan.Steps)
	}
}
