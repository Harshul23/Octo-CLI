package intelligence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// fakeRunningProcess allows deterministic control over process lifecycle without sleeps.
type fakeRunningProcess struct {
	pid      int
	waitCh   chan error
	stopOnce sync.Once
	stopped  bool
	mu       sync.Mutex
}

func newFakeRunningProcess(pid int) *fakeRunningProcess {
	return &fakeRunningProcess{
		pid:    pid,
		waitCh: make(chan error, 1),
	}
}

func (p *fakeRunningProcess) Wait() error {
	return <-p.waitCh
}

func (p *fakeRunningProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopOnce.Do(func() {
		p.stopped = true
		select {
		case p.waitCh <- nil:
		default:
		}
	})
	return nil
}

func (p *fakeRunningProcess) GracefulStop(_ time.Duration) error {
	return p.Stop()
}

func (p *fakeRunningProcess) Pid() int {
	return p.pid
}

func (p *fakeRunningProcess) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.stopped
}

func (p *fakeRunningProcess) isStopped() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stopped
}

// fakeStartableAdapter implements StartableRuntimeAdapter for deterministic testing.
type fakeStartableAdapter struct {
	proc     *fakeRunningProcess
	startErr error
	execErr  error
	onStart  func(step ExecutionStep)
}

func (f *fakeStartableAdapter) Name() string                  { return "fake-adapter" }
func (f *fakeStartableAdapter) Supports(_ ExecutionStep) bool { return true }
func (f *fakeStartableAdapter) Execute(_ context.Context, _ ExecutionStep, _ ResolvedEnvironment) error {
	return f.execErr
}
func (f *fakeStartableAdapter) Start(_ context.Context, step ExecutionStep, _ ResolvedEnvironment) (RunningProcess, error) {
	if f.onStart != nil {
		f.onStart(step)
	}
	if f.startErr != nil {
		return nil, f.startErr
	}
	return f.proc, nil
}

func setupTestProject(t *testing.T) (string, ProjectModel, ExecutionPlan) {
	t.Helper()
	root := t.TempDir()
	candidate := ExecutionCandidate{ID: "node.start", Command: "npm start", Confidence: 0.9}
	component := Component{
		Name:                "web",
		Path:                ".",
		Language:            "Node",
		ExecutionCandidates: []ExecutionCandidate{candidate},
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"web","scripts":{"start":"node index.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), []byte("console.log('hi');"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := ProjectModel{
		Name:       "web-app",
		Root:       root,
		Components: []Component{component},
	}
	plan := ExecutionPlan{
		ProjectName: "web-app",
		Root:        root,
		Steps: []ExecutionStep{{
			ID:                "component.web.start",
			Component:         "web",
			Phase:             PhaseStart,
			Command:           "npm start",
			LongRunning:       true,
			Candidates:        []ExecutionCandidate{candidate},
			SelectedCandidate: "node.start",
		}},
	}
	return root, model, plan
}

// 1. verified long-running startup saves before Ctrl+C
func TestVerifiedStartupSavesBeforeInterrupt(t *testing.T) {
	root, model, plan := setupTestProject(t)
	proc := newFakeRunningProcess(101)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	checkpointReached := make(chan struct{})
	lock := OctoLock{Project: model.Name}

	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			if !rep.VerifiedStartup {
				t.Error("expected VerifiedStartup to be true at checkpoint")
			}
			err := RecordVerifiedStrategies(root, model, plan, rep, lock)
			close(checkpointReached)
			return err
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan ExecutionReport)
	go func() {
		done <- ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
	}()

	// Wait for the verified-startup checkpoint
	select {
	case <-checkpointReached:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for verified-startup checkpoint")
	}

	// At this checkpoint, before any interrupt, the lock must already be saved on disk!
	savedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatalf("LoadOctoLock failed before Ctrl+C: %v", err)
	}
	if len(savedLock.Strategies) != 1 || savedLock.Strategies[0].Candidate != "node.start" {
		t.Fatalf("expected 1 verified strategy saved before Ctrl+C, got: %+v", savedLock)
	}

	// Clean up: cancel and let process wrap up
	cancel()
	report := <-done
	if !report.VerifiedStartup {
		t.Fatal("expected report.VerifiedStartup to stay true")
	}
}

// 2. Ctrl+C after readiness leaves a loadable lock and stops children
func TestCtrlCAfterReadinessLeavesLoadableLockAndStopsChildren(t *testing.T) {
	root, model, plan := setupTestProject(t)
	proc := newFakeRunningProcess(102)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	checkpointReached := make(chan struct{})
	lock := OctoLock{Project: model.Name}

	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			err := RecordVerifiedStrategies(root, model, plan, rep, lock)
			close(checkpointReached)
			return err
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan ExecutionReport)
	go func() {
		done <- ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
	}()

	// Wait for readiness checkpoint
	<-checkpointReached

	// Simulate Ctrl+C after readiness
	cancel()

	report := <-done

	if !report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to remain true")
	}
	if report.Success {
		t.Fatal("expected Success to be false on cancellation")
	}
	if !proc.isStopped() {
		t.Fatal("expected child process to be stopped on cancellation")
	}

	// Verify lock is loadable and valid
	savedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatalf("failed to load lock: %v", err)
	}
	if len(savedLock.Strategies) != 1 || savedLock.Strategies[0].Candidate != "node.start" {
		t.Fatalf("unexpected strategies in lock: %+v", savedLock.Strategies)
	}
}

// 3. Interruption during install/start/readiness creates no new verified strategy
func TestInterruptionDuringStartupCreatesNoVerifiedStrategy(t *testing.T) {
	// Case A: Interruption during setup/install step
	t.Run("during_setup", func(t *testing.T) {
		root, model, plan := setupTestProject(t)
		// Prepend a setup step that gets cancelled
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already cancelled before execution

		adapter := &fakeStartableAdapter{proc: newFakeRunningProcess(201)}
		resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
		env := ResolvedEnvironment{Values: map[string]string{}}
		lock := OctoLock{Project: model.Name}

		hookCalled := false
		opts := ExecutionOptions{
			OnVerifiedStartup: func(rep ExecutionReport) error {
				hookCalled = true
				return RecordVerifiedStrategies(root, model, plan, rep, lock)
			},
		}

		report := ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
		if report.VerifiedStartup {
			t.Fatal("expected VerifiedStartup to be false when cancelled during setup")
		}
		if hookCalled {
			t.Fatal("OnVerifiedStartup hook should not be called when cancelled during setup")
		}
		loaded, _ := LoadOctoLock(root)
		if len(loaded.Strategies) != 0 {
			t.Fatalf("expected 0 strategies, got %d", len(loaded.Strategies))
		}
	})

	// Case B: Interruption during candidate start
	t.Run("during_start", func(t *testing.T) {
		root, model, plan := setupTestProject(t)
		ctx, cancel := context.WithCancel(context.Background())

		adapter := &fakeStartableAdapter{
			onStart: func(_ ExecutionStep) {
				cancel() // cancel right when start is initiated
			},
			proc: newFakeRunningProcess(202),
		}
		resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
		env := ResolvedEnvironment{Values: map[string]string{}}
		lock := OctoLock{Project: model.Name}

		hookCalled := false
		opts := ExecutionOptions{
			OnVerifiedStartup: func(rep ExecutionReport) error {
				hookCalled = true
				return RecordVerifiedStrategies(root, model, plan, rep, lock)
			},
		}

		// Also make startable adapter return error if context is cancelled
		adapter.startErr = context.Canceled

		report := ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
		if report.VerifiedStartup {
			t.Fatal("expected VerifiedStartup to be false when start failed/cancelled")
		}
		if hookCalled {
			t.Fatal("OnVerifiedStartup hook should not be called")
		}
		loaded, _ := LoadOctoLock(root)
		if len(loaded.Strategies) != 0 {
			t.Fatalf("expected 0 strategies, got %d", len(loaded.Strategies))
		}
	})

	// Case C: Interruption during readiness verification
	t.Run("during_readiness_verification", func(t *testing.T) {
		root, model, plan := setupTestProject(t)
		proc := newFakeRunningProcess(203)
		adapter := &fakeStartableAdapter{proc: proc}
		resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
		env := ResolvedEnvironment{Values: map[string]string{}}
		lock := OctoLock{Project: model.Name}

		// Add a verification check that blocks and cancels
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reg := NewVerificationRegistry()
		blockingProvider := &blockingVerificationProvider{
			kind: VerificationHealth,
			onVerify: func() {
				cancel() // cancel context during verification
			},
		}
		reg.Register(blockingProvider)

		// Substitute verification check into model
		model.CustomVerification = []VerificationCheck{{
			ID:        "check.health",
			Kind:      VerificationHealth,
			Component: "web",
		}}

		// Temporarily replace default registry provider
		RegisterVerificationProvider(blockingProvider)
		defer func() {
			RegisterVerificationProvider(&HealthVerificationProvider{})
		}()

		hookCalled := false
		opts := ExecutionOptions{
			OnVerifiedStartup: func(rep ExecutionReport) error {
				hookCalled = true
				return RecordVerifiedStrategies(root, model, plan, rep, lock)
			},
		}

		report := ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
		if report.VerifiedStartup {
			t.Fatal("expected VerifiedStartup to be false when cancelled during readiness")
		}
		if hookCalled {
			t.Fatal("OnVerifiedStartup hook should not be called when readiness was interrupted")
		}
		loaded, _ := LoadOctoLock(root)
		if len(loaded.Strategies) != 0 {
			t.Fatalf("expected 0 strategies, got %d", len(loaded.Strategies))
		}
	})
}

type blockingVerificationProvider struct {
	kind     VerificationKind
	onVerify func()
}

func (b *blockingVerificationProvider) Kind() VerificationKind { return b.kind }
func (b *blockingVerificationProvider) Verify(ctx context.Context, check VerificationCheck) (VerificationResult, error) {
	if b.onVerify != nil {
		b.onVerify()
	}
	if ctx.Err() != nil {
		return VerificationResult{CheckID: check.ID, Passed: false, Reason: ctx.Err().Error()}, ctx.Err()
	}
	return VerificationResult{CheckID: check.ID, Passed: true}, nil
}

// 4. Verification failure does not overwrite an existing valid lock
func TestVerificationFailureDoesNotOverwriteExistingLock(t *testing.T) {
	root, model, plan := setupTestProject(t)
	// Pre-create a valid lock
	initialLock := OctoLock{
		Project: model.Name,
		Strategies: []VerifiedStrategy{{
			Component:   "web",
			Candidate:   "existing.candidate",
			Command:     "npm run existing",
			Fingerprint: "sha256:original",
			VerifiedAt:  time.Now().UTC(),
		}},
	}
	if err := SaveOctoLock(root, initialLock); err != nil {
		t.Fatal(err)
	}

	proc := newFakeRunningProcess(301)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	// Register a failing verification provider
	failingProvider := &failingVerificationProvider{kind: VerificationHealth}
	RegisterVerificationProvider(failingProvider)
	defer func() {
		RegisterVerificationProvider(&HealthVerificationProvider{})
	}()

	model.CustomVerification = []VerificationCheck{{
		ID:        "check.health",
		Kind:      VerificationHealth,
		Component: "web",
	}}

	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			t.Fatal("OnVerifiedStartup should not be invoked on verification failure")
			return nil
		},
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, env, opts)
	if report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to be false on verification failure")
	}

	// Verify original lock was NOT overwritten
	preservedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(preservedLock.Strategies) != 1 || preservedLock.Strategies[0].Candidate != "existing.candidate" {
		t.Fatalf("expected existing lock to be preserved, got %+v", preservedLock)
	}
}

type failingVerificationProvider struct {
	kind VerificationKind
}

func (f *failingVerificationProvider) Kind() VerificationKind { return f.kind }
func (f *failingVerificationProvider) Verify(_ context.Context, check VerificationCheck) (VerificationResult, error) {
	return VerificationResult{CheckID: check.ID, Passed: false, Reason: "forced verification failure"}, errors.New("check failed")
}

// 5. A later runtime failure stays a runtime failure
func TestLaterRuntimeFailureStaysRuntimeFailure(t *testing.T) {
	root, model, plan := setupTestProject(t)
	proc := newFakeRunningProcess(401)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	checkpointReached := make(chan struct{})
	lock := OctoLock{Project: model.Name}

	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			err := RecordVerifiedStrategies(root, model, plan, rep, lock)
			close(checkpointReached)
			return err
		},
	}

	done := make(chan ExecutionReport)
	go func() {
		done <- ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, env, opts)
	}()

	// Wait for verified-startup checkpoint
	<-checkpointReached

	// Simulate later crash in child process
	proc.waitCh <- errors.New("exit status 1")

	report := <-done

	if !report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to remain true")
	}
	if report.Success {
		t.Fatal("expected Success to be false for a crashed runtime process")
	}
	if report.FailureReason == "" {
		t.Fatal("expected FailureReason to explain the crash")
	}

	// Lock saved at verified startup must still be preserved
	savedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(savedLock.Strategies) != 1 || savedLock.Strategies[0].Candidate != "node.start" {
		t.Fatalf("expected verified strategy from startup to be preserved, got: %+v", savedLock)
	}
}

// 6. Watch checkpoints refresh saved state
func TestWatchCheckpointsRefreshSavedState(t *testing.T) {
	root, model, plan := setupTestProject(t)
	lock := OctoLock{Project: model.Name}

	// Checkpoint 1
	rep1 := ExecutionReport{
		ProjectName:     model.Name,
		VerifiedStartup: true,
		Steps: []ExecutionStepResult{{
			ID:          "component.web.start",
			Status:      StepSucceeded,
			CandidateID: "node.start",
		}},
	}
	updatedLock, err := RecordVerifiedStrategiesWithLock(root, model, plan, rep1, lock)
	if err != nil {
		t.Fatalf("checkpoint 1 failed: %v", err)
	}
	if len(updatedLock.Strategies) != 1 || updatedLock.Strategies[0].Candidate != "node.start" {
		t.Fatalf("expected 1 strategy in refreshed lock, got %+v", updatedLock)
	}
	lock = updatedLock

	// Checkpoint 2: add a second component "api"
	candAPI := ExecutionCandidate{ID: "node.api.start", Command: "node api.js", Confidence: 0.88}
	compAPI := Component{
		Name:                "api",
		Path:                "api",
		Language:            "Node",
		ExecutionCandidates: []ExecutionCandidate{candAPI},
	}
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "package.json"), []byte(`{"name":"api"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "api.js"), []byte("console.log('api');"), 0o644); err != nil {
		t.Fatal(err)
	}
	model.Components = append(model.Components, compAPI)
	plan.Steps = append(plan.Steps, ExecutionStep{
		ID:                "component.api.start",
		Component:         "api",
		Phase:             PhaseStart,
		Command:           "node api.js",
		Candidates:        []ExecutionCandidate{candAPI},
		SelectedCandidate: "node.api.start",
	})

	rep2 := ExecutionReport{
		ProjectName:     model.Name,
		VerifiedStartup: true,
		Steps: []ExecutionStepResult{
			{ID: "component.web.start", Status: StepSucceeded, CandidateID: "node.start"},
			{ID: "component.api.start", Status: StepSucceeded, CandidateID: "node.api.start"},
		},
	}
	refreshedLock2, err := RecordVerifiedStrategiesWithLock(root, model, plan, rep2, lock)
	if err != nil {
		t.Fatalf("checkpoint 2 failed: %v", err)
	}
	if len(refreshedLock2.Strategies) != 2 {
		t.Fatalf("expected 2 strategies in refreshed lock after watch update, got %d", len(refreshedLock2.Strategies))
	}

	onDisk, err := LoadOctoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Strategies) != 2 {
		t.Fatalf("expected 2 strategies on disk, got %d", len(onDisk.Strategies))
	}
}

// 7. Unwritable lock errors surface
func TestUnwritableLockErrorsSurface(t *testing.T) {
	root, model, plan := setupTestProject(t)
	proc := newFakeRunningProcess(501)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}
	lock := OctoLock{Project: model.Name}

	// Create a read-only .octo.lock file
	lockPath := filepath.Join(root, ".octo.lock")
	if err := os.WriteFile(lockPath, []byte("read-only"), 0o400); err != nil {
		t.Fatal(err)
	}
	// On Unix, ensure chmod 0400 is set
	_ = os.Chmod(lockPath, 0o400)

	var lockErr error
	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			updatedLock, err := RecordVerifiedStrategiesWithLock(root, model, plan, rep, lock)
			if err != nil {
				lockErr = err
				return err
			}
			lock = updatedLock
			return nil
		},
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, env, opts)

	if lockErr == nil {
		t.Fatal("expected lock save error to surface, got nil")
	}
	if report.FailureReason == "" {
		t.Fatal("expected report.FailureReason to reflect the lock save failure")
	}

	// Restore permissions for cleanup
	_ = os.Chmod(lockPath, 0o644)
}

// 8. Detached behavior still works
func TestDetachedBehaviorStillWorks(t *testing.T) {
	root, model, plan := setupTestProject(t)
	proc := newFakeRunningProcess(601)
	adapter := &fakeStartableAdapter{proc: proc}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}
	lock := OctoLock{Project: model.Name}

	hookCalled := false
	opts := ExecutionOptions{
		Detach: true,
		OnVerifiedStartup: func(rep ExecutionReport) error {
			hookCalled = true
			return RecordVerifiedStrategies(root, model, plan, rep, lock)
		},
	}

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, resolver, env, opts)

	if !report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to be true in detached mode")
	}
	if !report.Success {
		t.Fatalf("expected Success to be true in detached mode, got: %s", report.FailureReason)
	}
	if !hookCalled {
		t.Fatal("expected OnVerifiedStartup to be called in detached mode")
	}
	if proc.isStopped() {
		t.Fatal("detached process should not be stopped upon return")
	}

	savedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(savedLock.Strategies) != 1 || savedLock.Strategies[0].Candidate != "node.start" {
		t.Fatalf("expected 1 verified strategy saved in detached mode, got: %+v", savedLock)
	}
}

// Subprocess SIGINT test where supported, without flaky sleeps
func TestSubprocessSIGINTGracefulTeardown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process groups and SIGINT forwarding are not supported on Windows")
	}

	root := t.TempDir()
	pipeFile := filepath.Join(root, "ready.pipe")
	scriptPath := filepath.Join(root, "server.sh")

	// Script writes to pipe to signal ready, then traps SIGTERM/SIGINT and exits 0 cleanly
	scriptContent := fmt.Sprintf(`#!/bin/sh
trap 'exit 0' TERM INT
touch "%s"
while true; do
  sleep 0.1
done
`, pipeFile)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		t.Fatal(err)
	}

	candidate := ExecutionCandidate{ID: "sh.start", Command: "/bin/sh " + scriptPath, Confidence: 0.95}
	component := Component{
		Name:                "service",
		Path:                ".",
		Language:            "Shell",
		ExecutionCandidates: []ExecutionCandidate{candidate},
	}
	model := ProjectModel{
		Name:       "sigint-demo",
		Root:       root,
		Components: []Component{component},
	}
	plan := ExecutionPlan{
		ProjectName: "sigint-demo",
		Root:        root,
		Steps: []ExecutionStep{{
			ID:                "component.service.start",
			Component:         "service",
			Phase:             PhaseStart,
			Command:           "/bin/sh " + scriptPath,
			LongRunning:       true,
			Candidates:        []ExecutionCandidate{candidate},
			SelectedCandidate: "sh.start",
		}},
	}

	resolver := NewRuntimeResolver()
	env := ResolvedEnvironment{Values: map[string]string{}}
	lock := OctoLock{Project: model.Name}

	checkpointReached := make(chan struct{})
	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			err := RecordVerifiedStrategies(root, model, plan, rep, lock)
			close(checkpointReached)
			return err
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan ExecutionReport)
	go func() {
		done <- ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
	}()

	// Deterministic wait: wait for checkpoint and wait for ready.pipe created by script
	select {
	case <-checkpointReached:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("timed out waiting for checkpoint")
	}

	// Poll pipeFile deterministically without arbitrary sleeps
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(pipeFile); err == nil {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		cancel()
		t.Fatal("timed out waiting for subprocess ready signal")
	}

	// Now send SIGINT / cancel context to simulate Ctrl+C
	cancel()

	report := <-done

	if !report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to be true")
	}
	if report.Success {
		t.Fatal("expected Success to be false on SIGINT / cancellation")
	}

	// Verify lock file was left in valid, loadable state
	savedLock, err := LoadOctoLock(root)
	if err != nil {
		t.Fatalf("failed to load lock after SIGINT: %v", err)
	}
	if len(savedLock.Strategies) != 1 || savedLock.Strategies[0].Candidate != "sh.start" {
		t.Fatalf("expected strategy 'sh.start' saved after SIGINT, got %+v", savedLock)
	}
}
