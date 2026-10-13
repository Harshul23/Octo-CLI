package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// These tests build the real octo binary and drive it against a small
// monorepo fixture. They are opt-in because they bind local ports and start a
// Postgres container: set OCTO_INTEGRATION_TESTS=1. They are skipped when
// Docker or Node are not installed.

func requireIntegrationEnv(t *testing.T, needDocker bool) {
	t.Helper()
	if os.Getenv("OCTO_INTEGRATION_TESTS") != "1" {
		t.Skip("set OCTO_INTEGRATION_TESTS=1 to run real octo binary integration tests")
	}
	if runtime.GOOS == "windows" {
		t.Skip("integration fixture uses POSIX signals")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	if needDocker {
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("docker not available")
		}
		if err := exec.Command("docker", "info").Run(); err != nil {
			t.Skip("docker daemon not running")
		}
	}
}

func buildOcto(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "octo")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const fixtureServer = `const http = require('http');
const port = process.env.PORT;
http.createServer((q, r) => { r.end('ok'); }).listen(port, () => console.log('listening ' + port));
`

func monorepoFixture(t *testing.T, webPort, apiPort int, withCompose bool) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","private":true,"workspaces":["apps/*"]}`)
	for name, port := range map[string]int{"web": webPort, "api": apiPort} {
		app := filepath.Join(dir, "apps", name)
		writeFixtureFile(t, filepath.Join(app, "package.json"), fmt.Sprintf(`{"name":%q,"version":"1.0.0","scripts":{"start":"node server.js"}}`, name))
		writeFixtureFile(t, filepath.Join(app, "server.js"), fixtureServer)
		writeFixtureFile(t, filepath.Join(app, ".env"), fmt.Sprintf("PORT=%d\n", port))
	}
	if withCompose {
		writeFixtureFile(t, filepath.Join(dir, "docker-compose.yml"), `services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: password
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 2s
      timeout: 2s
      retries: 15
`)
	}
	return dir
}

func waitForHTTP(t *testing.T, port int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("nothing responded on port %d within %s", port, within)
}

func stopDetached(t *testing.T, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".octo", "processes.json"))
	if err != nil {
		return
	}
	var procs []struct {
		PID int `json:"pid"`
	}
	_ = json.Unmarshal(data, &procs)
	for _, p := range procs {
		if p.PID > 0 {
			_ = syscall.Kill(-p.PID, syscall.SIGTERM)
			_ = syscall.Kill(p.PID, syscall.SIGTERM)
		}
	}
}

func runOcto(t *testing.T, bin, dir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"run"}, args...)...)
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestBinaryJSONSelectionErrorIsStructured(t *testing.T) {
	requireIntegrationEnv(t, false)
	bin := buildOcto(t)
	dir := monorepoFixture(t, 38101, 38102, false)
	stdout, _, err := runOcto(t, bin, dir, "--json")
	if err == nil {
		t.Fatal("expected a non-zero exit when no selection is given")
	}
	var parsed struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if jerr := json.Unmarshal([]byte(stdout), &parsed); jerr != nil || parsed.Error.Code != "selection_required" {
		t.Fatalf("expected structured JSON error on stdout, got %q (%v)", stdout, jerr)
	}
}

func TestBinaryOnlyWebDetach(t *testing.T) {
	requireIntegrationEnv(t, false)
	bin := buildOcto(t)
	dir := monorepoFixture(t, 38111, 38112, false)
	defer stopDetached(t, dir)
	if _, stderr, err := runOcto(t, bin, dir, "--only", "web", "--detach", "--no-tui"); err != nil {
		t.Fatalf("octo run --only web --detach failed: %v\n%s", err, stderr)
	}
	waitForHTTP(t, 38111, 15*time.Second)
	if _, err := os.Stat(filepath.Join(dir, ".octo.lock")); err != nil {
		t.Fatalf("expected .octo.lock after a verified detached start: %v", err)
	}
}

func TestBinaryOnlyApiDetach(t *testing.T) {
	requireIntegrationEnv(t, false)
	bin := buildOcto(t)
	dir := monorepoFixture(t, 38121, 38122, false)
	defer stopDetached(t, dir)
	if _, stderr, err := runOcto(t, bin, dir, "--only", "api", "--detach", "--no-tui"); err != nil {
		t.Fatalf("octo run --only api --detach failed: %v\n%s", err, stderr)
	}
	waitForHTTP(t, 38122, 15*time.Second)
}

func TestBinaryAllDetachWithCompose(t *testing.T) {
	requireIntegrationEnv(t, true)
	bin := buildOcto(t)
	dir := monorepoFixture(t, 38131, 38132, true)
	defer stopDetached(t, dir)
	defer func() {
		stop := exec.Command("docker", "compose", "-f", filepath.Join(dir, "docker-compose.yml"), "stop", "postgres")
		stop.Dir = dir
		_ = stop.Run()
	}()
	if _, stderr, err := runOcto(t, bin, dir, "--all", "--detach", "--no-tui"); err != nil {
		t.Fatalf("octo run --all --detach failed: %v\n%s", err, stderr)
	}
	waitForHTTP(t, 38131, 15*time.Second)
	waitForHTTP(t, 38132, 15*time.Second)
}

func TestBinaryCtrlCAfterVerifiedStartupLeavesLock(t *testing.T) {
	requireIntegrationEnv(t, false)
	bin := buildOcto(t)
	dir := monorepoFixture(t, 38141, 38142, false)
	cmd := exec.Command(bin, "run", "--only", "web", "--no-tui")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waitForHTTP(t, 38141, 20*time.Second)
	time.Sleep(500 * time.Millisecond)
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT) // Ctrl+C reaches the whole foreground group
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.Fatal("octo did not exit after SIGINT")
	}
	if _, err := os.Stat(filepath.Join(dir, ".octo.lock")); err != nil {
		t.Fatalf("expected .octo.lock to survive Ctrl+C after verified startup: %v", err)
	}
}
