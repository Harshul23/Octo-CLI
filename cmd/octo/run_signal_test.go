package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
	"github.com/spf13/cobra"
)

func TestRunWithIntelligenceForegroundCtrlCPersistsLock(t *testing.T) {
	root := t.TempDir()

	// Create test package files
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"test-app","scripts":{"start":"node index.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("console.log('running');"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	// Write .octo.yaml declaring the component port so planner marks it LongRunning
	octoYAML := fmt.Sprintf(`spec: octo.dev/v1
components:
  test-app:
    run_command: "npm start"
    port: %d
`, port)
	if err := os.WriteFile(filepath.Join(root, ".octo.yaml"), []byte(octoYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("engine", "intelligence", "")
	cmd.Flags().Bool("watch", false, "")
	cmd.Flags().Bool("detach", false, "")
	cmd.Flags().Bool("sandbox", false, "")
	cmd.Flags().Bool("json", true, "")
	cmd.Flags().Bool("no-tui", true, "")
	cmd.Flags().Bool("select", false, "")
	cmd.Flags().Bool("skip-env-check", true, "")
	cmd.Flags().Bool("skip-env", true, "")

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	// Custom adapter that starts a real listener and blocks in Start until cancel
	blockCh := make(chan struct{})
	startedCh := make(chan struct{})
	adapter := &testBlockingStartableAdapter{
		port:      port,
		startedCh: startedCh,
		blockCh:   blockCh,
	}
	intelligence.RegisterRuntimeAdapter(adapter)
	t.Cleanup(func() {
		intelligence.UnregisterRuntimeAdapter(adapter)
	})

	done := make(chan error, 1)
	go func() {
		done <- runWithIntelligence(cmd, []string{root})
	}()

	// Wait until step has started
	select {
	case <-startedCh:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timed out waiting for adapter start")
	}

	// Wait briefly for verified startup checkpoint to complete
	deadline := time.Now().Add(2 * time.Second)
	lockFound := false
	for time.Now().Before(deadline) {
		lock, err := intelligence.LoadOctoLock(root)
		if err == nil && len(lock.Strategies) > 0 {
			lockFound = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !lockFound {
		cancel()
		t.Fatal("expected .octo.lock to be saved at verified startup checkpoint")
	}

	// Trigger cancellation (simulating Ctrl+C)
	cancel()
	close(blockCh)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithIntelligence returned unexpected error on interrupt: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for runWithIntelligence wrap-up")
	}

	// Assert lock remains valid on disk after Ctrl+C
	lock, err := intelligence.LoadOctoLock(root)
	if err != nil {
		t.Fatalf("failed to load .octo.lock: %v", err)
	}
	if len(lock.Strategies) == 0 {
		t.Fatal("expected .octo.lock to have strategies preserved after Ctrl+C")
	}
}

type testBlockingStartableAdapter struct {
	port      int
	startedCh chan struct{}
	blockCh   chan struct{}
}

func (a *testBlockingStartableAdapter) Name() string { return "test-blocking" }
func (a *testBlockingStartableAdapter) Supports(step intelligence.ExecutionStep) bool {
	return step.Component == "test-app"
}
func (a *testBlockingStartableAdapter) Execute(_ context.Context, _ intelligence.ExecutionStep, _ intelligence.ResolvedEnvironment) error {
	return nil
}
func (a *testBlockingStartableAdapter) Start(_ context.Context, step intelligence.ExecutionStep, _ intelligence.ResolvedEnvironment) (intelligence.RunningProcess, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", a.port))
	if err != nil {
		return nil, err
	}
	select {
	case a.startedCh <- struct{}{}:
	default:
	}
	return &testBlockingProcess{listener: l, blockCh: a.blockCh}, nil
}

type testBlockingProcess struct {
	listener net.Listener
	blockCh  chan struct{}
}

func (p *testBlockingProcess) Wait() error {
	<-p.blockCh
	if p.listener != nil {
		_ = p.listener.Close()
	}
	return nil
}
func (p *testBlockingProcess) Stop() error {
	if p.listener != nil {
		_ = p.listener.Close()
	}
	return nil
}
func (p *testBlockingProcess) GracefulStop(_ time.Duration) error {
	return p.Stop()
}
func (p *testBlockingProcess) Pid() int {
	return 99999
}
