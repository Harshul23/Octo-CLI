package benchmark

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func TestEvaluateRepository(t *testing.T) {
	root := t.TempDir()
	port := getFreePort()
	packageJSON := fmt.Sprintf(`{
  "name": "benchmark-demo",
  "scripts": {
    "start": "node index.js -p %d"
  }
}`, port)
	serverJS := fmt.Sprintf(`const http = require('http');
const port = process.env.PORT || %d;
const server = http.createServer((req, res) => { res.writeHead(200); res.end('OK'); });
server.listen(port, '0.0.0.0', () => {});
process.on('SIGTERM', () => process.exit(0));
process.on('SIGINT', () => process.exit(0));`, port)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte(serverJS), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EvaluateRepository(context.Background(), root, "Node.js Single App")
	if err != nil {
		t.Fatalf("EvaluateRepository failed: %v", err)
	}

	if result.Repository != filepath.Base(root) {
		t.Fatalf("Repository = %q, want %q", result.Repository, filepath.Base(root))
	}
	t.Logf("OctoScore: %+v", result.OctoScore)
	if !result.OctoScore.OneShotSuccess {
		t.Fatalf("expected Octo OneShotSuccess to be true; diagnostics: %+v", result.Diagnostics)
	}
	if result.OctoScore.HallucinationRate != 0.0 {
		t.Fatalf("Octo HallucinationRate = %v, want 0.0", result.OctoScore.HallucinationRate)
	}
	if !result.OctoScore.VerifiedReadiness {
		t.Fatal("expected VerifiedReadiness to be true")
	}
	if result.Comparison.TokenReductionPct <= 0 {
		t.Fatalf("expected positive token reduction, got %v%%", result.Comparison.TokenReductionPct)
	}
	assertBenchmarkReleasedResources(t, root, port)
}

// Check immediately, without retries: EvaluateRepository owns cleanup and must
// finish it before returning, including inherited child log-file handles.
func assertBenchmarkReleasedResources(t *testing.T, root string, port int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err == nil {
		conn.Close()
		t.Fatalf("benchmark child still serves port %d after return", port)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("benchmark still owns temporary files: %v", err)
	}
}

func TestRunBuiltinSuite(t *testing.T) {
	report, err := RunBuiltinSuite(context.Background())
	if err != nil {
		t.Fatalf("RunBuiltinSuite failed: %v", err)
	}

	if report.TotalCases < 5 {
		t.Fatalf("expected at least 5 test cases in suite, got %d", report.TotalCases)
	}
	if report.OctoPassRate != 100.0 {
		t.Fatalf("OctoPassRate = %v, want 100.0", report.OctoPassRate)
	}
	if !report.ZeroHallucinationConfirmed {
		t.Fatal("expected ZeroHallucinationConfirmed to be true")
	}
	if report.AvgTokenSavingsPct < 50.0 {
		t.Fatalf("expected AvgTokenSavingsPct >= 50%%, got %v%%", report.AvgTokenSavingsPct)
	}
	if len(report.Results) != report.TotalCases {
		t.Fatalf("Results count %d != TotalCases %d", len(report.Results), report.TotalCases)
	}
}

// A slow install that exceeds the start budget must still succeed, because
// installation runs under its own, larger budget.
func TestEvaluateRepositorySlowInstallOutlastsStartBudget(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not installed")
	}
	oldSetup, oldStart := SetupBudget, StartBudget
	SetupBudget, StartBudget = 60*time.Second, 5*time.Second
	t.Cleanup(func() { SetupBudget, StartBudget = oldSetup, oldStart })

	root := t.TempDir()
	port := getFreePort()
	packageJSON := fmt.Sprintf(`{
  "name": "slow-install-demo",
  "scripts": {
    "preinstall": "node -e \"setTimeout(function(){}, 6500)\"",
    "start": "node index.js -p %d"
  }
}`, port)
	serverJS := fmt.Sprintf(`const http = require('http');
const port = process.env.PORT || %d;
const server = http.createServer((req, res) => { res.writeHead(200); res.end('OK'); });
server.listen(port, '0.0.0.0', () => {});
process.on('SIGTERM', () => process.exit(0));
process.on('SIGINT', () => process.exit(0));`, port)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte(serverJS), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := EvaluateRepository(context.Background(), root, "Node.js Slow Install")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OctoScore.OneShotSuccess || !result.OctoScore.VerifiedReadiness {
		t.Fatalf("slow install should succeed under the setup budget; diagnostics: %+v", result.Diagnostics)
	}
	assertBenchmarkReleasedResources(t, root, port)
}

// A hung start must still fail within the start budget, with the deadline recorded.
func TestEvaluateRepositoryStartBudgetStillEnforced(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not installed")
	}
	oldSetup, oldStart := SetupBudget, StartBudget
	SetupBudget, StartBudget = 60*time.Second, 1500*time.Millisecond
	t.Cleanup(func() { SetupBudget, StartBudget = oldSetup, oldStart })

	root := t.TempDir()
	port := getFreePort()
	packageJSON := fmt.Sprintf(`{"name":"hung-start","scripts":{"start":"node index.js -p %d"}}`, port)
	// Listens only after the start budget has expired.
	serverJS := fmt.Sprintf(`setTimeout(function(){ require('http').createServer(function(q,r){r.end('OK')}).listen(%d); }, 20000);
process.on('SIGTERM', () => process.exit(0));`, port)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte(serverJS), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateRepository(context.Background(), root, "Node.js Hung Start")
	if err != nil {
		t.Fatal(err)
	}
	if result.OctoScore.OneShotSuccess || result.OctoScore.VerifiedReadiness {
		t.Fatal("a server that never becomes ready must not count as success")
	}
	if result.Diagnostics == nil || result.Diagnostics.FailureReason == "" {
		t.Fatalf("expected failure diagnostics, got %+v", result.Diagnostics)
	}
}

func TestSplitPlanByPhase(t *testing.T) {
	plan := intelligence.ExecutionPlan{
		Ports: []intelligence.PortAssignment{{Component: "web", Resolved: 1234}},
		Steps: []intelligence.ExecutionStep{
			{ID: "prep", Phase: intelligence.PhasePrepare},
			{ID: "inst", Phase: intelligence.PhaseInstall, DependsOn: []string{"prep"}},
			{ID: "run", Phase: intelligence.PhaseStart, DependsOn: []string{"inst"}},
			{ID: "health", Phase: intelligence.PhaseHealth, DependsOn: []string{"run"}},
		},
	}
	setup, start := splitPlanByPhase(plan)
	if len(setup.Steps) != 2 || len(start.Steps) != 2 {
		t.Fatalf("setup=%d start=%d steps, want 2 and 2", len(setup.Steps), len(start.Steps))
	}
	if len(setup.Ports) != 0 || len(start.Ports) != 1 {
		t.Fatalf("ports: setup=%d start=%d", len(setup.Ports), len(start.Ports))
	}
	if err := intelligence.ValidateExecutionPlan(setup); err != nil {
		t.Fatal(err)
	}
	if err := intelligence.ValidateExecutionPlan(start); err != nil {
		t.Fatal(err)
	}
	if len(start.Steps[0].DependsOn) != 0 {
		t.Fatalf("cross-plan dependency kept: %v", start.Steps[0].DependsOn)
	}
}

func TestBenchmarkTeardownRejectsOpenListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()
	report := intelligence.ExecutionReport{ActiveProcesses: []intelligence.ActiveProcess{{Port: port}}}
	if err := waitForBenchmarkPortsClosed(report, 25*time.Millisecond); err == nil {
		t.Fatal("an open benchmark listener must be a teardown failure")
	}
	listener.Close()
	if err := waitForBenchmarkPortsClosed(report, time.Second); err != nil {
		t.Fatal(err)
	}
}
