package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harshul/octo-cli/internal/benchmark"
)

func TestBenchmarkCommandSingleRepo(t *testing.T) {
	root := t.TempDir()
	packageJSON := `{
  "name": "benchmark-cli-test",
  "scripts": {
    "start": "node server.js"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	_ = benchmarkCmd.Flags().Set("suite", "false")
	_ = benchmarkCmd.Flags().Set("json", "false")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"benchmark", root, "--json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("benchmark command failed: %v", err)
	}

	var res benchmark.BenchmarkResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON benchmark output: %v\nOutput: %s", err, buf.String())
	}

	if !res.OctoScore.OneShotSuccess {
		t.Fatal("expected Octo OneShotSuccess to be true")
	}
	if res.OctoScore.HallucinationRate != 0.0 {
		t.Fatalf("expected 0.0 hallucination rate, got %v", res.OctoScore.HallucinationRate)
	}
}

func TestBenchmarkCommandSuiteJSON(t *testing.T) {
	_ = benchmarkCmd.Flags().Set("suite", "false")
	_ = benchmarkCmd.Flags().Set("json", "false")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"benchmark", "--suite", "--json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("benchmark --suite --json failed: %v", err)
	}

	var report benchmark.BenchmarkSuiteReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse benchmark suite JSON: %v\nOutput: %s", err, buf.String())
	}

	if report.TotalCases < 8 {
		t.Fatalf("expected >= 8 test cases, got %d", report.TotalCases)
	}
	if report.OctoPassRate != 100.0 {
		t.Fatalf("expected 100.0%% pass rate, got %v", report.OctoPassRate)
	}
	if !report.ZeroHallucinationConfirmed {
		t.Fatal("expected zero hallucination confirmed")
	}
}

func TestBenchmarkCommandHumanOutput(t *testing.T) {
	_ = benchmarkCmd.Flags().Set("suite", "false")
	_ = benchmarkCmd.Flags().Set("json", "false")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"benchmark", "--suite"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("benchmark --suite human output failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Autonomous AI Agent Benchmark") {
		t.Fatalf("expected suite name in output, got:\n%s", output)
	}
	if !strings.Contains(output, "100.0%") {
		t.Fatalf("expected 100.0%% pass rate in output, got:\n%s", output)
	}
}
