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
	"syscall"

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

// NewRuntimeResolver creates a resolver with the built-in adapters.
func NewRuntimeResolver() RuntimeResolver {
	return RuntimeResolver{
		adapters: []RuntimeAdapter{
			ComposeAdapter{},
			ShellAdapter{},
		},
	}
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

func (ShellAdapter) Execute(ctx context.Context, step ExecutionStep, env ResolvedEnvironment) error {
	if step.Command == "" {
		return fmt.Errorf("step %q has no command", step.ID)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
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
	cmd := exec.Command("sh", "-c", step.Command)
	if step.WorkDir != "" {
		cmd.Dir = step.WorkDir
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
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
	logFile, err := os.OpenFile(filepath.Join(logDir, logName+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else if isSilentExecution(ctx) {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("shell step %q failed to start: %w", step.ID, err)
	}
	return &shellProcess{cmd: cmd}, nil
}

type shellProcess struct {
	cmd *exec.Cmd
}

func (p *shellProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *shellProcess) Stop() error {
	if p.cmd.Process == nil {
		return nil
	}
	if runtime.GOOS != "windows" && p.cmd.Process.Pid > 0 {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *shellProcess) Pid() int {
	if p.cmd.Process == nil {
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
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Command)
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
			if strings.HasPrefix(entry, prefix) {
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
