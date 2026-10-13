package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/benchmark"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestBenchmarkCommandSingleRepo(t *testing.T) {
	root := t.TempDir()
	port := getFreePort(t)
	packageJSON := `{
  "name": "benchmark-cli-test",
  "scripts": {
    "start": "node server.js"
  }
}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	serverCode := fmt.Sprintf(`const http = require('http');
const port = process.env.PORT || %d;
const server = http.createServer((req, res) => { res.writeHead(200); res.end('OK'); });
server.listen(port, '0.0.0.0', () => {});
process.on('SIGTERM', () => process.exit(0));
process.on('SIGINT', () => process.exit(0));`, port)
	if err := os.WriteFile(filepath.Join(root, "server.js"), []byte(serverCode), 0644); err != nil {
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

	if report.TotalCases < 5 {
		t.Fatalf("expected >= 5 test cases, got %d", report.TotalCases)
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
