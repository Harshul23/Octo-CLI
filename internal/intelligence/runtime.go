package intelligence

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// RuntimeAdapter executes one execution step for a specific runtime kind.
type RuntimeAdapter interface {
	Name() string
	Supports(step ExecutionStep) bool
	Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error
}

// RunningProcess represents a started long-running execution step.
type RunningProcess interface {
	Wait() error
	Stop() error
	GracefulStop(timeout time.Duration) error
	Pid() int
}

// StartableRuntimeAdapter supports starting long-running commands without
// blocking the planner while readiness verification runs.
type StartableRuntimeAdapter interface {
	RuntimeAdapter
	Start(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) (RunningProcess, error)
}

// RuntimeResolver selects an adapter for an execution step.
type RuntimeResolver struct {
	adapters []RuntimeAdapter
}

// RegisterAdapter adds a custom runtime adapter with highest precedence.
func (r *RuntimeResolver) RegisterAdapter(adapter RuntimeAdapter) {
	r.adapters = append([]RuntimeAdapter{adapter}, r.adapters...)
}

// RuntimeAdapterRegistry manages global or custom runtime adapters.
type RuntimeAdapterRegistry struct {
	mu       sync.RWMutex
	adapters []RuntimeAdapter
}

func NewRuntimeAdapterRegistry() *RuntimeAdapterRegistry {
	return &RuntimeAdapterRegistry{}
}

var defaultRuntimeAdapterRegistry = NewRuntimeAdapterRegistry()

func DefaultRuntimeAdapterRegistry() *RuntimeAdapterRegistry {
	return defaultRuntimeAdapterRegistry
}

func RegisterRuntimeAdapter(adapter RuntimeAdapter) {
	defaultRuntimeAdapterRegistry.Register(adapter)
}

func UnregisterRuntimeAdapter(adapter RuntimeAdapter) {
	defaultRuntimeAdapterRegistry.Unregister(adapter)
}

func (r *RuntimeAdapterRegistry) Register(adapter RuntimeAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = append([]RuntimeAdapter{adapter}, r.adapters...)
}

func (r *RuntimeAdapterRegistry) Unregister(adapter RuntimeAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := make([]RuntimeAdapter, 0, len(r.adapters))
	for _, a := range r.adapters {
		if a != adapter {
			filtered = append(filtered, a)
		}
	}
	r.adapters = filtered
}

func (r *RuntimeAdapterRegistry) Adapters() []RuntimeAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RuntimeAdapter, len(r.adapters))
	copy(out, r.adapters)
	return out
}

// RuntimeResolverOptions configures adapter selection in RuntimeResolver.
type RuntimeResolverOptions struct {
	Sandbox        bool
	Root           string
	Language       string
	RuntimeVersion string
}

// NewRuntimeResolver creates a resolver with the built-in adapters.
func NewRuntimeResolver() RuntimeResolver {
	return NewRuntimeResolverWithOptions(RuntimeResolverOptions{})
}

// NewRuntimeResolverWithOptions creates a resolver configured for standard or sandboxed container execution.
func NewRuntimeResolverWithOptions(opts RuntimeResolverOptions) RuntimeResolver {
	custom := defaultRuntimeAdapterRegistry.Adapters()
	if opts.Sandbox {
		container := NewContainerAdapter(opts.Root)
		container.Language = opts.Language
		container.RuntimeVersion = opts.RuntimeVersion
		base := []RuntimeAdapter{
			ComposeAdapter{},
			container,
			ShellAdapter{},
		}
		return RuntimeResolver{adapters: append(custom, base...)}
	}
	base := []RuntimeAdapter{
		ComposeAdapter{},
		ShellAdapter{},
	}
	return RuntimeResolver{adapters: append(custom, base...)}
}

// Resolve returns the first adapter that explicitly supports the step.
func (r RuntimeResolver) Resolve(step ExecutionStep) (RuntimeAdapter, error) {
	for _, adapter := range r.adapters {
		if adapter.Supports(step) {
			return adapter, nil
		}
	}
	return nil, fmt.Errorf("no runtime adapter supports step %q", step.ID)
}

// ShellAdapter executes ordinary shell commands.
type ShellAdapter struct{}

func (ShellAdapter) Name() string { return "shell" }

func (ShellAdapter) Supports(step ExecutionStep) bool {
	return step.Command != "" && !strings.HasPrefix(step.Command, "docker compose ")
}

func createShellCmdContext(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func createShellCmd(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

func (ShellAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if step.Command == "" {
		return fmt.Errorf("step %q has no command", step.ID)
	}

	cmd := createShellCmdContext(ctx, step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	cmd.Stdin = os.Stdin
	cmd.Env = mergedEnvironmentWithStep(env.ForStep(step).Values, step.Environment)

	if isSilentExecution(ctx) {
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(output.String())
			if detail != "" {
				return fmt.Errorf("shell step %q failed: %w\n%s", step.ID, err, detail)
			}
			return fmt.Errorf("shell step %q failed: %w", step.ID, err)
		}
		return nil
	}

	// Interactive start commands (for example terminal UIs) must keep their
	// stdout/stderr attached to the user's terminal. Capturing them would make
	// the process appear stuck because the child is waiting for terminal input
	// while its UI is hidden in a buffer.
	if step.Phase == PhaseStart && interactiveShellTerminal() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("shell step %q failed: %w", step.ID, err)
		}
		return nil
	}

	// Non-interactive candidate starts are exploratory. Capture their output so
	// failed candidates do not flood the terminal when Octo can fall back.
	if step.Phase == PhaseStart {
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(output.String())
			if detail != "" {
				return fmt.Errorf("shell step %q failed: %w\n%s", step.ID, err, detail)
			}
			return fmt.Errorf("shell step %q failed: %w", step.ID, err)
		}
		return nil
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell step %q failed: %w", step.ID, err)
	}
	return nil
}

func (ShellAdapter) Start(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) (RunningProcess, error) {
	if step.Command == "" {
		return nil, fmt.Errorf("step %q has no command", step.ID)
	}
	cmd := createShellCmd(step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	setProcessGroup(cmd)
	stepEnv := env.ForStep(step)
	cmd.Env = mergedEnvironmentWithStep(stepEnv.Values, step.Environment)

	workDir := step.WorkDir
	if workDir == "" {
		workDir = "."
	}
	logDir := filepath.Join(workDir, ".octo", "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logName := step.Component
	if logName == "" {
		logName = "process"
	}

	var stdoutW, stderrW io.Writer
	if isSilentExecution(ctx) || isDetachedExecution(ctx) {
		stdoutW = io.Discard
		stderrW = io.Discard
	} else {
		pfx := fmt.Sprintf("[%s] ", logName)
		pw := &prefixWriter{writer: os.Stdout, prefix: pfx}
		stdoutW = pw
		stderrW = pw
	}

	logFilePath := filepath.Join(logDir, logName+".log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		if stdoutW == io.Discard {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		} else {
			cmd.Stdout = io.MultiWriter(logFile, stdoutW)
			cmd.Stderr = io.MultiWriter(logFile, stderrW)
		}
	} else {
		cmd.Stdout = stdoutW
		cmd.Stderr = stderrW
	}

	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return nil, fmt.Errorf("shell step %q failed to start: %w", step.ID, err)
	}
	proc := &shellProcess{
		cmd:      cmd,
		logFile:  logFile,
		logPath:  logFilePath,
		waitDone: make(chan struct{}),
	}
	go func() {
		proc.waitErr = cmd.Wait()
		proc.closeLog()
		close(proc.waitDone)
	}()
	return proc, nil
}

type prefixWriter struct {
	mu     sync.Mutex
	writer io.Writer
	prefix string
	buffer []byte
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.buffer = append(pw.buffer, p...)

	for {
		newlineIdx := bytes.IndexByte(pw.buffer, '\n')
		if newlineIdx == -1 {
			break
		}
		line := pw.buffer[:newlineIdx+1]
		if _, err := pw.writer.Write(append([]byte(pw.prefix), line...)); err != nil {
			return len(p), err
		}
		pw.buffer = pw.buffer[newlineIdx+1:]
	}
	return len(p), nil
}

type shellProcess struct {
	cmd      *exec.Cmd
	logFile  *os.File
	logPath  string
	waitDone chan struct{}
	waitErr  error
	closeMu  sync.Mutex
	closed   bool
}

// LastOutput returns the final lines of the process log file if available,
// aiding failure diagnosis (e.g. port bind collision or compilation panic).
func (p *shellProcess) LastOutput() string {
	if p.logPath == "" {
		return ""
	}
	data, err := os.ReadFile(p.logPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p *shellProcess) closeLog() {
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if !p.closed && p.logFile != nil {
		p.closed = true
		_ = p.logFile.Close()
	}
}

func (p *shellProcess) Wait() error {
	<-p.waitDone
	return p.waitErr
}

func (p *shellProcess) IsAlive() bool {
	select {
	case <-p.waitDone:
		return false
	default:
		return true
	}
}

func (p *shellProcess) Stop() error {
	return p.GracefulStop(2 * time.Second)
}

func (p *shellProcess) GracefulStop(timeout time.Duration) error {
	// The wait goroutine closes the log before signaling completion. Do not
	// close it here while cmd.Wait may still be draining output.
	return gracefulStopShellProcess(p.cmd, p.waitDone, timeout)
}

func (p *shellProcess) Pid() int {
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// ComposeAdapter executes Docker Compose service steps.
type ComposeAdapter struct{}

func (ComposeAdapter) Name() string { return "compose" }

func (ComposeAdapter) Supports(step ExecutionStep) bool {
	return strings.HasPrefix(step.Command, "docker compose ")
}

func (ComposeAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if !(ComposeAdapter{}).Supports(step) {
		return fmt.Errorf("step %q is not a Compose step", step.ID)
	}
	cmd := createShellCmdContext(ctx, step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	if isSilentExecution(ctx) {
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		cmd.Env = mergedEnvironmentWithStep(env.Values, step.Environment)
		if err := cmd.Run(); err != nil {
			detail := strings.TrimSpace(output.String())
			if detail != "" {
				return fmt.Errorf("Compose step %q failed: %w\n%s", step.ID, err, detail)
			}
			return fmt.Errorf("Compose step %q failed: %w", step.ID, err)
		}
		return nil
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = mergedEnvironmentWithStep(env.Values, step.Environment)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Compose step %q failed: %w", step.ID, err)
	}
	return nil
}

func mergedEnvironment(values map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for name, value := range values {
		replaced := false
		prefix := name + "="
		for i, entry := range env {
			match := strings.HasPrefix(entry, prefix)
			if !match && runtime.GOOS == "windows" {
				if eq := strings.Index(entry, "="); eq >= 0 {
					match = strings.EqualFold(entry[:eq+1], prefix)
				}
			}
			if match {
				env[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}

func mergedEnvironmentWithStep(values, stepValues map[string]string) []string {
	merged := make(map[string]string, len(values)+len(stepValues))
	for name, value := range values {
		merged[name] = value
	}
	for name, value := range stepValues {
		merged[name] = value
	}
	return mergedEnvironment(merged)
}

func interactiveShellTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd())) &&
		term.IsTerminal(int(os.Stderr.Fd()))
}
