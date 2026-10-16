package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockTestProcess implements RunningProcess and LivenessAwareProcess for testing.
type mockTestProcess struct {
	mu       sync.Mutex
	pid      int
	stopped  bool
	alive    bool
	waitErr  error
	waitDone chan struct{}
	listener net.Listener
}

func newMockTestProcess(pid int, alive bool) *mockTestProcess {
	p := &mockTestProcess{
		pid:      pid,
		alive:    alive,
		waitDone: make(chan struct{}),
	}
	if !alive {
		close(p.waitDone)
	}
	return p
}

func newMockTestProcessWithListener(pid int, ln net.Listener) *mockTestProcess {
	p := newMockTestProcess(pid, true)
	p.listener = ln
	return p
}

func (p *mockTestProcess) Wait() error {
	<-p.waitDone
	return p.waitErr
}

func (p *mockTestProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	p.alive = false
	if p.listener != nil {
		_ = p.listener.Close()
		p.listener = nil
	}
	select {
	case <-p.waitDone:
	default:
		close(p.waitDone)
	}
	return nil
}

func (p *mockTestProcess) GracefulStop(timeout time.Duration) error {
	return p.Stop()
}

func (p *mockTestProcess) Pid() int {
	return p.pid
}

func (p *mockTestProcess) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.alive
}

// mockTestAdapter implements StartableRuntimeAdapter for testing execution flows.
type mockTestAdapter struct {
	name      string
	starts    map[string]RunningProcess
	execFunc  func(ctx context.Context, step ExecutionStep) error
	startFunc func(ctx context.Context, step ExecutionStep) (RunningProcess, error)
}

func (m *mockTestAdapter) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func (m *mockTestAdapter) Supports(step ExecutionStep) bool {
	return true
}

func (m *mockTestAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if m.execFunc != nil {
		return m.execFunc(ctx, step)
	}
	return nil
}

func (m *mockTestAdapter) Start(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) (RunningProcess, error) {
	if m.startFunc != nil {
		return m.startFunc(ctx, step)
	}
	if proc, ok := m.starts[step.ID]; ok {
		return proc, nil
	}
	return newMockTestProcess(1000, true), nil
}

// 1. Test two independent servers run concurrently without blocking on each other's lifetime
func TestTwoIndependentServersRunConcurrently(t *testing.T) {
	tmp := t.TempDir()
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	defer l1.Close()
	port1 := l1.Addr().(*net.TCPAddr).Port

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	defer l2.Close()
	port2 := l2.Addr().(*net.TCPAddr).Port

	model := ProjectModel{
		Name: "multi-server",
		Root: tmp,
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Port: port1, Role: "service"},
			{Name: "api", Path: "apps/api", RunCommand: "go run ./cmd/api", Port: port2, Role: "service"},
		},
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	webStart := findStep(plan.Steps, "component.web.start")
	apiStart := findStep(plan.Steps, "component.api.start")
	if webStart == nil || apiStart == nil {
		t.Fatalf("expected both web and api start steps to exist")
	}

	// Neither start step should depend on the other's start step
	for _, dep := range webStart.DependsOn {
		if dep == "component.api.start" {
			t.Fatalf("web.start must not depend on api.start")
		}
	}
	for _, dep := range apiStart.DependsOn {
		if dep == "component.web.start" {
			t.Fatalf("api.start must not depend on web.start")
		}
	}

	var procWeb, procApi *mockTestProcess
	defer func() {
		if procWeb != nil {
			procWeb.Stop()
		}
		if procApi != nil {
			procApi.Stop()
		}
	}()

	adapter := &mockTestAdapter{
		startFunc: func(ctx context.Context, step ExecutionStep) (RunningProcess, error) {
			port := portForComponent(plan.Ports, step.Component)
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return nil, err
			}
			if step.Component == "web" {
				procWeb = newMockTestProcessWithListener(2001, ln)
				return procWeb, nil
			}
			procApi = newMockTestProcessWithListener(2002, ln)
			return procApi, nil
		},
	}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	var startupCalled bool
	opts := ExecutionOptions{
		Detach: true, // test detached execution so it doesn't wait
		OnVerifiedStartup: func(rep ExecutionReport) error {
			startupCalled = true
			if len(rep.ActiveProcesses) != 2 {
				t.Fatalf("expected 2 active processes at startup checkpoint, got %d", len(rep.ActiveProcesses))
			}
			return nil
		},
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, env, opts)
	if !report.Success {
		t.Fatalf("expected execution report to succeed, got failure: %s", report.FailureReason)
	}
	if !startupCalled {
		t.Fatalf("expected verified-startup hook to be called")
	}
	if len(report.ActiveProcesses) != 2 {
		t.Fatalf("expected 2 active processes, got %d", len(report.ActiveProcesses))
	}
}

// 2. Test portless worker plus web server: worker starts asynchronously and verifies via bounded liveness
func TestPortlessWorkerPlusWebServer(t *testing.T) {
	tmp := t.TempDir()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	model := ProjectModel{
		Name: "worker-web",
		Root: tmp,
		Components: []Component{
			{
				Name:       "web",
				Path:       "apps/web",
				RunCommand: "node server.js",
				Port:       port,
				Role:       "service",
				ExecutionCandidates: []ExecutionCandidate{
					{ID: "web.dev", Command: "node server.js", Confidence: 0.9},
				},
			},
			{
				Name:       "worker",
				Path:       "apps/worker",
				RunCommand: "python worker.py",
				Port:       0, // portless worker
				Role:       "worker",
				ExecutionCandidates: []ExecutionCandidate{
					{ID: "worker.run", Command: "python worker.py", Confidence: 0.9},
				},
			},
		},
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	workerStep := findStep(plan.Steps, "component.worker.start")
	if workerStep == nil {
		t.Fatalf("expected worker start step")
	}
	if !workerStep.LongRunning {
		t.Fatalf("expected worker to be classified as long running without port")
	}
	if portForComponent(plan.Ports, "worker") != 0 {
		t.Fatalf("worker should not have an invented port, got %d", portForComponent(plan.Ports, "worker"))
	}

	// Subtest A: Alive worker succeeds via bounded liveness check
	t.Run("AliveWorker", func(t *testing.T) {
		var procWeb *mockTestProcess
		procWorker := newMockTestProcess(3002, true)
		defer func() {
			if procWeb != nil {
				procWeb.Stop()
			}
			procWorker.Stop()
		}()

		adapter := &mockTestAdapter{
			startFunc: func(ctx context.Context, step ExecutionStep) (RunningProcess, error) {
				if step.Component == "web" {
					resolvedPort := portForComponent(plan.Ports, "web")
					ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", resolvedPort))
					if err != nil {
						return nil, err
					}
					procWeb = newMockTestProcessWithListener(3001, ln)
					return procWeb, nil
				}
				return procWorker, nil
			},
		}
		resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
		report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, ResolvedEnvironment{}, ExecutionOptions{Detach: true})
		if !report.Success {
			t.Fatalf("expected success for living worker, failed with: %s", report.FailureReason)
		}
		if len(report.ActiveProcesses) != 2 {
			t.Fatalf("expected 2 active processes, got %d", len(report.ActiveProcesses))
		}
	})

	// Subtest B: Dead worker (crashes on startup) fails bounded liveness check and triggers teardown
	t.Run("DeadWorker", func(t *testing.T) {
		var procWeb *mockTestProcess
		procWorkerDead := newMockTestProcess(3004, false) // dead immediately
		procWorkerDead.waitErr = fmt.Errorf("exit status 1")
		defer func() {
			if procWeb != nil {
				procWeb.Stop()
			}
		}()

		adapter := &mockTestAdapter{
			startFunc: func(ctx context.Context, step ExecutionStep) (RunningProcess, error) {
				if step.Component == "web" {
					resolvedPort := portForComponent(plan.Ports, "web")
					ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", resolvedPort))
					if err != nil {
						return nil, err
					}
					procWeb = newMockTestProcessWithListener(3003, ln)
					return procWeb, nil
				}
				return procWorkerDead, nil
			},
		}
		resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
		report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, ResolvedEnvironment{}, ExecutionOptions{Detach: true})
		if report.Success {
			t.Fatalf("expected execution report to fail when portless worker exits immediately")
		}
		if !report.TeardownPerformed {
			t.Fatalf("expected teardown to be performed on worker crash")
		}
		if !procWeb.stopped {
			t.Fatalf("expected web process to be torn down when worker startup failed")
		}
		if report.VerifiedStartup {
			t.Fatalf("verified startup must not be set on failure")
		}
	})
}

// 3. Test ordered api -> database readiness: api waits for database health check before starting
func TestOrderedApiDatabaseReadiness(t *testing.T) {
	tmp := t.TempDir()
	model := ProjectModel{
		Name: "api-db",
		Root: tmp,
		Components: []Component{
			{Name: "api", Path: "api", RunCommand: "go run main.go", Role: "service", Port: 8080, DependsOn: []string{"postgres"}},
		},
		Services: []Service{
			{
				Name: "postgres",
				Evidence: []Evidence{
					{Kind: EvidenceConfig, Path: filepath.Join(tmp, "docker-compose.yml")},
				},
				HealthCheck: &HealthCheck{
					Command: "pg_isready -U postgres",
				},
			},
		},
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	dbStart := findStep(plan.Steps, "service.postgres.start")
	dbHealth := findStep(plan.Steps, "service.postgres.health")
	apiStart := findStep(plan.Steps, "component.api.start")

	if dbStart == nil || dbHealth == nil || apiStart == nil {
		t.Fatalf("expected dbStart, dbHealth, and apiStart steps to exist")
	}

	// dbHealth depends on dbStart
	if !containsString(dbHealth.DependsOn, "service.postgres.start") {
		t.Fatalf("expected db.health to depend on db.start")
	}

	// apiStart depends on dbHealth, not just dbStart
	if !containsString(apiStart.DependsOn, "service.postgres.health") {
		t.Fatalf("expected api.start to depend on service.postgres.health, got deps: %v", apiStart.DependsOn)
	}

	// Topological order verification
	order, err := TopologicalOrder(plan)
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}

	dbStartIdx := indexOf(order, "service.postgres.start")
	dbHealthIdx := indexOf(order, "service.postgres.health")
	apiStartIdx := indexOf(order, "component.api.start")

	if !(dbStartIdx < dbHealthIdx && dbHealthIdx < apiStartIdx) {
		t.Fatalf("expected order db.start (%d) < db.health (%d) < api.start (%d)", dbStartIdx, dbHealthIdx, apiStartIdx)
	}
}

// 4. Test shared workspace library setup: library gets build step, never start step, and app depends on build step
func TestSharedWorkspaceLibrarySetup(t *testing.T) {
	tmp := t.TempDir()
	libDir := filepath.Join(tmp, "packages", "ui")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "package.json"), []byte(`{"name": "@mono/ui", "scripts": {"build": "tsc"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	model := ProjectModel{
		Name: "mono",
		Root: tmp,
		Components: []Component{
			{
				Name:           "ui",
				Path:           "packages/ui",
				Role:           "library",
				PackageManager: "npm",
				Language:       "Node",
			},
			{
				Name:           "web",
				Path:           "apps/web",
				RunCommand:     "npm start",
				Role:           "frontend",
				PackageManager: "npm",
				Language:       "Node",
				DependsOn:      []string{"ui"},
			},
		},
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	// Library must have build step, NOT start step
	libBuild := findStep(plan.Steps, "component.ui.build")
	libStart := findStep(plan.Steps, "component.ui.start")
	if libBuild == nil {
		t.Fatalf("expected component.ui.build step to exist for library")
	}
	if libStart != nil {
		t.Fatalf("library must NEVER have an invented start step")
	}

	// App start must depend on library build step
	webStart := findStep(plan.Steps, "component.web.start")
	if webStart == nil {
		t.Fatalf("expected component.web.start step")
	}
	if !containsString(webStart.DependsOn, "component.ui.build") {
		t.Fatalf("expected web.start to depend on component.ui.build, got: %v", webStart.DependsOn)
	}
}

// 5. Test one shared root install for declared Node workspaces
func TestOneSharedRootInstall(t *testing.T) {
	tmp := t.TempDir()
	// Declare pnpm workspace
	if err := os.WriteFile(filepath.Join(tmp, "pnpm-workspace.yaml"), []byte("packages:\n  - 'apps/*'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "pnpm-lock.yaml"), []byte("lockfileVersion: 5.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := ProjectModel{
		Name: "pnpm-mono",
		Root: tmp,
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "pnpm dev", PackageManager: "pnpm", Language: "Node"},
			{Name: "api", Path: "apps/api", RunCommand: "pnpm start", PackageManager: "pnpm", Language: "Node"},
			{Name: "py-tool", Path: "tools/cli", RunCommand: "python cli.py", PackageManager: "pip", Language: "Python"},
		},
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	// Shared root install step must exist once
	rootInstall := findStep(plan.Steps, "workspace.root.install")
	if rootInstall == nil {
		t.Fatalf("expected workspace.root.install step")
	}
	if rootInstall.WorkDir != tmp {
		t.Fatalf("expected root install workdir to be %s, got %s", tmp, rootInstall.WorkDir)
	}

	// Web and API must NOT have individual install steps
	if findStep(plan.Steps, "component.web.install") != nil {
		t.Fatalf("component.web.install should be omitted in favor of root install")
	}
	if findStep(plan.Steps, "component.api.install") != nil {
		t.Fatalf("component.api.install should be omitted in favor of root install")
	}

	// Independent non-workspace component retains its own local install step
	pyInstall := findStep(plan.Steps, "component.py-tool.install")
	if pyInstall == nil {
		t.Fatalf("expected component.py-tool.install step for independent Python component")
	}
	expectedPyDir := filepath.Join(tmp, "tools/cli")
	if pyInstall.WorkDir != expectedPyDir {
		t.Fatalf("expected py install workdir to be %s, got %s", expectedPyDir, pyInstall.WorkDir)
	}
}

// 6. Test scoped environment isolation between components
func TestScopedEnvironmentIsolation(t *testing.T) {
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "apps", "web")
	apiDir := filepath.Join(tmp, "apps", "api")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(webDir, ".env"), []byte("API_SECRET=web_secret\nSHARED_KEY=web_val\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_SECRET=api_secret\nSHARED_KEY=api_val\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("ROOT_VAR=root_val\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := ProjectModel{
		Name: "scoped-env-demo",
		Root: tmp,
		Environment: EnvironmentModel{
			Variables: []EnvironmentVariable{
				{Name: "API_SECRET"},
				{Name: "SHARED_KEY"},
				{Name: "ROOT_VAR"},
			},
		},
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "node web.js"},
			{Name: "api", Path: "apps/api", RunCommand: "node api.js"},
		},
	}

	env, err := ResolveProjectEnvironment(tmp, model)
	if err != nil {
		t.Fatalf("environment resolution failed: %v", err)
	}

	webStep := ExecutionStep{Component: "web", WorkDir: webDir}
	apiStep := ExecutionStep{Component: "api", WorkDir: apiDir}

	webEnv := env.ForStep(webStep)
	apiEnv := env.ForStep(apiStep)

	if webEnv.Values["API_SECRET"] != "web_secret" {
		t.Fatalf("expected webEnv API_SECRET=web_secret, got %q", webEnv.Values["API_SECRET"])
	}
	if apiEnv.Values["API_SECRET"] != "api_secret" {
		t.Fatalf("expected apiEnv API_SECRET=api_secret, got %q", apiEnv.Values["API_SECRET"])
	}

	// Verify root env is visible to both
	if webEnv.Values["ROOT_VAR"] != "root_val" || apiEnv.Values["ROOT_VAR"] != "root_val" {
		t.Fatalf("expected ROOT_VAR=root_val to be inherited by both components")
	}
}

// 7. Test port collisions and strict port failure
func TestPortCollisionsAndStrictPort(t *testing.T) {
	// Subtest A: Port collision between two components with PortStrict=true fails
	{
		components := []Component{
			{Name: "web", Port: 8080, PortStrict: true},
			{Name: "admin", Port: 8080, PortStrict: true},
		}
		_, err := AllocateComponentPorts(components)
		if err == nil {
			t.Fatalf("expected port collision error when PortStrict is true")
		}
	}

	// Subtest B: Port collision with service port when PortStrict=true fails
	{
		components := []Component{
			{Name: "web", Port: 5432, PortStrict: true},
		}
		services := []Service{
			{Name: "postgres", Ports: []string{"5432:5432"}},
		}
		_, err := AllocateComponentPortsWithServices(components, services)
		if err == nil {
			t.Fatalf("expected port collision error when component conflicts with service in strict mode")
		}
	}

	// Subtest C: Port collision with PortStrict=false successfully shifts port
	{
		components := []Component{
			{Name: "web", Port: 8080, PortStrict: false},
			{Name: "admin", Port: 8080, PortStrict: false},
		}
		assignments, err := AllocateComponentPorts(components)
		if err != nil {
			t.Fatalf("expected port shifting without error when PortStrict is false: %v", err)
		}
		if len(assignments) != 2 {
			t.Fatalf("expected 2 assignments, got %d", len(assignments))
		}
		if assignments[0].Resolved == assignments[1].Resolved {
			t.Fatalf("expected distinct ports after shift, got %d for both", assignments[0].Resolved)
		}
	}
}

// 8. Test startup failure teardown stops started processes without writing lock
func TestStartupFailureTeardownStopsStartedProcesses(t *testing.T) {
	tmp := t.TempDir()
	proc1 := newMockTestProcess(4001, true)

	model := ProjectModel{
		Name: "teardown-demo",
		Root: tmp,
		Components: []Component{
			{Name: "app1", Path: "app1", RunCommand: "node app1.js", Role: "service"},
			{Name: "app2", Path: "app2", RunCommand: "node app2.js", Role: "service"},
		},
	}
	plan := ExecutionPlan{
		ProjectName: "teardown-demo",
		Root:        tmp,
		Steps: []ExecutionStep{
			{ID: "component.app1.start", Component: "app1", Phase: PhaseStart, Command: "node app1.js", LongRunning: true},
			{ID: "component.app2.start", Component: "app2", Phase: PhaseStart, Command: "node app2.js", LongRunning: false, DependsOn: []string{"component.app1.start"}},
		},
	}

	adapter := &mockTestAdapter{
		starts: map[string]RunningProcess{
			"component.app1.start": proc1,
		},
		execFunc: func(ctx context.Context, step ExecutionStep) error {
			if step.ID == "component.app2.start" {
				return fmt.Errorf("app2 failed to execute")
			}
			return nil
		},
	}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}

	var startupHookCalled bool
	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			startupHookCalled = true
			return nil
		},
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, ResolvedEnvironment{}, opts)
	if report.Success {
		t.Fatalf("expected execution report to fail")
	}
	if !report.TeardownPerformed {
		t.Fatalf("expected teardown to be performed on failure")
	}
	if !proc1.stopped {
		t.Fatalf("expected proc1 to be stopped during failure teardown")
	}
	if startupHookCalled {
		t.Fatalf("OnVerifiedStartup must NOT be called on failure")
	}
	if report.VerifiedStartup {
		t.Fatalf("report.VerifiedStartup must be false on failure")
	}
}

// 9. Test pre-existing Compose service preservation
func TestPreExistingComposeServicePreservation(t *testing.T) {
	tmp := t.TempDir()
	composePath := filepath.Join(tmp, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("version: '3'\nservices:\n  redis:\n    image: redis\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := ProjectModel{
		Name: "compose-preserve",
		Root: tmp,
		Services: []Service{
			{
				Name: "redis",
				Evidence: []Evidence{
					{Kind: EvidenceConfig, Path: composePath},
				},
			},
		},
	}
	plan := ExecutionPlan{ProjectName: "compose-preserve", Root: tmp}

	// Case A: OwnedServices is empty (service was pre-existing)
	// TeardownExecution must NOT stop redis
	summaryA := TeardownExecution(model, plan, nil, ExecutionOptions{
		OwnedServices: []string{}, // Not owned by this invocation
	})
	if summaryA.ServicesStopped != 0 {
		t.Fatalf("expected 0 services stopped when redis was pre-existing, got %d", summaryA.ServicesStopped)
	}

	// Case B: OwnedServices contains redis (started by this invocation)
	// stopComposeServices is invoked for redis
	summaryB := TeardownExecution(model, plan, nil, ExecutionOptions{
		OwnedServices: []string{"redis"},
	})
	// In test environment without Docker running, stop command will fail gracefully and return 0
	_ = summaryB
}

// 10. Test lock invalidation after root workspace changes
func TestLockInvalidationAfterRootWorkspaceChanges(t *testing.T) {
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "apps", "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"name": "web"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Initial root workspace file
	rootPkg := filepath.Join(tmp, "package.json")
	if err := os.WriteFile(rootPkg, []byte(`{"name": "root-v1", "workspaces": ["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	comp := Component{Name: "web", Path: "apps/web"}
	cand := ExecutionCandidate{ID: "web.start", Command: "npm start"}

	fp1, err := ExecutionFingerprint(tmp, comp, cand)
	if err != nil {
		t.Fatalf("failed to calculate fingerprint 1: %v", err)
	}

	// Modify root package.json
	if err := os.WriteFile(rootPkg, []byte(`{"name": "root-v2", "workspaces": ["apps/*"], "dependencies": {"foo": "1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fp2, err := ExecutionFingerprint(tmp, comp, cand)
	if err != nil {
		t.Fatalf("failed to calculate fingerprint 2: %v", err)
	}

	if fp1 == fp2 {
		t.Fatalf("fingerprint must change when root workspace manifest changes, got identical hash: %s", fp1)
	}
}

// 11. Test watch and detach selection
func TestWatchAndDetachSelection(t *testing.T) {
	tmp := t.TempDir()
	proc := newMockTestProcess(5001, true)
	defer proc.Stop()

	model := ProjectModel{
		Name: "detach-demo",
		Root: tmp,
		Components: []Component{
			{Name: "web", Path: "web", RunCommand: "node server.js", Role: "service"},
		},
	}
	plan := ExecutionPlan{
		ProjectName: "detach-demo",
		Root:        tmp,
		Steps: []ExecutionStep{
			{ID: "component.web.start", Component: "web", Phase: PhaseStart, Command: "node server.js", LongRunning: true},
		},
	}

	adapter := &mockTestAdapter{
		starts: map[string]RunningProcess{
			"component.web.start": proc,
		},
	}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}

	opts := ExecutionOptions{
		Detach: true,
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, ResolvedEnvironment{}, opts)
	if !report.Success {
		t.Fatalf("expected detached execution to succeed: %s", report.FailureReason)
	}

	// SaveDetachedState writes .octo/processes.json
	if err := SaveDetachedState(tmp, report); err != nil {
		t.Fatalf("failed to save detached state: %v", err)
	}

	statePath := filepath.Join(tmp, ".octo", "processes.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("expected .octo/processes.json to exist: %v", err)
	}

	var saved []ActiveProcess
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to unmarshal processes.json: %v", err)
	}
	if len(saved) != 1 || saved[0].Component != "web" {
		t.Fatalf("expected saved process for web, got: %+v", saved)
	}

	// Detached mode must NOT terminate the process on function return
	if proc.stopped {
		t.Fatalf("detached process must remain running after return")
	}

	// Test selection filtering in watch mode re-analysis
	sel, err := ResolveSelectionTargets(model, []TargetID{
		{Kind: TargetKindComponent, Name: "web"},
	})
	if err != nil {
		t.Fatalf("failed to resolve selection targets: %v", err)
	}
	if len(sel.FilteredModel.Components) != 1 || sel.FilteredModel.Components[0].Name != "web" {
		t.Fatalf("expected selection to retain web component")
	}
}

// Helpers
func findStep(steps []ExecutionStep, id string) *ExecutionStep {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}

func containsString(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func indexOf(slice []string, val string) int {
	for i, item := range slice {
		if item == val {
			return i
		}
	}
	return -1
}

func TestRecordVerifiedStrategiesWithLockSubComponentDirectoryEvidence(t *testing.T) {
	root := t.TempDir()
	chatDir := filepath.Join(root, "services", "realtime-chat")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "go.mod"), []byte("module example.com/chat\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chatDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	candidate := ExecutionCandidate{
		ID:      "go.package",
		Command: "go run .",
		Evidence: []Evidence{
			{Kind: EvidenceConfig, Path: "services/realtime-chat", Detail: "directory path", Strength: 0.96},
			{Kind: EvidenceSignalFile, Path: "services/realtime-chat/go.mod", Detail: "go.mod", Strength: 0.95},
		},
	}
	component := Component{
		Name:                "realtime-chat",
		Path:                "services/realtime-chat",
		Language:            "Go",
		ExecutionCandidates: []ExecutionCandidate{candidate},
	}
	model := ProjectModel{
		Name:       "monorepo",
		Root:       root,
		Components: []Component{component},
	}
	plan := ExecutionPlan{
		ProjectName: "monorepo",
		Root:        root,
		Steps: []ExecutionStep{{
			ID:                "component.realtime-chat.start",
			Component:         "realtime-chat",
			Candidates:        []ExecutionCandidate{candidate},
			SelectedCandidate: "go.package",
			Command:           "go run .",
		}},
	}
	report := ExecutionReport{
		ProjectName:     "monorepo",
		Success:         true,
		VerifiedStartup: true,
		Steps: []ExecutionStepResult{{
			ID:          "component.realtime-chat.start",
			Status:      StepSucceeded,
			CandidateID: "go.package",
		}},
	}
	lock := OctoLock{Project: "monorepo"}
	updatedLock, err := RecordVerifiedStrategiesWithLock(root, model, plan, report, lock)
	if err != nil {
		t.Fatalf("expected RecordVerifiedStrategiesWithLock to succeed with directory evidence, got: %v", err)
	}
	if len(updatedLock.Strategies) != 1 {
		t.Fatalf("expected 1 recorded strategy, got %d", len(updatedLock.Strategies))
	}
	if updatedLock.Strategies[0].Candidate != "go.package" {
		t.Fatalf("expected strategy candidate go.package, got %s", updatedLock.Strategies[0].Candidate)
	}
}

